package block

import (
	"context"
	"fmt"
	"time"

	"github.com/ltick/tick-framework/module/config"
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
		Config: config.NewInstance(),
	}
}

type Instance struct {
	Config      *config.Instance
	handlerName string
	handler     BlockHandler
}

func (this *Instance) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	if newCtx, err = this.Config.Initiate(ctx); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	var configs map[string]config.Option = map[string]config.Option{
		"FILESYSTEM_BLOCK_PROVIDER": config.Option{Type: config.String, Default: defaultProvider, EnvironmentKey: "FILESYSTEM_BLOCK_PROVIDER"},
		"FILESYSTEM_BLOCK_DIR":      config.Option{Type: config.String, Default: "/tmp/block", EnvironmentKey: "FILESYSTEM_BLOCK_DIR"},
		"FILESYSTEM_BLOCK_SIZE":     config.Option{Type: config.Int64, Default: 64 * 1024 * 1024, EnvironmentKey: "FILESYSTEM_BLOCK_SIZE"},
		"FILESYSTEM_BLOCK_IDLE":     config.Option{Type: config.Int, Default: 32, EnvironmentKey: "FILESYSTEM_BLOCK_IDLE"},
	}
	if newCtx, err = this.Config.SetOptions(ctx, configs); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	if err = FileBlockRegister(defaultProvider, NewFileBlockHandler); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	return
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

func (this *Instance) Read(index *Index) (data []byte, err error) {
	return this.handler.Read(index)
}

func (this *Instance) Write(key, value []byte) (index *Index, err error) {
	return this.handler.Write(key, value)
}

func (this *Instance) Defrag(defragLifetime time.Duration, rebuildIndex func(key string, index *Index)) {
	this.handler.Defrag(defragLifetime, rebuildIndex)
}

type BlockHandler interface {
	Initiate(ctx context.Context, conf *config.Instance) error
	Read(index *Index) (data []byte, err error)
	Write(key, value []byte) (index *Index, err error)
	Defrag(defragDuration time.Duration, rebuildIndex func(key string, index *Index))
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
