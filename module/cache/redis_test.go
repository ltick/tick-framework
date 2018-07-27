package cache

import "testing"
import "time"

var config = map[string]string{
	"host":      "127.0.0.1",
	"port":      "6379",
	"db":        "0",
	"auth":      "",
	"timeout":   "500ms",
	"maxIdle":   "100",
	"maxActive": "300",
}

func TestGet(t *testing.T) {
	myredis := NewRedis(config)
	k := "1503037240RBW1Ti"
	v := "01f9eeac5b2cb596b401677a77fd9b36.jpg"
	value, err := myredis.Get(k)
	if err != nil {
		t.Error("redis get failure:", err)
	}
	if value != v {
		t.Errorf("myredis.Get(%s)!=%s", k, v)
	}
}
func TestSet(t *testing.T) {
	myredis := NewRedis(config)
	k := "abc"
	v := "456"
	err := myredis.Set(k, v)
	if err != nil {
		t.Errorf("redis set failure:", err)
	}
	value, err := myredis.Get(k)
	if err != nil {
		t.Errorf("redis get failure:", err)
	}
	if value != v {
		t.Errorf("myredis.Get(%s)!=%s", k, v)
	}
}
func TestDel(t *testing.T) {
	myredis := NewRedis(config)
	k := "abc"
	v := "456"
	err := myredis.Set(k, v)
	if err != nil {
		t.Errorf("redis set failure:", err)
	}
	value, err := myredis.Get(k)
	if err != nil {
		t.Errorf("redis get failure:", err)
	}
	if value != v {
		t.Errorf("myredis.Get(%s)!=%s", k, v)
	}
	_, err = myredis.Del(k)
	if err != nil {
		t.Errorf("redis del failure:", err)
	}
	_, err = myredis.Get(k)
	if err == nil {
		t.Errorf("redis get failure:", err)
	}
}
func TestSetExpire(t *testing.T) {
	myredis := NewRedis(config)
	k := "abc"
	v := "456"
	var ti int64
	ti = 3
	err := myredis.Set(k, v)
	if err != nil {
		t.Errorf("redis set failure:", err)
	}
	value, err := myredis.Get(k)
	if err != nil {
		t.Errorf("redis get failure:", err)
	}
	if value != v {
		t.Errorf("myredis.Get(%s)!=%s", k, v)
	}
	myredis.SetExpire(k, ti)
	time.Sleep(4 * time.Second)
	_, err = myredis.Get(k)
	if err == nil {
		t.Errorf("redis get failure:", err)
	}
}
