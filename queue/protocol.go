package queue

import (
	"context"
	"fmt"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
)

var (
	errRegister = "queue: register error"
	errUse      = "queue: use error"
	errPrepare  = "queue: prepare '%s' error"
	errInitiate = "queue: initiate '%s' error"
	errStartup  = "queue: startup '%s' error"
	errNewQueue = "queue: new '%s' queue error"
	errGetQueue = "queue: get '%s' queue error"
)

func NewQueue() *Queue {
	instance := &Queue{}
	return instance
}

type Queue struct {
	Config   *config.Config `inject:"true"`
	configs  map[string]interface{}
	Provider string
	handler  Handler
}

func (q *Queue) Prepare(ctx context.Context) (context.Context, error) {
	var configs map[string]config.Option = map[string]config.Option{
		"QUEUE_PROVIDER":          config.Option{Type: config.String, Default: "kafka", EnvironmentKey: "QUEUE_PROVIDER"},
		"QUEUE_KAFKA_BROKERS":     config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_BROKERS"},
	}
	err := q.Config.SetOptions(configs)
	if err != nil {
		return ctx, errors.Annotate(err, errPrepare)
	}
	q.configs = make(map[string]interface{}, 0)
	return ctx, nil
}

func (q *Queue) Initiate(ctx context.Context) (context.Context, error) {
	err := Register("kafka", NewKafkaHandler)
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, q.Provider))
	}
	if _, ok := q.configs["QUEUE_KAFKA_BROKERS"]; !ok {
		q.configs["QUEUE_KAFKA_BROKERS"] = q.Config.GetString("QUEUE_KAFKA_BROKERS")
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
		return ctx, errors.Annotate(err, fmt.Sprintf(errStartup, q.Provider))
	}
	err = q.handler.Initiate(ctx)
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, q.Provider))
	}
	return ctx, nil
}
func (q *Queue) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (q *Queue) GetProvider() string {
	return q.Provider
}
func (q *Queue) Use(ctx context.Context, Provider string) error {
	handler, err := Use(Provider)
	if err != nil {
		return err
	}
	q.Provider = Provider
	q.handler = handler()
	return nil
}

type queueHandler func() Handler

var queueHandlers = make(map[string]queueHandler)

func Register(name string, queueHandler queueHandler) error {
	if queueHandler == nil {
		return errors.Annotate(errors.New("queue: Register queue is nil"), errRegister)
	}
	if _, ok := queueHandlers[name]; !ok {
		queueHandlers[name] = queueHandler
	}
	return nil
}
func Use(name string) (queueHandler, error) {
	if _, exist := queueHandlers[name]; !exist {
		return nil, errors.Annotate(errors.New("queue: unknown queue "+name+" (forgotten register?)"), errUse)
	}
	return queueHandlers[name], nil
}

func (q *Queue) NewQueue(ctx context.Context, name string, configs ...map[string]interface{}) (QueueHandler, error) {
	queueHandler, err := q.GetQueue(name)
	if err == nil {
		return queueHandler, nil
	}
	if len(configs) > 0 {
		// merge
		for key, value := range q.configs {
			if _, ok := configs[0][key]; !ok {
				configs[0][key] = value
			}
		}
		queueHandler, err = q.handler.NewQueue(ctx, name, configs[0])
	} else {
		queueHandler, err = q.handler.NewQueue(ctx, name, q.configs)
	}
	if err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errNewQueue, name))
	}
	if queueHandler == nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errNewQueue+": empty pool", name))
	}
	return queueHandler, nil
}
func (q *Queue) GetQueue(name string) (QueueHandler, error) {
	queueHandler, err := q.handler.GetQueue(name)
	if err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errGetQueue, name))
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
