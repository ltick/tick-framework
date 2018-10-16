package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate = "cache: initiate '%s' error"
	errStartup  = "cache: startup '%s' error"
	errNewCache = "cache: new '%s' cache error"
	errGetCache = "cache: get '%s' cache error"
)

func NewCache() *Cache {
	instance := &Cache{
	}
	return instance
}

type Cache struct {
	Config      *config.Config
	handlerName string
	handler     Handler
}

func (c *Cache) Initiate(ctx context.Context) (context.Context, error) {
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
	err := c.Config.SetOptions(configs)
	if err != nil {
		return ctx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	return ctx, nil
}
func (c *Cache) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	err = Register("redis", NewRedisHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), c.handlerName))
	}
	cacheProvider := c.Config.GetString("CACHE_PROVIDER")
	if cacheProvider != "" {
		err = c.Use(ctx, cacheProvider)
	} else {
		err = c.Use(ctx, "redis")
	}
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), c.handlerName))
	}
	return ctx, nil
}
func (c *Cache) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (c *Cache) OnRequestStartup(ctx *routing.Context) error {
	return nil
}
func (c *Cache) OnRequestShutdown(ctx *routing.Context) error {
	return nil
}
func (c *Cache) HandlerName() string {
	return c.handlerName
}
func (c *Cache) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	c.handlerName = handlerName
	c.handler = handler()
	err = c.handler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), c.handlerName))
	}
	return nil
}
func (c *Cache) NewCache(ctx context.Context, name string, config map[string]interface{}) (CacheHandler, error) {
	cacheHandler, err := c.GetCache(name)
	if err == nil {
		return cacheHandler, nil
	}
	if _, ok := config["CACHE_REDIS_HOST"]; !ok {
		config["CACHE_REDIS_HOST"] = c.Config.GetString("CACHE_REDIS_HOST")
	}
	if _, ok := config["CACHE_REDIS_PORT"]; !ok {
		config["CACHE_REDIS_PORT"] = c.Config.GetString("CACHE_REDIS_PORT")
	}
	if _, ok := config["CACHE_REDIS_PASSWORD"]; !ok {
		config["CACHE_REDIS_PASSWORD"] = c.Config.GetString("CACHE_REDIS_PASSWORD")
	}
	if _, ok := config["CACHE_REDIS_DATABASE"]; !ok {
		config["CACHE_REDIS_DATABASE"] = c.Config.GetInt("CACHE_REDIS_DATABASE")
	}
	if _, ok := config["CACHE_REDIS_KEY_PREFIX"]; !ok {
		config["CACHE_REDIS_KEY_PREFIX"] = c.Config.GetString("CACHE_REDIS_KEY_PREFIX")
	}
	if _, ok := config["CACHE_REDIS_MAX_ACTIVE"]; !ok {
		config["CACHE_REDIS_MAX_ACTIVE"] = c.Config.GetInt("CACHE_REDIS_MAX_ACTIVE")
	}
	if _, ok := config["CACHE_REDIS_MAX_IDLE"]; !ok {
		config["CACHE_REDIS_MAX_IDLE"] = c.Config.GetInt("CACHE_REDIS_MAX_IDLE")
	}
	cacheHandler, err = c.handler.NewCache(ctx, name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewCache+": "+err.Error(), name))
	}
	if cacheHandler == nil {
		return nil, errors.New(fmt.Sprintf(errNewCache+": empty pool", name))
	}
	return cacheHandler, nil
}
func (c *Cache) GetCache(name string) (CacheHandler, error) {
	cacheHandler, err := c.handler.GetCache(name)
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
	Hmset(key interface{}, args ...interface{}) error
	Hmget(key interface{}, args ...interface{}) (interface{}, error)
	Hget(key interface{}, field interface{}) (interface{}, error)
	Hdel(key interface{}, field interface{}) (interface{}, error)
	Hgetall(key interface{}) (interface{}, error)
	Exists(key interface{}) (bool, error)
	ScanStruct(src []interface{}, dest interface{}) error
	Sadd(key interface{}, args ...interface{}) error
	Scard(key interface{}) (int64, error)
	Zadd(key interface{}, args ...interface{}) error
	Zrem(key interface{}, field interface{}) (interface{}, error)
	Zrange(key interface{}, start interface{}, end interface{}) (interface{}, error)
	Zscore(key interface{}, field interface{}) (interface{}, error)
	Zcard(key interface{}) (int64, error)
	Zscan(key interface{}, cursor string, match string, count int64) (nextCursor string, keys []string, err error)
	Sscan(key interface{}, cursor string, match string, count int64) (interface{}, error)
	Hscan(key interface{}, cursor string, match string, count int64) (interface{}, error)
	Scan(cursor string, match string, count int64) (nextCursor string, keys []string, err error)
	Sort(key interface{}, by interface{}, offest int64, count int64, asc *bool, alpha *bool, get ...interface{}) ([]string, error)
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

func ErrNil(err error) bool {
	return RedisErrNil(err)
}
