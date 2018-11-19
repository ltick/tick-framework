package kvstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-routing"
	"strings"
)

var (
	errInitiate      = "kvstore: initiate '%s' error"
	errStartup       = "kvstore: startup '%s' error"
	errNewConnection = "kvstore: new '%s' kvstore error"
	errGetConnection = "kvstore: get '%s' kvstore error"
)

func NewKvstore() *Kvstore {
	instance := &Kvstore{}
	return instance
}

type Kvstore struct {
	Config   *config.Config `inject:"true"`
	Provider string
	handler  Handler
}

func (c *Kvstore) Initiate(ctx context.Context) (context.Context, error) {
	var configs map[string]config.Option = map[string]config.Option{
		"KVSTORE_PROVIDER":         config.Option{Type: config.String, EnvironmentKey: "KVSTORE_PROVIDER"},
		"KVSTORE_REDIS_HOST":       config.Option{Type: config.String, EnvironmentKey: "KVSTORE_REDIS_HOST"},
		"KVSTORE_REDIS_PORT":       config.Option{Type: config.String, EnvironmentKey: "KVSTORE_REDIS_PORT"},
		"KVSTORE_REDIS_PASSWORD":   config.Option{Type: config.String, EnvironmentKey: "KVSTORE_REDIS_PASSWORD"},
		"KVSTORE_REDIS_DATABASE":   config.Option{Type: config.Int, EnvironmentKey: "KVSTORE_REDIS_DATABASE"},
		"KVSTORE_REDIS_MAX_IDLE":   config.Option{Type: config.Int, EnvironmentKey: "KVSTORE_REDIS_MAX_IDLE"},
		"KVSTORE_REDIS_MAX_ACTIVE": config.Option{Type: config.Int, EnvironmentKey: "KVSTORE_REDIS_MAX_ACTIVE"},
		"KVSTORE_REDIS_KEY_PREFIX": config.Option{Type: config.String, EnvironmentKey: "KVSTORE_REDIS_KEY_PREFIX"},
	}
	err := c.Config.SetOptions(configs)
	if err != nil {
		return ctx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	err = Register("redis", NewRedisHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate + ": " + err.Error()))
	}
	err = c.Use(ctx, "redis")
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate + ": " + err.Error()))
	}
	return ctx, nil
}
func (c *Kvstore) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	err = Register("redis", NewRedisHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), c.Provider))
	}
	if kvstoreProvider := c.Config.GetString("KVSTORE_PROVIDER"); kvstoreProvider != "" {
		err = c.Use(ctx, kvstoreProvider)
		if err != nil {
			return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), c.Provider))
		}
	}
	return ctx, nil
}
func (c *Kvstore) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (c *Kvstore) OnRequestStartup(ctx *routing.Context) error {
	return nil
}
func (c *Kvstore) OnRequestShutdown(ctx *routing.Context) error {
	return nil
}
func (c *Kvstore) GetProvider() string {
	return c.Provider
}
func (c *Kvstore) Use(ctx context.Context, Provider string) error {
	handler, err := Use(Provider)
	if err != nil {
		return err
	}
	c.Provider = Provider
	c.handler = handler()
	err = c.handler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), c.Provider))
	}
	return nil
}
func (c *Kvstore) NewConnection(name string, config map[string]interface{}) (KvstoreHandler, error) {
	kvstoreHandler, err := c.GetConnection(name)
	if err == nil {
		return kvstoreHandler, nil
	}
	if _, ok := config["KVSTORE_REDIS_HOST"]; !ok {
		config["KVSTORE_REDIS_HOST"] = c.Config.GetString("KVSTORE_REDIS_HOST")
	}
	if _, ok := config["KVSTORE_REDIS_PORT"]; !ok {
		config["KVSTORE_REDIS_PORT"] = c.Config.GetString("KVSTORE_REDIS_PORT")
	}
	if _, ok := config["KVSTORE_REDIS_PASSWORD"]; !ok {
		config["KVSTORE_REDIS_PASSWORD"] = c.Config.GetString("KVSTORE_REDIS_PASSWORD")
	}
	if _, ok := config["KVSTORE_REDIS_DATABASE"]; !ok {
		config["KVSTORE_REDIS_DATABASE"] = c.Config.GetInt("KVSTORE_REDIS_DATABASE")
	}
	if _, ok := config["KVSTORE_REDIS_KEY_PREFIX"]; !ok {
		config["KVSTORE_REDIS_KEY_PREFIX"] = c.Config.GetString("KVSTORE_REDIS_KEY_PREFIX")
	}
	if _, ok := config["KVSTORE_REDIS_MAX_ACTIVE"]; !ok {
		config["KVSTORE_REDIS_MAX_ACTIVE"] = c.Config.GetInt("KVSTORE_REDIS_MAX_ACTIVE")
	}
	if _, ok := config["KVSTORE_REDIS_MAX_IDLE"]; !ok {
		config["KVSTORE_REDIS_MAX_IDLE"] = c.Config.GetInt("KVSTORE_REDIS_MAX_IDLE")
	}
	kvstoreHandler, err = c.handler.NewConnection(name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewConnection+": "+err.Error(), name))
	}
	if kvstoreHandler == nil {
		return nil, errors.New(fmt.Sprintf(errNewConnection+": empty pool", name))
	}
	return kvstoreHandler, nil
}
func (c *Kvstore) GetConnection(name string) (KvstoreHandler, error) {
	kvstoreHandler, err := c.handler.GetConnection(name)
	if err != nil {
		if ConnectionNotExists(err) {
			kvstoreHandler, err = c.handler.NewConnection(name, map[string]interface{}{})
		}
		return nil, errors.New(fmt.Sprintf(errGetConnection+": "+err.Error(), name))
	}
	return kvstoreHandler, err
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewConnection(name string, config map[string]interface{}) (KvstoreHandler, error)
	GetConnection(name string) (KvstoreHandler, error)
}

type KvstoreHandler interface {
	GetConfig() map[string]interface{}
	Set(key interface{}, value interface{}) error
	Get(key interface{}) (interface{}, error)
	Keys(key interface{}) (interface{}, error)
	Expire(key interface{}, expire int64) error
	Hmset(key interface{}, value ...interface{}) error
	Hmget(key interface{}, value ...interface{}) (interface{}, error)
	Del(key interface{}) (interface{}, error)
	Hset(key interface{}, field interface{}, value interface{}) error
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

type kvstoreHandler func() Handler

var kvstoreHandlers = make(map[string]kvstoreHandler)

func Register(name string, kvstoreHandler kvstoreHandler) error {
	if kvstoreHandler == nil {
		return errors.New("kvstore: Register kvstore handler is nil")
	}
	if _, ok := kvstoreHandlers[name]; !ok {
		kvstoreHandlers[name] = kvstoreHandler
	}
	return nil
}
func Use(name string) (kvstoreHandler, error) {
	if _, exist := kvstoreHandlers[name]; !exist {
		return nil, errors.New("kvstore: unknown kvstore " + name + " (forgotten register?)")
	}
	return kvstoreHandlers[name], nil
}

func ErrNil(err error) bool {
	return RedisErrNil(err)
}

func ConnectionNotExists(err error) bool {
	return strings.Contains(err.Error(), errRedisConnectionNotExists)
}
