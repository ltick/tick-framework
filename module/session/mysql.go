package session

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	libDatabase "github.com/ltick/tick-framework/module/database"
)

type MysqlStore struct {
	sessionMaxAge           int64
	sessionDatabaseProvider libDatabase.DatabaseHandler
	sessionId               string
	lock                    sync.RWMutex
	sessionData             map[interface{}]interface{}
}
type MysqlStoreData struct {
	ID          string     `gorm:"type:char(36);not null;primary_key";`                                                                    //文件唯一ID
	SessionData string     `gorm:"column:session_data;type:text;size:1024;not null;default:'';unique_index:uniq_resource_ucid_expiredat";` //资源名称
	SessionId   string     `gorm:"column:session_id;type:char(50);not null;unique_index:uniq_resource_ucid_expiredat";`                    //账户ID
	CreatedAt   time.Time  `gorm:"type:datetime;not null;default:'0000-00-00 00:00:00'";`                                                  //创建时间，gorm自带
	UpdatedAt   time.Time  `gorm:"type:datetime;not null;default:'0000-00-00 00:00:00'";`                                                  //更新时间，gorm自带
	ExpiredAt   *time.Time `gorm:"type:datetime;not null;default:'0000-00-00 00:00:00'";`                                                  //过期时间
}

type MysqlHandler struct {
	Database *libDatabase.Instance

	sessionDatabaseProvider libDatabase.DatabaseHandler
	sessionMaxAge           int64
	sessionId               string
}

func NewMysqlHandler() Handler {
	return &MysqlHandler{}
}

var (
	errMysqlInitiate          = "session mysql: initiate error"
	errMysqlSessionExists     = "session mysql: session exists error"
	errMysqlSessionRead       = "session mysql: session read error"
	errMysqlSessionRegenerate = "session mysql: session regenerate error"
	errMysqlSessionDestory    = "session mysql: session destory error"
	errMysqlSessionAll        = "session mysql: session all error"
	errMysqlSessionNotExists  = "session mysql: session not exists"
)

