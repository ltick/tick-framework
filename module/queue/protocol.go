package queue

import (
	"context"
	"errors"
	"fmt"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate = "queue: initiate '%s' error"
	errStartup  = "queue: startup '%s' error"
	errNewQueue = "queue: new '%s' queue error"
	errGetQueue = "queue: get '%s' queue error"
)

func NewInstance() *Instance {
	util := &utility.Instance{}
	instance := &Instance{
		Utility: util,
	}
	return instance
}

type Instance struct {
	Config      *config.Instance
	Utility     *utility.Instance
	handlerName string
	handler     Handler
}

func (this *Instance) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var configs map[string]config.Option = map[string]config.Option{
		"QUEUE_PROVIDER":          config.Option{Type: config.String, Default: "kafka", EnvironmentKey: "QUEUE_PROVIDER"},
		"QUEUE_KAFKA_BROKERS":     config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_BROKERS"},
		"QUEUE_KAFKA_EVENT_GROUP": config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_EVENT_GROUP"},
		"QUEUE_KAFKA_EVENT_TOPIC": config.Option{Type: config.String, EnvironmentKey: "QUEUE_KAFKA_EVENT_TOPIC"},
	}
	newCtx, err = this.Config.SetOptions(ctx, configs)
	if err != nil {
		return newCtx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	err = Register("kafka", NewKafkaHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}

	return ctx, nil
}
func (this *Instance) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	queueProvider := this.Config.GetString("QUEUE_PROVIDER")
	if queueProvider != "" {
		err = this.Use(ctx, queueProvider)
	} else {
		err = this.Use(ctx, "kafka")
	}
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), this.handlerName))
	}
	err = this.handler.Initiate(ctx)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return ctx, nil
}
func (this *Instance) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestStartup(c *routing.Context) error {
	return nil
}
func (this *Instance) OnRequestShutdown(c *routing.Context) error {
	return nil
}
func (this *Instance) HandlerName() string {
	return this.handlerName
}
func (this *Instance) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	this.handlerName = handlerName
	this.handler = handler()
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

func (this *Instance) NewQueue(ctx context.Context, name string, config map[string]interface{}) (QueueHandler, error) {
	queueHandler, err := this.GetQueue(name)
	if err == nil {
		return queueHandler, nil
	}
	queueHandler, err = this.handler.NewQueue(ctx, name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewQueue+": "+err.Error(), name))
	}
	if queueHandler == nil {
		return nil, errors.New(fmt.Sprintf(errNewQueue+": empty pool", name))
	}
	return queueHandler, nil
}
func (this *Instance) GetQueue(name string) (QueueHandler, error) {
	queueHandler, err := this.handler.GetQueue(name)
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
