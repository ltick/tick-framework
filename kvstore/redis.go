package kvstore

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"fmt"

	"github.com/gomodule/redigo/redis"
	"os"
)

var (
	errRedisNewHandler            = "kvstore(redis): new handler error"
	errRedisConnectionNotExists   = "kvstore(redis): '%s' handler not exists"
	errRedisZscanCursorTypeError  = "kvstore(redis): zscan cursor type error"
	errRedisZscanValueTypeError   = "kvstore(redis): zscan value type error"
	errRedisZscanValueLengthError = "kvstore(redis): zscan value length error"
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

func (this *RedisHandler) NewHandler(name string, config map[string]interface{}) (KvstoreHandler, error) {
	pool := &RedisPool{Pool: &redis.Pool{}}
	configHost := config["KVSTORE_REDIS_HOST"]
	if configHost != nil {
		host, ok := configHost.(string)
		if ok {
			pool.Host = host
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_HOST data type must be string")
		}
	}
	configPort := config["KVSTORE_REDIS_PORT"]
	if configPort != nil {
		port, ok := configPort.(string)
		if ok {
			pool.Port = port
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_PORT data type must be string")
		}
	}
	configPassword := config["KVSTORE_REDIS_PASSWORD"]
	if configPassword != nil {
		password, ok := configPassword.(string)
		if ok {
			pool.Password = password
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_PASSWORD data type must be string")
		}
	}
	configDatabase := config["KVSTORE_REDIS_DATABASE"]
	if configDatabase != nil {
		database, ok := configDatabase.(int)
		if ok {
			pool.Database = database
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_DATABASE data type must be int")
		}
	}
	configKeyPrefix := config["KVSTORE_REDIS_KEY_PREFIX"]
	if configKeyPrefix != nil {
		keyPrefix, ok := configKeyPrefix.(string)
		if ok {
			pool.KeyPrefix = keyPrefix
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_KEY_PREFIX data type must be string")
		}
	}
	configMaxActive := config["KVSTORE_REDIS_MAX_ACTIVE"]
	if configMaxActive != nil {
		maxActive, ok := configMaxActive.(int)
		if ok {
			pool.MaxActive = maxActive
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_MAX_ACTIVE data type must be int")
		}
	}
	configMaxIdle := config["KVSTORE_REDIS_MAX_IDLE"]
	if configMaxIdle != nil {
		maxIdle, ok := configMaxIdle.(int)
		if ok {
			pool.MaxIdle = maxIdle
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_MAX_IDLE data type must be int")
		}
	}
	configDebug := config["KVSTORE_REDIS_DEBUG"]
	if configDebug != nil {
		debug, ok := configDebug.(bool)
		if ok {
			pool.Debug = debug
		} else {
			return nil, errors.New(errRedisNewHandler + ": KVSTORE_REDIS_DEBUG data type must be bool")
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
	return nil, errors.New(errRedisNewHandler + ": pool.Host is empty")
}

func (this *RedisHandler) GetHandler(name string) (KvstoreHandler, error) {
	if this.pools == nil {
		return nil, errors.New(fmt.Sprintf(errRedisConnectionNotExists, name))
	}
	handlerPool, ok := this.pools[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf(errRedisConnectionNotExists, name))
	}
	return handlerPool, nil
}

type RedisPool struct {
	*redis.Pool
	Host      string
	Port      string
	Password  string
	Database  int
	KeyPrefix string
	Debug     bool
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
		"debug":      this.Debug,
	}
}

func (this *RedisPool) Get(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}
	redisArgs := redis.Args{}.Add(sKey)
	for i := 0; i < len(args); i = i + 2 {
		redisArgs = redisArgs.Add(args[i]).Add(args[i+1])
	}
	_, err = c.Do("HMSET", redisArgs...)
	return err
}
func (this *RedisPool) Hmget(key interface{}, args ...interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
func (this *RedisPool) Hlen(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	count, err := c.Do("HLEN", sKey)
	if err != nil {
		return nil, err
	}
	return count, nil
}
func (this *RedisPool) Hdel(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("HDEL", sKey, field)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Hgetall(key interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
func (this *RedisPool) Sadd(key interface{}, args ...interface{}) error {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return err
	}
	redisArgs := redis.Args{}.Add(sKey)
	for _, arg := range args {
		redisArgs = redisArgs.AddFlat(arg)
	}
	_, err = c.Do("SADD", redisArgs...)
	return err
}
func (this *RedisPool) Scard(key interface{}) (int64, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return 0, err
	}
	return redis.Int64(c.Do("SCARD", sKey))
}
func (this *RedisPool) Zadd(key interface{}, value ...interface{}) error {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
func (this *RedisPool) Zcard(key interface{}) (int64, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return 0, err
	}
	return redis.Int64(c.Do("ZCARD", sKey))
	//return count, err
}
func (this *RedisPool) Zscore(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
func (this *RedisPool) Zrem(key interface{}, field interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
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
func (this *RedisPool) Zrange(key interface{}, start, end interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("ZRANGE", sKey, start, end)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Zrevrange(key interface{}, start, end interface{}) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	value, err := c.Do("ZREVRANGE", sKey, start, end)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) ZrangeByScore(key interface{}, min, max interface{}, limits ...interface{}) (value interface{}, err error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	args := redis.Args{}.Add(sKey).Add(min).Add(max).Add("WITHSCORES")
	if len(limits) > 0 {
		args = args.Add("LIMIT").Add(limits...)
	}
	if len(limits) > 0 {
		args = args.Add("LIMIT").Add(limits...)
	}
	value, err = c.Do("ZRANGEBYSCORE", args...)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) ZrevrangeByScore(key interface{}, min, max interface{}, limits ...interface{}) (value interface{}, err error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	args := redis.Args{}.Add(sKey).Add(max).Add(min).Add("WITHSCORES")
	if len(limits) > 0 {
		args = args.Add("LIMIT").Add(limits...)
	}
	value, err = c.Do("ZREVRANGEBYSCORE", args...)
	if err != nil {
		return nil, err
	}
	return value, nil
}
func (this *RedisPool) Zscan(key interface{}, cursor string, match string, count int64) (nextCursor string, keys []string, err error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return "", nil, err
	}
	args := redis.Args{}.Add(sKey).Add(cursor)
	if match != "" {
		args = args.Add("MATCH", match)
	}
	if count == 0 {
		count = 1000
	}
	if count > 0 {
		args = args.Add("COUNT", count)
	}
	values, err := redis.Values(c.Do("ZSCAN", args...))
	if err != nil {
		return "", nil, err
	}
	if len(values) == 2 {
		nextCursor = ""
		if cursorByte, ok := values[0].([]uint8); ok {
			nextCursor = string(cursorByte)
		} else {
			return "", nil, errors.New(errRedisZscanCursorTypeError)
		}
		keys = make([]string, 0)
		if keysArr, ok := values[1].([]interface{}); ok {
			for _, keyInterface := range keysArr {
				if keyByte, ok := keyInterface.([]uint8); ok {
					keys = append(keys, string(keyByte))
				} else {
					return "", nil, errors.New(errRedisZscanCursorTypeError)
				}
			}
			return nextCursor, keys, nil
		} else {
			return "", nil, errors.New(errRedisZscanValueTypeError)
		}
	} else {
		return "", nil, errors.New(errRedisZscanValueLengthError)
	}
}
func (this *RedisPool) Sscan(key interface{}, cursor string, match string, count int64) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	args := redis.Args{}.Add(sKey).Add(cursor)
	if match != "" {
		args = args.Add("MATCH", match)
	}
	if count == 0 {
		count = 1000
	}
	if count > 0 {
		args = args.Add("COUNT", count)
	}
	return c.Do("SSCAN", args...)
}
func (this *RedisPool) Hscan(key interface{}, cursor string, match string, count int64) (interface{}, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	args := redis.Args{}.Add(sKey).Add(cursor)
	if match != "" {
		args = args.Add("MATCH", match)
	}
	if count == 0 {
		count = 1000
	}
	if count > 0 {
		args = args.Add("COUNT", count)
	}
	return c.Do("HSCAN", args...)
}
func (this *RedisPool) Scan(cursor string, match string, count int64) (nextCursor string, keys []string, err error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	args := redis.Args{}.Add(cursor)
	if match != "" {
		args = args.Add("MATCH", match)
	}
	if count > 0 {
		args = args.Add("COUNT", count)
	}
	values, err := redis.Values(c.Do("SCAN", args...))
	if err != nil {
		return "", nil, err
	}
	if len(values) == 2 {
		nextCursor := ""
		if cursorByte, ok := values[0].([]uint8); ok {
			nextCursor = string(cursorByte)
		} else {
			return "", nil, errors.New(errRedisZscanCursorTypeError)
		}
		keys = make([]string, 0)
		if keysArr, ok := values[1].([]interface{}); ok {
			for _, keyInterface := range keysArr {
				if keyByte, ok := keyInterface.([]uint8); ok {
					keys = append(keys, string(keyByte))
				} else {
					return "", nil, errors.New(errRedisZscanCursorTypeError)
				}
			}
			return nextCursor, keys, nil
		} else {
			return "", nil, errors.New(errRedisZscanValueTypeError)
		}
	} else {
		return "", nil, errors.New(errRedisZscanValueLengthError)
	}
}
func (this *RedisPool) Sort(key interface{}, by interface{}, offest int64, count int64, desc *bool, alpha *bool, gets ...interface{}) ([]string, error) {
	c := this.Pool.Get()
	if this.Debug {
		c = redis.NewLoggingConn(c, log.New(os.Stdout, "", log.LstdFlags), "")
	}
	defer c.Close()
	sKey, err := this.generateKey(key)
	if err != nil {
		return nil, err
	}
	args := redis.Args{}.Add(sKey)
	if by != nil {
		args = args.Add("BY", by)
	}
	if len(gets) > 0 {
		for _, get := range gets {
			if get != "" {
				args = args.Add("GET", get)
			}
		}
	}
	if count == 0 {
		count = 1000
	}
	args = append(args, "LIMIT", offest, count)
	if desc != nil && *desc {
		args = args.Add("DESC")
	}
	if alpha != nil && *alpha {
		args = args.Add("ALPHA")
	}
	return redis.Strings(c.Do("SORT", args...))
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

func RedisErrNil(err error) bool {
	return strings.Contains(err.Error(), redis.ErrNil.Error())
}
