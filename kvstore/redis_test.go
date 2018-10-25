package kvstore

import "testing"
import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"time"
)

var configs = map[string]interface{}{
	"CACHE_REDIS_HOST":       "127.0.0.1",
	"CACHE_REDIS_PORT":       "6379",
	"CACHE_REDIS_PASSWORD":   "",
	"CACHE_REDIS_DATABASE":   "0",
	"CACHE_REDIS_MAX_IDLE":   "100",
	"CACHE_REDIS_MAX_ACTIVE": "300",
	"CACHE_REDIS_KEY_PREFIX": "test",
}

func TestGet(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection(context.Background(), "test", configs)
	assert.Nil(t, err)
	k := "1503037240RBW1Ti"
	v := "01f9eeac5b2cb596b401677a77fd9b36.jpg"
	value, err := myredis.Get(k)
	assert.Nil(t, err, "redis get failure:"+err.Error())
	assert.Equal(t, value, v, fmt.Printf("myredis.Get(%s)!=%s", k, v))
}
func TestSet(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection(context.Background(), "test", configs)
	assert.Nil(t, err)
	k := "abc"
	v := "456"
	err = myredis.Set(k, v)
	assert.Nil(t, err, "redis set failure:"+err.Error())
	value, err := myredis.Get(k)
	assert.Nil(t, err, "redis get failure:"+err.Error())
	assert.Equal(t, value, v, fmt.Printf("myredis.Get(%s)!=%s", k, v))
}
func TestDel(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection(context.Background(), "test", configs)
	assert.Nil(t, err)
	k := "abc"
	v := "456"
	assert.Nil(t, err, "redis set failure:"+err.Error())
	value, err := myredis.Get(k)
	assert.Nil(t, err, "redis get failure:"+err.Error())
	assert.Equal(t, value, v, fmt.Printf("myredis.Get(%s)!=%s", k, v))
	_, err = myredis.Del(k)
	assert.Nil(t, err, "redis del failure:"+err.Error())
	_, err = myredis.Get(k)
	assert.Nil(t, err, "redis get failure:"+err.Error())
}
func TestSetExpire(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection(context.Background(), "test", configs)
	assert.Nil(t, err)
	k := "abc"
	v := "456"
	var ti int64
	ti = 3
	err = myredis.Set(k, v)
	assert.Nil(t, err, "redis set failure:"+err.Error())
	value, err := myredis.Get(k)
	assert.Nil(t, err, "redis get failure:"+err.Error())
	assert.Equal(t, value, v, fmt.Printf("myredis.Get(%s)!=%s", k, v))
	myredis.Expire(k, ti)
	time.Sleep(4 * time.Second)
	_, err = myredis.Get(k)
	assert.NotNil(t, err, "redis expire failure:"+err.Error())
}
