package cache

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"fmt"

	"github.com/gomodule/redigo/redis"
)

var (
	errRedisRegister      = "cache(redis): register error"
	errRedisNewCache      = "cache(redis): new pool error"
	errRedisPoolNotExists = "cache(redis): pool '%s' not exists"
)

type RedisHandler struct {
	pools map[string]*RedisPool
}

func NewRedisHandler() Handler {
	return &RedisHandler{}
}

func (this *RedisHandler) Initiate(ctx context.Context) error {
	this.pools = make(map[string]*RedisPool)
	return nil
}

func (this *RedisHandler) NewCache(ctx context.Context, name string, config map[string]interface{}) (CacheHandler, error) {
	pool := &RedisPool{}
	configHost := config["CACHE_REDIS_HOST"]
	if configHost != nil {
		host, ok := configHost.(string)
		if ok {
			pool.Host = host
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_HOST data type must be string")
		}
	}
	configPort := config["CACHE_REDIS_PORT"]
	if configPort != nil {
		port, ok := configPort.(string)
		if ok {
			pool.Port = port
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_PORT data type must be string")
		}
	}
	configPassword := config["CACHE_REDIS_PASSWORD"]
	if configPassword != nil {
		password, ok := configPassword.(string)
		if ok {
			pool.Password = password
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_PASSWORD data type must be string")
		}
	}
	configDatabase := config["CACHE_REDIS_DATABASE"]
	if configDatabase != nil {
		database, ok := configDatabase.(int)
		if ok {
			pool.Database = database
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_DATABASE data type must be int")
		}
	}
	configKeyPrefix := config["CACHE_REDIS_KEY_PREFIX"]
	if configKeyPrefix != nil {
		keyPrefix, ok := configKeyPrefix.(string)
		if ok {
			pool.KeyPrefix = keyPrefix
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_KEY_PREFIX data type must be string")
		}
	}
	configMaxActive := config["CACHE_REDIS_MAX_ACTIVE"]
	if configMaxActive != nil {
		maxActive, ok := configMaxActive.(int)
		if ok {
			pool.MaxActive = maxActive
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_MAX_ACTIVE data type must be int")
		}
	}
	configMaxIdle := config["CACHE_REDIS_MAX_IDLE"]
	if configMaxIdle != nil {
		maxIdle, ok := configMaxIdle.(int)
		if ok {
			pool.MaxIdle = maxIdle
		} else {
			return nil, errors.New(errRedisNewCache + ": CACHE_REDIS_MAX_IDLE data type must be int")
		}
	}
	if pool.Host != "" {
		pool.Pool = &redis.Pool{
			MaxIdle:   pool.MaxIdle,
			MaxActive: pool.MaxActive,
			Dial: func() (conn redis.Conn, err error) {
				c, err := redis.Dial("tcp",
					pool.Host+":"+pool.Port,
					redis.DialPassword(pool.Password),
					redis.DialDatabase(pool.Database),
				)
				if err != nil {
					return nil, err
				}
				return c, nil
			},
		}
		if this.pools == nil {
			this.pools = make(map[string]*RedisPool)
		}
		this.pools[name] = pool
		return pool, nil
	}
	return nil, nil
}

func (this *RedisHandler) GetCache(name string) (CacheHandler, error) {
	if this.pools == nil {
		return nil, errors.New(fmt.Sprintf(errRedisPoolNotExists, name))
	}
	handlerPool, ok := this.pools[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf(errRedisPoolNotExists, name))
	}
	return handlerPool, nil
}

type RedisPool struct {
	Host      string
	Port      string
	Password  string
	Database  int
	MaxActive int
	MaxIdle   int
	KeyPrefix string
	Pool      *redis.Pool
}

func (this *RedisPool) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":       this.Host,
		"port":       this.Port,
		"password":   this.Password,
		"database":   this.Database,
		"max_idle":   this.MaxIdle,
		"max_active": this.MaxActive,
		"prefix":     this.KeyPrefix,
	}
}

