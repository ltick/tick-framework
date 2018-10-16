package block

import (
	"context"
	"fmt"
	"time"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate  string = "filesystem block: initiate '%s' error"
	errOnStartup string = "filesystem block: onStartup '%s' error"

	errRegister string = "filesystem block: Register block is nil"
	errUse      string = "filesystem block: unknown block '%s' (forgotten register?)"
)

var (
	defaultProvider string = "fileBlock"
)

func NewInstance() *Instance {
	return &Instance{
		Config: config.NewConfig(),
	}
}

type Instance struct {
	Config      *config.Config
	handlerName string
	handler     BlockHandler
}

func (this *Instance) Initiate(ctx context.Context) (context.Context, error) {
	ctx, err := this.Config.Initiate(ctx)
	if err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	var configs map[string]config.Option = map[string]config.Option{
		"FILESYSTEM_BLOCK_PROVIDER":            config.Option{Type: config.String, Default: defaultProvider, EnvironmentKey: "FILESYSTEM_BLOCK_PROVIDER"},
		"FILESYSTEM_BLOCK_DIR":                 config.Option{Type: config.String, Default: "/tmp/block", EnvironmentKey: "FILESYSTEM_BLOCK_DIR"},
		"FILESYSTEM_BLOCK_CONTENT_SIZE":        config.Option{Type: config.Int64, Default: 64 * 1024 * 1024, EnvironmentKey: "FILESYSTEM_BLOCK_CONTENT_SIZE"},
		"FILESYSTEM_BLOCK_INDEX_SAVE_INTERVAL": config.Option{Type: config.Duration, Default: 5 * time.Minute, EnvironmentKey: "FILESYSTEM_BLOCK_INDEX_SAVE_INTERVAL"},
	}
	if err = this.Config.SetOptions(configs); err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	if err = FileBlockRegister(defaultProvider, NewFileBlockHandler); err != nil {
		return ctx, fmt.Errorf(errInitiate, err.Error())
	}
	return ctx, nil
}
func (this *Instance) OnStartup(ctx context.Context) (newCtx context.Context, err error) {
	newCtx = ctx
	var provider string = this.Config.GetString("FILESYSTEM_BLOCK_PROVIDER")
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
	var handler fileBlockHandler
	if handler, err = FileBlockUse(handlerName); err != nil {
		return
	}
	this.handlerName = handlerName
	this.handler = handler()
	if err = this.handler.Initiate(ctx, this.Config); err != nil {
		return
	}
	return
}

func (this *Instance) Set(key string, value []byte) (err error) {
	return this.handler.Set(key, value)
}

func (this *Instance) Get(key string) (value []byte, err error) {
	return this.handler.Get(key)
}

func (this *Instance) Del(key string) (err error) {
	return this.handler.Del(key)
}

func (this *Instance) DefragContent(defragDuration time.Duration) (err error) {
	return this.handler.DefragContent(defragDuration)
}

func (this *Instance) Range(doFunc func(key string, exist bool)) (err error) {
	return this.handler.Range(doFunc)
}

type BlockHandler interface {
	Initiate(ctx context.Context, conf *config.Config) error
	Set(key string, value []byte) (err error)
	Get(key string) (value []byte, err error)
	Del(key string) (err error)
	DefragContent(defragDuration time.Duration) (err error)
	Range(doFunc func(key string, exist bool)) (err error)
}

type fileBlockHandler func() BlockHandler

var fileBlockHandlers map[string]fileBlockHandler = make(map[string]fileBlockHandler)

func FileBlockRegister(name string, handler fileBlockHandler) (err error) {
	if handler == nil {
		err = fmt.Errorf(errRegister)
		return
	}
	var ok bool
	if _, ok = fileBlockHandlers[name]; ok {
		return
	}
	fileBlockHandlers[name] = handler
	return
}

func FileBlockUse(name string) (handler fileBlockHandler, err error) {
	var ok bool
	if handler, ok = fileBlockHandlers[name]; !ok {
		err = fmt.Errorf(errUse, name)
		return
	}
	return
}
