package kvstore

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var configs = map[string]interface{}{
	"KVSTORE_REDIS_HOST":       "127.0.0.1",
	"KVSTORE_REDIS_PORT":       "6379",
	"KVSTORE_REDIS_PASSWORD":   "",
	"KVSTORE_REDIS_DATABASE":   0,
	"KVSTORE_REDIS_MAX_IDLE":   100,
	"KVSTORE_REDIS_MAX_ACTIVE": 300,
	"KVSTORE_REDIS_KEY_PREFIX": "test",
}

func TestGet(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection("test", configs)
	assert.Nil(t, err)
	if err == nil {
		k := "1503037240RBW1Ti"
		v := "01f9eeac5b2cb596b401677a77fd9b36.jpg"
		value, err := myredis.Get(k)
		assert.Nil(t, err, "redis get failure:"+err.Error())
		assert.Equal(t, value, v, fmt.Sprintf("myredis.Get(%s)!=%s", k, v))
	}
}
func TestSet(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection("test", configs)
	assert.Nil(t, err)
	if err == nil {
		k := "abc"
		v := "456"
		err = myredis.Set(k, v)
		assert.Nil(t, err, "redis set failure:"+err.Error())
		value, err := myredis.Get(k)
		assert.Nil(t, err, "redis get failure:"+err.Error())
		assert.Equal(t, value, v, fmt.Sprintf("myredis.Get(%s)!=%s", k, v))
	}
}
func TestDel(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection("test", configs)
	assert.Nil(t, err)
	if err == nil {
		k := "abc"
		v := "456"
		assert.Nil(t, err, "redis set failure:"+err.Error())
		value, err := myredis.Get(k)
		assert.Nil(t, err, "redis get failure:"+err.Error())
		assert.Equal(t, value, v, fmt.Sprintf("myredis.Get(%s)!=%s", k, v))
		_, err = myredis.Del(k)
		assert.Nil(t, err, "redis del failure:"+err.Error())
		_, err = myredis.Get(k)
		assert.Nil(t, err, "redis get failure:"+err.Error())
	}
}
func TestSetExpire(t *testing.T) {
	redisHandler := NewRedisHandler()
	myredis, err := redisHandler.NewConnection("test", configs)
	assert.Nil(t, err)
	if err == nil {
		k := "abc"
		v := "456"
		var ti int64
		ti = 3
		err = myredis.Set(k, v)
		assert.Nil(t, err, "redis set failure:"+err.Error())
		value, err := myredis.Get(k)
		assert.Nil(t, err, "redis get failure:"+err.Error())
		assert.Equal(t, value, v, fmt.Sprintf("myredis.Get(%s)!=%s", k, v))
		myredis.Expire(k, ti)
		time.Sleep(4 * time.Second)
		_, err = myredis.Get(k)
		assert.NotNil(t, err, "redis expire failure:"+err.Error())
	}
}