func (this *RedisPool) Get(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := redis.String(c.Do("GET", sKey))
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (this *RedisPool) Set(key interface{}, value interface{}) error {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}
	_, err = c.Do("SET", sKey, value)
	return err
}
func (this *RedisPool) Del(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("DEL", sKey)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Keys(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := redis.Strings(c.Do("KEYS", sKey))
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Expire(key interface{}, expire int64) error {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}
	_, err = c.Do("EXPIRE", sKey, expire)
	return err
}
func (this *RedisPool) Hmset(key interface{}, args ...interface{}) error {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}

	redisArgs := redis.Args{}
	redisArgs = redisArgs.Add(sKey)
	for i := 0; i < len(args); i = i + 2 {
		redisArgs = redisArgs.Add(args[i])
		redisArgs = redisArgs.AddFlat(args[i+1])
	}

	_, err = c.Do("HMSET", redisArgs...)
	return err
}
func (this *RedisPool) Hmget(key interface{}, args ...interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	redisArgs := redis.Args{}
	redisArgs = redisArgs.Add(sKey)
	for i := 0; i < len(args); i = i + 1 {
		redisArgs = redisArgs.Add(args[i])
	}
	values, err := c.Do("HMGET", redisArgs...)
	if err != nil {
		return nil, err
	}
	return values, nil
}
func (this *RedisPool) Hset(key interface{}, field interface{}, value interface{}) error {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}
	_, err = c.Do("HSET", sKey, field, value)
	return err
}
func (this *RedisPool) Hget(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("HGET", sKey, field)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Hdel(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	reply, err := c.Do("HDEL", sKey, field)
	if err != nil {
		return nil, err
	}
	return reply, nil
}
func (this *RedisPool) Hgetall(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := redis.Values(c.Do("HGETALL", sKey))
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) ScanStruct(src []interface{}, dest interface{}) error {
	return redis.ScanStruct(src, dest)
}

func (this *RedisPool) Exists(key interface{}) (bool, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return false, err
	}
	value, err := redis.Int(c.Do("EXISTS", sKey))
	if err != nil {
		return false, err
	}
	if value == 1 {
		return true, nil
	} else {
		return false, nil
	}
}
func (this *RedisPool) Sadd(key interface{}, value interface{}) error {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}
	_, err = c.Do("SADD", sKey, value)
	return err
}
func (this *RedisPool) Zadd(key interface{}, value ...interface{}) error {
	c := this.Pool.Get()
	defer c.Close()

	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}

	var valueLen int
	for _, v := range value {
		if v == nil {
			continue
		}
		valueLen++
	}

	var inputs []interface{}
	if valueLen > 0 {
		inputs = append(inputs, sKey)
		inputs = append(inputs, value...)

		_, err = c.Do("ZADD", inputs...)
		return err
	}

	return nil
}
func (this *RedisPool) Zscore(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("ZSCORE", sKey, field)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Zrange(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("ZREVRANGEBYSCORE", sKey, "+inf", "-inf", "WITHSCORES")
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Zrem(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	res, err := c.Do("ZREM", sKey, field)
	if err != nil {
		return nil, err
	}
	return res, nil
}
func (this *RedisPool) generateKey(key interface{}) (rKey string, err error) {
	switch key.(type) {
	case string:
		rKey = this.KeyPrefix + key.(string)
	case int:
		rKey = this.KeyPrefix + strconv.FormatInt(key.(int64), 10)
	case int64:
		rKey = this.KeyPrefix + strconv.FormatInt(key.(int64), 10)
	case []byte:
		rKey = this.KeyPrefix + string(key.([]byte))
	default:
		return "", errors.New("key type not support")
	}
	return rKey, nil
}

func ErrNil(err error) bool {
	return strings.Contains(err.Error(), redis.ErrNil.Error())
}
