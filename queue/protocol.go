package queue

import (
	"context"
	"errors"
	"fmt"

	"github.com/ltick/tick-framework/config"
)

var (
	errInitiate = "queue: initiate '%s' error"
	errStartup  = "queue: startup '%s' error"
	errNewQueue = "queue: new '%s' queue error"
	errGetQueue = "queue: get '%s' queue error"
)

func NewQueue() *Queue {
	instance := &Queue{
	}
	return instance
}

type Queue struct {
	Config      *config.Config
	handlerName string
	handler     Handler
}

func (q *Queue) Initiate(ctx context.Context) (context.Context, error) {
	var configs map[string]config.Option = map[string]config.Option{
		"QUEUE_PROVIDER":          config.Option{Type: config.String, Default: "kafka", EnvironmentKey: "QUEUE_PROVIDER"},
		"QUEUE_KAFKA_BROKERS":     config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_BROKERS"},
		"QUEUE_KAFKA_EVENT_GROUP": config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_EVENT_GROUP"},
		"QUEUE_KAFKA_EVENT_TOPIC": config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_EVENT_TOPIC"},
	}
	err := q.Config.SetOptions( configs)
	if err != nil {
		return ctx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	err = Register("kafka", NewKafkaHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), q.handlerName))
	}

	return ctx, nil
}
func (q *Queue) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	queueProvider := q.Config.GetString("QUEUE_PROVIDER")
	if queueProvider != "" {
		err = q.Use(ctx, queueProvider)
	} else {
		err = q.Use(ctx, "kafka")
	}
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), q.handlerName))
	}
	err = q.handler.Initiate(ctx)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), q.handlerName))
	}
	return ctx, nil
}
func (q *Queue) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (q *Queue) HandlerName() string {
	return q.handlerName
}
func (q *Queue) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	q.handlerName = handlerName
	q.handler = handler()
	return nil
}

type queueHandler func() Handler

var queueHandlers = make(map[string]queueHandler)

func Register(name string, queueHandler queueHandler) error {
	if queueHandler == nil {
		return errors.New("queue: Register queue is nil")
	}
	if _, ok := queueHandlers[name]; !ok {
		queueHandlers[name] = queueHandler
	}
	return nil
}
func Use(name string) (queueHandler, error) {
	if _, exist := queueHandlers[name]; !exist {
		return nil, errors.New("queue: unknown queue " + name + " (forgotten register?)")
	}
	return queueHandlers[name], nil
}

func (q *Queue) NewQueue(ctx context.Context, name string, config map[string]interface{}) (QueueHandler, error) {
	queueHandler, err := q.GetQueue(name)
	if err == nil {
		return queueHandler, nil
	}
	queueHandler, err = q.handler.NewQueue(ctx, name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewQueue+": "+err.Error(), name))
	}
	if queueHandler == nil {
		return nil, errors.New(fmt.Sprintf(errNewQueue+": empty pool", name))
	}
	return queueHandler, nil
}
func (q *Queue) GetQueue(name string) (QueueHandler, error) {
	queueHandler, err := q.handler.GetQueue(name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errGetQueue+": "+err.Error(), name))
	}
	return queueHandler, err
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewQueue(ctx context.Context, name string, config map[string]interface{}) (QueueHandler, error)
	GetQueue(name string) (QueueHandler, error)
}

type QueueHandler interface {
	NewConsumer(ctx context.Context, group string, topic string) (*Consumer, error)
	NewProducer(ctx context.Context, topic string, errHandles ...func(context.Context, string, string, error)) (*Producer, error)
	GetConfig() map[string]interface{}
}
