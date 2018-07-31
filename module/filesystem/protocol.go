package filesystem

import (
	"context"
	"fmt"
	"time"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate  string = "filesystem template: initiate '%s' error"
	errOnStartup string = "filesystem template: onStartup '%s' error"

	errRegister string = "filesystem template: Register template is nil"
	errUse      string = "filesystem template: unknown template '%s' (forgotten register?)"
)

var (
	defaultProvider string = "fileTemplate"
)

func NewInstance() *Instance {
	return &Instance{
		Config: config.NewInstance(),
	}
}

type Instance struct {
	Config      *config.Instance
	handlerName string
	handler     TemplateHandler
}

func (this *Instance) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	if newCtx, err = this.Config.Initiate(ctx); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	var configs map[string]config.Option = map[string]config.Option{
		"FILESYSTEM_TEMPLATE_PROVIDER":        config.Option{Type: config.String, Default: defaultProvider, EnvironmentKey: "FILESYSTEM_TEMPLATE_PROVIDER"},
		"FILESYSTEM_TEMPLATE_DEFRAG_INTERVAL": config.Option{Type: config.Duration, Default: 30 * time.Minute, EnvironmentKey: "FILESYSTEM_TEMPLATE_DEFRAG_INTERVAL"},
		"FILESYSTEM_TEMPLATE_DEFRAG_LIFETIME": config.Option{Type: config.Duration, Default: 24 * time.Hour, EnvironmentKey: "FILESYSTEM_TEMPLATE_DEFRAG_LIFETIME"},
		"LRU_PROVIDER":      config.Option{Type: config.String, Default: defaultProvider, EnvironmentKey: "LRU_PROVIDER"},
		"LRU_CAPACITY":      config.Option{Type: config.Int64, Default: 32 * 1024 * 1024, EnvironmentKey: "LRU_CAPACITY"},
		"LRU_DIR":           config.Option{Type: config.String, Default: "/tmp/lru", EnvironmentKey: "LRU_DIR"},
		"LRU_SAVE_INTERVAL": config.Option{Type: config.Duration, Default: 5 * time.Minute, EnvironmentKey: "LRU_SAVE_INTERVAL"},
	}
	if newCtx, err = this.Config.SetOptions(ctx, configs); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	if err = FileTemplateRegister(defaultProvider, NewFileTemplateHandler); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	return
}
func (this *Instance) OnStartup(ctx context.Context) (newCtx context.Context, err error) {
	newCtx = ctx
	var provider string = this.Config.GetString("FILESYSTEM_TEMPLATE_PROVIDER")
	if provider == "" {
		provider = defaultProvider
	}
	if err = this.Use(ctx, provider); err != nil {
		err = fmt.Errorf(errOnStartup, err.Error())
		return
	}
	return
}
func (this *Instance) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestStartup(ctx context.Context, c *routing.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestShutdown(ctx context.Context, c *routing.Context) (context.Context, error) {
	return ctx, nil
}

func (this *Instance) Use(ctx context.Context, handlerName string) (err error) {
	var handler fileTemplateHandler
	if handler, err = FileTemplateUse(handlerName); err != nil {
		return
	}
	this.handlerName = handlerName
	this.handler = handler()
	if err = this.handler.Initiate(ctx, this.Config); err != nil {
		return
	}
	return
}

func (this *Instance) SetContent(key string, content []byte) (err error) {
	return this.handler.SetContent(key, content)
}

func (this *Instance) GetContent(key string) (content []byte, err error) {
	return this.handler.GetContent(key)
}

type TemplateHandler interface {
	Initiate(ctx context.Context, conf *config.Instance) error
	SetContent(key string, content []byte) (err error)
	GetContent(key string) (content []byte, err error)
}

type fileTemplateHandler func() TemplateHandler

var fileTemplateHandlers map[string]fileTemplateHandler = make(map[string]fileTemplateHandler)

func FileTemplateRegister(name string, handler fileTemplateHandler) (err error) {
	if handler == nil {
		err = fmt.Errorf(errRegister)
		return
	}
	var ok bool
	if _, ok = fileTemplateHandlers[name]; ok {
		return
	}
	fileTemplateHandlers[name] = handler
	return
}

func FileTemplateUse(name string) (handler fileTemplateHandler, err error) {
	var ok bool
	if handler, ok = fileTemplateHandlers[name]; !ok {
		err = fmt.Errorf(errUse, name)
		return
	}
	return
}
