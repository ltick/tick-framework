package session

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ltick/tick-framework/kvstore"
	"time"
)

type RedisStore struct {
	sessionMaxAge          int64
	sessionKvstoreProvider kvstore.KvstoreHandler
	sessionId              string
	lock                   sync.RWMutex
	sessionData            map[interface{}]interface{}
}
type RedisHandler struct {
	Kvstore *kvstore.Kvstore

	sessionKvstoreProvider kvstore.KvstoreHandler
	sessionMaxAge          int64
}

func NewRedisHandler() Handler {
	return &RedisHandler{}
}

var (
	errRedisInitiate          = "session redis: initiate error"
	errRedisSessionExists     = "session redis: session exists error"
	errRedisSessionRead       = "session redis: session read error"
	errRedisSessionRegenerate = "session redis: session regenerate error"
	errRedisSessionDestory    = "session redis: session destory error"
	errRedisSessionAll        = "session redis: session all error"
	errRedisSessionNotExists  = "session redis: session not exists"
)

func (m *RedisHandler) Initiate(ctx context.Context, maxAge int64, config map[string]interface{}) (err error) {
	kvstoreInstance, ok := config["KVSTORE_INSTANCE"]
	if !ok {
		return errors.New(errRedisInitiate + ": empty kvstore instance")
	}
	m.Kvstore, ok = kvstoreInstance.(*kvstore.Kvstore)
	if !ok {
		return errors.New(errRedisInitiate + ": invaild kvstore instance")
	}
	err = m.Kvstore.Use(ctx, "redis")
	if err != nil {
		return errors.New(errStartup + ": " + err.Error())
	}
	m.sessionKvstoreProvider, err = m.Kvstore.NewConnection("redis", map[string]interface{}{
		"KVSTORE_REDIS_DATABASE":   config["KVSTORE_REDIS_DATABASE"],
		"KVSTORE_REDIS_KEY_PREFIX": config["KVSTORE_REDIS_KEY_PREFIX"],
	})
	if err != nil {
		return errors.New(errRedisInitiate + ": " + err.Error())
	}
	m.sessionMaxAge = maxAge
	return nil
}

func (m *RedisHandler) Read(sessionId string) (Store, error) {
	sessionStoreData, err := m.sessionKvstoreProvider.Get(sessionId)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errRedisSessionRead+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	var sessionData map[interface{}]interface{}
	if sessionStoreData == nil {
		sessionData = make(map[interface{}]interface{})
	} else {
		sessionDataByte, ok := sessionStoreData.([]byte)
		if !ok {
			return nil, errors.New(fmt.Sprintf(errRedisSessionRead+": [sessionId:'%s', error:'invalid session data type']", sessionId))
		}
		sessionData, err = DecodeGob(sessionDataByte)
		if err != nil {
			return nil, err
		}
	}
	sessionStore := &RedisStore{
		sessionMaxAge:          m.sessionMaxAge,
		sessionKvstoreProvider: m.sessionKvstoreProvider,
		sessionId:              sessionId,
		sessionData:            sessionData,
	}
	return sessionStore, nil
}
func (m *RedisHandler) Exist(sessionId string) (bool, error) {
	//获取cookie验证是否登录
	_, err := m.sessionKvstoreProvider.Get(sessionId)
	if err != nil {
		if kvstore.ErrNil(err) {
			return false, nil
		}
		return false, errors.New(fmt.Sprintf(errRedisSessionExists+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	return true, nil
}
func (m *RedisHandler) Regenerate(oldSessionId, sessionId string) (Store, error) {
	sessionStoreData, err := m.sessionKvstoreProvider.Get(oldSessionId)
	if err != nil {
		if kvstore.ErrNil(err) {
			err := m.sessionKvstoreProvider.Set(sessionId, "")
			if err != nil {
				return nil, errors.New(fmt.Sprintf(errRedisSessionRegenerate+": %s", err.Error()))
			}
			err = m.sessionKvstoreProvider.Expire(sessionId, m.sessionMaxAge)
			if err != nil {
				return nil, errors.New(fmt.Sprintf(errRedisSessionRegenerate+": %s", err.Error()))
			}
		} else {
			return nil, errors.New(fmt.Sprintf(errRedisSessionRegenerate+": [oldSessionId:'%s', sessionId:'%s', error:'%s']", oldSessionId, sessionId, err.Error()))
		}
	}
	var sessionData map[interface{}]interface{}
	if sessionStoreData == nil {
		sessionData = make(map[interface{}]interface{})
	} else {
		sessionDataByte, ok := sessionStoreData.([]byte)
		if !ok {
			return nil, errors.New(fmt.Sprintf(errRedisSessionRegenerate+": [oldSessionId:'%s', sessionId:'%s', error:'invalid session data type']", oldSessionId, sessionId))
		}
		sessionData, err = DecodeGob(sessionDataByte)
		if err != nil {
			return nil, err
		}
	}
	sessionStore := &RedisStore{
		sessionMaxAge:          m.sessionMaxAge,
		sessionKvstoreProvider: m.sessionKvstoreProvider,
		sessionId:              sessionId,
		sessionData:            sessionData,
	}
	return sessionStore, nil
}
func (m *RedisHandler) Destroy(sessionId string) error {
	_, err := m.sessionKvstoreProvider.Del(sessionId)
	if err != nil {
		return errors.New(fmt.Sprintf(errRedisSessionDestory+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	return nil
}
func (m *RedisHandler) All() (count int, err error) {
	nextCursor, keys, err := m.sessionKvstoreProvider.Scan("", "", 0)
	if err != nil {
		return 0, errors.New(fmt.Sprintf(errRedisSessionAll+": [error:'%s']", err.Error()))
	}
	for len(keys) > 0 {
		count = count + len(keys)
		nextCursor, keys, err = m.sessionKvstoreProvider.Scan(nextCursor, "", 0)
		if err != nil {
			return 0, errors.New(fmt.Sprintf(errRedisSessionAll+": [error:'%s']", err.Error()))
		}
	}
	return count, nil
}

func (m *RedisHandler) GC() {
	return
}

// Session Store Set value in redis session.
// it is temp value in map.
func (m *RedisStore) Set(key interface{}, value interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.sessionData[key] = value
	return nil
}

// Session Store Get value from redis session
func (m *RedisStore) Get(key interface{}) interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if v, ok := m.sessionData[key]; ok {
		return v
	}
	return nil
}

// Session Store Delete value in redis session
func (m *RedisStore) Delete(key interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.sessionData, key)
	return nil
}

// Session Store Flush clear all sessionData in redis session
func (m *RedisStore) Flush() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.sessionData = make(map[interface{}]interface{})
	return nil
}

// Session Store ID get session id of this redis session store
func (m *RedisStore) ID() string {
	return m.sessionId
}

// Session Store Release save mysql session sessionData to database.
// must call this method to save sessionData to cache.
func (m *RedisStore) Release() {
	b, err := EncodeGob(m.sessionData)
	if err != nil {
		return
	}
	err = m.sessionKvstoreProvider.Set(m.sessionId, string(b))
	if err != nil {
		return
	}
	err = m.sessionKvstoreProvider.Expire(m.sessionId, m.sessionMaxAge)
	if err != nil {
		return
	}
}