func (m *MysqlHandler) Initiate(ctx context.Context, maxAge int64, config map[string]interface{}) (err error) {
	databaseInstance, ok := config["DATABASE_INSTANCE"]
	if !ok {
		return errors.New(errMysqlInitiate + ": empty database instance")
	}
	m.Database, ok = databaseInstance.(*libDatabase.Instance)
	if !ok {
		return errors.New(errMysqlInitiate + ": invaild database instance")
	}
	err = m.Database.Use(ctx, "mysql")
	if err != nil {
		return errors.New(errMysqlInitiate + ": " + err.Error())
	}
	m.sessionDatabaseProvider, err = m.Database.NewDatabase(ctx, "session", map[string]interface{}{
		"DATABASE_MYSQL_HOST":           config["DATABASE_MYSQL_HOST"],
		"DATABASE_MYSQL_PORT":           config["DATABASE_MYSQL_PORT"],
		"DATABASE_MYSQL_USER":           config["DATABASE_MYSQL_USER"],
		"DATABASE_MYSQL_PASSWORD":       config["DATABASE_MYSQL_PASSWORD"],
		"DATABASE_MYSQL_DATABASE":       config["DATABASE_MYSQL_DATABASE"],
		"DATABASE_MYSQL_TIMEOUT":        config["DATABASE_MYSQL_TIMEOUT"],
		"DATABASE_MYSQL_MAX_OPEN_CONNS": config["DATABASE_MYSQL_MAX_OPEN_CONNS"],
		"DATABASE_MYSQL_MAX_IDLE_CONNS": config["DATABASE_MYSQL_MAX_IDLE_CONNS"],
	})
	if err != nil {
		return errors.New(errMysqlInitiate + ": " + err.Error())
	}
	if _, ok := config["max_age"]; ok {
		m.sessionMaxAge, ok = config["max_age"].(int64)
		if !ok {
			return errors.New(errMysqlInitiate + ": invaild max age data type")
		}
	}
	return nil
}
func (m *MysqlHandler) SessionRead(ctx context.Context, sessionId string) (Store, error) {
	//获取cookie验证是否登录
	sessionStoreData := &MysqlStoreData{}
	selectModel := m.sessionDatabaseProvider.New().Model(&MysqlStoreData{})
	err := selectModel.Unscoped().Where(&MysqlStoreData{SessionId: sessionId}).Find(&sessionStoreData).Error()
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errMysqlSessionRead+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	var sessionData map[interface{}]interface{}
	if sessionStoreData == nil {
		sessionData = make(map[interface{}]interface{})
	} else {
		sessionData, err = DecodeGob([]byte(sessionStoreData.SessionData))
		if err != nil {
			return nil, err
		}
	}
	sessionStore := &MysqlStore{
		sessionDatabaseProvider: m.sessionDatabaseProvider,
		sessionId:               sessionId,
		sessionData:             sessionData,
	}
	return sessionStore, nil
}
func (m *MysqlHandler) SessionExist(ctx context.Context, sessionId string) (bool, error) {
	//获取cookie验证是否登录
	sessionStoreData := &MysqlStoreData{}
	selectModel := m.sessionDatabaseProvider.New().Model(&MysqlStoreData{})
	err := selectModel.Unscoped().Where(&MysqlStoreData{SessionId: sessionId}).Find(&sessionStoreData).Error()
	if err != nil {
		return false, errors.New(fmt.Sprintf(errMysqlSessionExists+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	if sessionStoreData != nil {
		return false, errors.New(errMysqlSessionNotExists)
	}
	return true, nil
}
func (m *MysqlHandler) SessionRegenerate(ctx context.Context, oldSessionId, sessionId string) (Store, error) {
	sessionStoreData := &MysqlStoreData{}
	selectModel := m.sessionDatabaseProvider.New().Model(&MysqlStoreData{})
	err := selectModel.Where(&MysqlStoreData{SessionId: oldSessionId}).Find(&sessionStoreData).Error()
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errMysqlSessionRegenerate+": [oldSessionId:'%s', sessionId:'%s', error:'%s']", oldSessionId, sessionId, err.Error()))
	}
	if m.sessionDatabaseProvider.RecordNotFound() {
		insertModel := m.sessionDatabaseProvider.New().Model(&MysqlStoreData{})
		expiredAt := time.Now().Add(time.Duration(m.sessionMaxAge) * time.Second)
		sessionStoreData.ExpiredAt = &expiredAt
		sessionStoreData.SessionId = sessionId
		err = insertModel.Save(sessionStoreData).Error()
		debugLog(ctx, "session: session regenerate [oldSessionId:'%s', sessionId:'%s', error:'%s']", oldSessionId, sessionId, err.Error())
		if err != nil {
			return nil, errors.New(fmt.Sprintf(errMysqlSessionRegenerate+": [oldSessionId:'%s', sessionId:'%s', error:'%s']", oldSessionId, sessionId, err.Error()))
		}
	}
	var sessionData map[interface{}]interface{}
	if sessionStoreData == nil {
		sessionData = make(map[interface{}]interface{})
	} else {
		sessionData, err = DecodeGob([]byte(sessionStoreData.SessionData))
		if err != nil {
			return nil, err
		}
	}
	sessionStore := &MysqlStore{
		sessionDatabaseProvider: m.sessionDatabaseProvider,
		sessionId:               sessionId,
		sessionData:             sessionData,
	}
	return sessionStore, nil
}
func (m *MysqlHandler) SessionDestroy(ctx context.Context, sessionId string) error {
	deleteModel := m.sessionDatabaseProvider.New().Model(&MysqlStoreData{})
	err := deleteModel.Unscoped().Delete(&MysqlStoreData{
		SessionId: sessionId,
	}).Error()
	if err != nil {
		return errors.New(fmt.Sprintf(errMysqlSessionDestory+": [sessionId:'%s', error:'%s']", sessionId, err.Error()))
	}
	return nil
}
func (m *MysqlHandler) SessionAll(ctx context.Context) (count int, err error) {
	selectModel := m.sessionDatabaseProvider.New().Model(&MysqlStoreData{})
	err = selectModel.Unscoped().Count(&count).Error()
	if err != nil {
		return 0, errors.New(fmt.Sprintf(errMysqlSessionAll+": [error:'%s']", err.Error()))
	}
	return count, nil
}
func (m *MysqlHandler) SessionGC(ctx context.Context) {
	m.sessionDatabaseProvider.New().Model(&MysqlStoreData{}).Unscoped().Where("expired_at < ?", time.Now().Unix()).Delete(&MysqlStoreData{})
}

// Set value in mysql session.
// it is temp value in map.
func (m *MysqlStore) Set(key interface{}, value interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.sessionData[key] = value
	return nil
}

// Get value from mysql session
func (m *MysqlStore) Get(key interface{}) interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if v, ok := m.sessionData[key]; ok {
		return v
	}
	return nil
}

// Delete value in mysql session
func (m *MysqlStore) Delete(key interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.sessionData, key)
	return nil
}

// Flush clear all sessionData in mysql session
func (m *MysqlStore) Flush() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.sessionData = make(map[interface{}]interface{})
	return nil
}

// SessionID get session id of this mysql session store
func (m *MysqlStore) SessionID() string {
	return m.sessionId
}

// SessionRelease save mysql session sessionData to database.
// must call this method to save sessionData to database.
func (m *MysqlStore) SessionRelease(w http.ResponseWriter) {
	b, err := EncodeGob(m.sessionData)
	if err != nil {
		return
	}
	now := time.Now().Local().Add(time.Second*time.Duration(m.sessionMaxAge))
	m.sessionDatabaseProvider.New().Model(&MysqlStoreData{}).Update(&MysqlStoreData{
		SessionData: string(b),
		SessionId:   m.sessionId,
		ExpiredAt:   &now,
	})
}

func (m *MysqlHandler) AutoMigrate() {
	if m.sessionDatabaseProvider != nil {
		m.sessionDatabaseProvider.Debug()
		m.sessionDatabaseProvider.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci comment '会话'").AutoMigrate(&MysqlStore{})
	}
}
func (m *MysqlHandler) Recreate() {
	if m.sessionDatabaseProvider != nil {
		m.sessionDatabaseProvider.Debug()
		if m.sessionDatabaseProvider.HasTable(&MysqlStore{}) {
			m.sessionDatabaseProvider.DropTable(&MysqlStore{})
		}
		m.sessionDatabaseProvider.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci comment '会话'").AutoMigrate(&MysqlStore{})
	}
}
