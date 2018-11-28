package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/ltick/tick-framework/kvstore"
)

type RedisStore struct {
	sessionMaxAge        int64
	sessionCacheProvider kvstore.KvstoreHandler
	sessionId            string
	lock                 sync.RWMutex
	sessionData          map[interface{}]interface{}
}
type RedisHandler struct {
	Cache *kvstore.Kvstore

	sessionCacheProvider kvstore.KvstoreHandler
	sessionMaxAge        int64
	sessionId            string
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
	m.Cache, ok = kvstoreInstance.(*kvstore.Kvstore)
	if !ok {
		return errors.New(errRedisInitiate + ": invaild kvstore instance")
	}
	err = m.Cache.Use(ctx, "redis")
	if err != nil {
		return errors.New(errStartup + ": " + err.Error())
	}
	m.sessionCacheProvider, err = m.Cache.NewConnection("redis", map[string]interface{}{
		"KVSTORE_REDIS_DATABASE":   config["KVSTORE_REDIS_DATABASE"],
		"KVSTORE_REDIS_KEY_PREFIX": config["KVSTORE_REDIS_KEY_PREFIX"],
	})
	if err != nil {
		return errors.New(errRedisInitiate + ": " + err.Error())
	}
	m.sessionMaxAge = maxAge
	return nil
}

func (m *RedisHandler) Read(ctx context.Context, sessionId string) (Store, error) {
	sessionStoreData, err := m.sessionCacheProvider.Get(sessionId)
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
		sessionMaxAge:        m.sessionMaxAge,
		sessionCacheProvider: m.sessionCacheProvider,
		sessionId:            sessionId,
		sessionData:          sessionData,
	}
	return sessionStore, nil
}
func (m *RedisHandler) Exist(ctx context.Context, sessionId string) (bool, error) {
	//获取cookie验证是否登录
	_, err := m.sessionCacheProvider.Get(sessionId)
	if err != nil {
		if kvstore.ErrNil(err) {
			return false, nil
		}
		return false, errors.New(fmt.Sprintf(errRedisSessionExists+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	return true, nil
}
func (m *RedisHandler) Regenerate(ctx context.Context, oldSessionId, sessionId string) (Store, error) {
	sessionStoreData, err := m.sessionCacheProvider.Get(oldSessionId)
	if err != nil {
		if kvstore.ErrNil(err) {
			err := m.sessionCacheProvider.Set(sessionId, "")
			debugLog(ctx, "receive: set session [sessionId:'%s', error:'%v']", sessionId, err)
			if err != nil {
				return nil, errors.New(fmt.Sprintf(errRedisSessionRegenerate+": %s", err.Error()))
			}
			err = m.sessionCacheProvider.Expire(sessionId, m.sessionMaxAge)
			debugLog(ctx, "receive: set session expire [sessionId:'%s', error:'%v']", sessionId, err)
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
		sessionMaxAge:        m.sessionMaxAge,
		sessionCacheProvider: m.sessionCacheProvider,
		sessionId:            sessionId,
		sessionData:          sessionData,
	}
	return sessionStore, nil
}
func (m *RedisHandler) Destroy(ctx context.Context, sessionId string) error {
	_, err := m.sessionCacheProvider.Del(sessionId)
	if err != nil {
		return errors.New(fmt.Sprintf(errRedisSessionDestory+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	return nil
}
func (m *RedisHandler) All(ctx context.Context) (count int, err error) {
	nextCursor, keys, err := m.sessionCacheProvider.Scan("", "", 0)
	if err != nil {
		return 0, errors.New(fmt.Sprintf(errRedisSessionAll+": [error:'%s']", err.Error()))
	}
	for len(keys) > 0 {
		count = count + len(keys)
		nextCursor, keys, err = m.sessionCacheProvider.Scan(nextCursor, "", 0)
		if err != nil {
			return 0, errors.New(fmt.Sprintf(errRedisSessionAll+": [error:'%s']", err.Error()))
		}
	}
	return count, nil
}
func (m *RedisHandler) GC(ctx context.Context) {
	return
}

// Set value in redis session.
// it is temp value in map.
func (m *RedisStore) Set(key interface{}, value interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.sessionData[key] = value
	return nil
}

// Get value from redis session
func (m *RedisStore) Get(key interface{}) interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if v, ok := m.sessionData[key]; ok {
		return v
	}
	return nil
}

// Delete value in redis session
func (m *RedisStore) Delete(key interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.sessionData, key)
	return nil
}

// Flush clear all sessionData in redis session
func (m *RedisStore) Flush() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.sessionData = make(map[interface{}]interface{})
	return nil
}

// SessionID get session id of this redis session store
func (m *RedisStore) ID() string {
	return m.sessionId
}

// SessionRelease save redis session sessionData to kvstore.
// must call this method to save sessionData to kvstore.
func (m *RedisStore) Release(w http.ResponseWriter) {
	b, err := EncodeGob(m.sessionData)
	if err != nil {
		return
	}
	err = m.sessionCacheProvider.Set(m.sessionId, string(b))
	if err != nil {
		return
	}
	err = m.sessionCacheProvider.Expire(m.sessionId, m.sessionMaxAge)
	if err != nil {
		return
	}
}
