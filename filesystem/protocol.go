package filesystem

import (
	"context"
	"fmt"
	"time"

	"github.com/ltick/tick-framework/config"
)

var (
	errInitiate  string = "filesystem template: initiate '%s' error"
	errOnStartup string = "filesystem template: onStartup '%s' error"

	errRegister string = "filesystem template: Register template is nil"
	errUse      string = "filesystem template: unknown template '%s' (forgotten register?)"
)

var (
	defaultProvider string = "file"
)

func NewFilesystem() *Filesystem {
	return &Filesystem{
		Config: config.NewConfig(),
	}
}

type Filesystem struct {
	Config      *config.Config
	handlerName string
	handler     Handler
}

func (this *Filesystem) Initiate(ctx context.Context) (context.Context, error) {
	ctx, err := this.Config.Initiate(ctx)
	if err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	var configs map[string]config.Option = map[string]config.Option{
		"FILESYSTEM_PROVIDER":                config.Option{Type: config.String, Default: defaultProvider, EnvironmentKey: "FILESYSTEM_PROVIDER"},
		"FILESYSTEM_DEFRAG_CONTENT_INTERVAL": config.Option{Type: config.Duration, Default: 30 * time.Minute, EnvironmentKey: "FILESYSTEM_DEFRAG_CONTENT_INTERVAL"},
		"FILESYSTEM_DEFRAG_CONTENT_LIFETIME": config.Option{Type: config.Duration, Default: 24 * time.Hour, EnvironmentKey: "FILESYSTEM_DEFRAG_CONTENT_LIFETIME"},
		"FILESYSTEM_LRU_CAPACITY":            config.Option{Type: config.Int64, Default: 32 * 1024 * 1024, EnvironmentKey: "FILESYSTEM_LRU_CAPACITY"},
	}
	if err = this.Config.SetOptions(configs); err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	if err = Register(defaultProvider, NewFileHandler); err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	if err = Register("lruFile", NewLRUFileHandler); err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	return ctx, nil
}
func (this *Filesystem) OnStartup(ctx context.Context) (context.Context, error) {
	var provider string = this.Config.GetString("FILESYSTEM_PROVIDER")
	if provider == "" {
		provider = defaultProvider
	}
	if err := this.Use(ctx, provider); err != nil {
		return ctx, fmt.Errorf(errOnStartup, err.Error())
	}
	return ctx, nil
}
func (this *Filesystem) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (this *Filesystem) Use(ctx context.Context, handlerName string) (err error) {
	var handler storageHandler
	if handler, err = Use(handlerName); err != nil {
		return
	}
	this.handlerName = handlerName
	this.handler = handler()
	if err = this.handler.Initiate(ctx, this.Config); err != nil {
		return
	}
	return
}

func (this *Filesystem) SetContent(key string, content []byte) (err error) {
	return this.handler.SetContent(key, content)
}

func (this *Filesystem) GetContent(key string) (content []byte, err error) {
	return this.handler.GetContent(key)
}

func (this *Filesystem) DelContent(key string) (err error) {
	return this.handler.DelContent(key)
}

type Handler interface {
	Initiate(ctx context.Context, conf *config.Config) error
	SetContent(key string, content []byte) (err error)
	GetContent(key string) (content []byte, err error)
	DelContent(key string) (err error)
}

type storageHandler func() Handler

var storageHandlers map[string]storageHandler = make(map[string]storageHandler)

func Register(name string, handler storageHandler) (err error) {
	if handler == nil {
		err = fmt.Errorf(errRegister)
		return
	}
	var ok bool
	if _, ok = storageHandlers[name]; ok {
		return
	}
	storageHandlers[name] = handler
	return
}

func Use(name string) (handler storageHandler, err error) {
	var ok bool
	if handler, ok = storageHandlers[name]; !ok {
		err = fmt.Errorf(errUse, name)
		return
	}
	return
}
