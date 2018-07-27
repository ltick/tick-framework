package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate = "cache: initiate '%s' error"
	errStartup  = "cache: startup '%s' error"
	errNewCache = "cache: new '%s' cache error"
	errGetCache = "cache: get '%s' cache error"
)

func NewInstance() *Instance {
	instance := &Instance{
		Utility: &utility.Instance{},
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
		"CACHE_PROVIDER":         config.Option{Type: config.String, EnvironmentKey: "CACHE_PROVIDER"},
		"CACHE_REDIS_HOST":       config.Option{Type: config.String, EnvironmentKey: "CACHE_REDIS_HOST"},
		"CACHE_REDIS_PORT":       config.Option{Type: config.String, EnvironmentKey: "CACHE_REDIS_PORT"},
		"CACHE_REDIS_PASSWORD":   config.Option{Type: config.String, EnvironmentKey: "CACHE_REDIS_PASSWORD"},
		"CACHE_REDIS_DATABASE":   config.Option{Type: config.Int, EnvironmentKey: "CACHE_REDIS_DATABASE"},
		"CACHE_REDIS_MAX_IDLE":   config.Option{Type: config.Int, EnvironmentKey: "CACHE_REDIS_MAX_IDLE"},
		"CACHE_REDIS_MAX_ACTIVE": config.Option{Type: config.Int, EnvironmentKey: "CACHE_REDIS_MAX_ACTIVE"},
		"CACHE_REDIS_KEY_PREFIX": config.Option{Type: config.String, EnvironmentKey: "CACHE_REDIS_KEY_PREFIX"},
	}
	newCtx, err = this.Config.SetOptions(ctx, configs)
	if err != nil {
		return newCtx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	return newCtx, nil
}
func (this *Instance) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	err = Register("redis", NewRedisHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), this.handlerName))
	}
	cacheProvider := this.Config.GetString("CACHE_PROVIDER")
	if cacheProvider != "" {
		err = this.Use(ctx, cacheProvider)
	} else {
		err = this.Use(ctx, "redis")
	}
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), this.handlerName))
	}
	return ctx, nil
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
	err = this.handler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return nil
}
func (this *Instance) NewCache(ctx context.Context, name string) (CacheHandler, error) {
	cacheHandler, err := this.GetCache(name)
	if err == nil {
		return cacheHandler, nil
	}
	config := map[string]interface{}{
		"CACHE_REDIS_HOST":       this.Config.GetString("CACHE_REDIS_HOST"),
		"CACHE_REDIS_PORT":       this.Config.GetString("CACHE_REDIS_PORT"),
		"CACHE_REDIS_PASSWORD":   this.Config.GetString("CACHE_REDIS_PASSWORD"),
		"CACHE_REDIS_DATABASE":   this.Config.GetInt("CACHE_REDIS_DATABASE"),
		"CACHE_REDIS_KEY_PREFIX": this.Config.GetString("CACHE_REDIS_KEY_PREFIX"),
		"CACHE_REDIS_MAX_ACTIVE": this.Config.GetInt("CACHE_REDIS_MAX_ACTIVE"),
		"CACHE_REDIS_MAX_IDLE":   this.Config.GetInt("CACHE_REDIS_MAX_IDLE"),
	}
	cacheHandler, err = this.handler.NewCache(ctx, name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewCache+": "+err.Error(), name))
	}
	if cacheHandler == nil {
		return nil, errors.New(fmt.Sprintf(errNewCache+": empty pool", name))
	}
	return cacheHandler, nil
}
func (this *Instance) GetCache(name string) (CacheHandler, error) {
	cacheHandler, err := this.handler.GetCache(name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errGetCache+": "+err.Error(), name))
	}
	return cacheHandler, err
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewCache(ctx context.Context, name string, config map[string]interface{}) (CacheHandler, error)
	GetCache(name string) (CacheHandler, error)
}

type CacheHandler interface {
	GetConfig() map[string]interface{}
	Set(key interface{}, value interface{}) error
	Get(key interface{}) (interface{}, error)
	Keys(key interface{}) (interface{}, error)
	Expire(key interface{}, expire int64) error
	Del(key interface{}) (interface{}, error)
	Hset(key interface{}, field interface{}, value interface{}) error
	Hget(key interface{}, field interface{}) (interface{}, error)
	Hgetall(key interface{}) (interface{}, error)
	Exists(key interface{}) (bool, error)
	ScanStruct(src []interface{}, dest interface{}) error
	Sadd(key interface{}, value interface{}) error
}

type cacheHandler func() Handler

var cacheHandlers = make(map[string]cacheHandler)

func Register(name string, cacheHandler cacheHandler) error {
	if cacheHandler == nil {
		return errors.New("cache: Register cache handler is nil")
	}
	if _, ok := cacheHandlers[name]; !ok {
		cacheHandlers[name] = cacheHandler
	}
	return nil
}
func Use(name string) (cacheHandler, error) {
	if _, exist := cacheHandlers[name]; !exist {
		return nil, errors.New("cache: unknown cache " + name + " (forgotten register?)")
	}
	return cacheHandlers[name], nil
}
