package lru

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate  string = "lru: initiate '%s' error"
	errOnStartup string = "lru: onStartup '%s' error"

	errRegister string = "lru: Register template is nil"
	errUse      string = "lru: unknown template '%s' (forgotten register?)"
)

var (
	defaultProvider string = "lru"
)

func NewInstance() *Instance {
	return &Instance{
		Config: config.NewInstance(),
	}
}

type Instance struct {
	Config      *config.Instance
	handlerName string
	handler     Handler
}

func (this *Instance) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	if newCtx, err = this.Config.Initiate(ctx); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	var configs map[string]config.Option = map[string]config.Option{
		"LRU_PROVIDER":      config.Option{Type: config.String, Default: defaultProvider, EnvironmentKey: "LRU_PROVIDER"},
		"LRU_CAPACITY":      config.Option{Type: config.Int64, Default: 32 * 1024 * 1024, EnvironmentKey: "LRU_CAPACITY"},
		"LRU_DIR":           config.Option{Type: config.String, Default: "/tmp/lru", EnvironmentKey: "LRU_DIR"},
		"LRU_SAVE_INTERVAL": config.Option{Type: config.Duration, Default: 5 * time.Minute, EnvironmentKey: "LRU_SAVE_INTERVAL"},
	}
	if newCtx, err = this.Config.SetOptions(ctx, configs); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	if err = LRURegister(defaultProvider, NewLRUHandler); err != nil {
		err = fmt.Errorf(errInitiate, err.Error())
		return
	}
	return
}
func (this *Instance) OnStartup(ctx context.Context) (newCtx context.Context, err error) {
	newCtx = ctx
	var provider string = this.Config.GetString("LRU_PROVIDER")
	if provider == "" {
		provider = "lru"
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
	var handler lruHandler
	if handler, err = LRUUse(handlerName); err != nil {
		return
	}
	this.handlerName = handlerName
	this.handler = handler()
	if err = this.handler.Initiate(ctx, this.Config); err != nil {
		return
	}
	return
}

func (this *Instance) Lock() {
	this.handler.Lock()
}

func (this *Instance) Unlock() {
	this.handler.Unlock()
}

func (this *Instance) Peek(key string) (v []byte, ok bool) {
	return this.handler.Peek(key)
}

func (this *Instance) Update(key string, value []byte) {
	this.handler.Update(key, value)
}

func (this *Instance) Get(key string) (v []byte, ok bool) {
	return this.handler.Get(key)
}

func (this *Instance) Set(key string, value []byte) {
	this.handler.Set(key, value)
}

type Handler interface {
	Initiate(ctx context.Context, conf *config.Instance) error
	sync.Locker
	Peek(key string) (v []byte, ok bool)
	Update(key string, value []byte)
	Get(key string) (v []byte, ok bool)
	Set(key string, value []byte)
}

type lruHandler func() Handler

var lruHandlers map[string]lruHandler = make(map[string]lruHandler)

func LRURegister(name string, handler lruHandler) (err error) {
	if handler == nil {
		err = fmt.Errorf(errRegister)
		return
	}
	var ok bool
	if _, ok = lruHandlers[name]; ok {
		return
	}
	lruHandlers[name] = handler
	return
}

func LRUUse(name string) (handler lruHandler, err error) {
	var ok bool
	if handler, ok = lruHandlers[name]; !ok {
		err = fmt.Errorf(errUse, name)
		return
	}
	return
}
