package session

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	libDatabase "github.com/ltick/tick-framework/database"
	"github.com/ltick/tick-framework/kvstore"
	libUtility "github.com/ltick/tick-framework/utility"
)

var (
	errPrepare              = "session: prepare '%s' error"
	errInitiate             = "session: initiate '%s' error"
	errStartup              = "session: startup '%s' error"
	errUseProvider          = "session: use '%s' provider error"
	errMissProvider         = "session: miss provider"
	errMissCookieName       = "session: miss Cookie Name"
	errInvalidProvider      = "session: invalid provider"
	errMissRedisDatabase    = "session: miss redis database"
	errInvalidRedisDatabase = "session: invalid redis database"
	errMissRedisKeyPrefix   = "session: miss redis key prefix"
	errMissMysqlDatabase    = "session: miss mysql database"
	errInvalidKeyPrefix     = "session: invalid session key prefix"
	errMissMaxAge           = "session: miss session max age"
	errInvalidMaxAge        = "session: invalid session max age"
	errMissKvstore          = "session: miss cache"
	errMissDatabase         = "session: miss database"
	errSessionNotExist      = "session: session does not exist"
	errSessionStart         = "session: session start"
	errSessionGetStore      = "session: session get store"
)

var debugLog libUtility.LogFunc
var systemLog libUtility.LogFunc

type Session struct {
	Database *libDatabase.Database `inject:"true"`
	Kvstore  *kvstore.Kvstore      `inject:"true"`

	Config    *config.Config     `inject:"true"`
	DebugLog  libUtility.LogFunc `inject:"true"`
	SystemLog libUtility.LogFunc `inject:"true"`
	handler   Handler

	provider                string
	CookieName              string
	MaxAge                  int64
	EnableSetCookie         bool
	Gclifetime              int64
	Maxlifetime             int64
	Secure                  bool
	CookieLifeTime          int
	providerConfig          string
	Domain                  string
	SessionIDLength         int64
	EnableSidInHttpHeader   bool
	SessionNameInHttpHeader string
	EnableSidInUrlQuery     bool
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Prepare(ctx context.Context) (context.Context, error) {
	var configs map[string]config.Option = map[string]config.Option{
		"SESSION_PROVIDER":         config.Option{Type: config.String, EnvironmentKey: "SESSION_PROVIDER"},
		"SESSION_REDIS_KEY_PREFIX": config.Option{Type: config.String, EnvironmentKey: "SESSION_REDIS_KEY_PREFIX"},
		"SESSION_REDIS_DATABASE":   config.Option{Type: config.Int, EnvironmentKey: "SESSION_REDIS_DATABASE"},
		"SESSION_COOKIE_NAME":      config.Option{Type: config.String, EnvironmentKey: "SESSION_COOKIE_NAME"},
		"SESSION_MAX_AGE":          config.Option{Type: config.Int64, EnvironmentKey: "SESSION_MAX_AGE"},
		"SESSION_MYSQL_DATABASE":   config.Option{Type: config.Int64, EnvironmentKey: "SESSION_MYSQL_DATABASE"},
	}
	err := s.Config.SetOptions(configs)
	if err != nil {
		return ctx, fmt.Errorf(errPrepare+": %s", err.Error())
	}
	return ctx, nil
}

func (s *Session) Initiate(ctx context.Context) (context.Context, error) {
	gob.Register([]interface{}{})
	gob.Register(map[int]interface{}{})
	gob.Register(map[string]interface{}{})
	gob.Register(map[interface{}]interface{}{})
	gob.Register(map[string]string{})
	gob.Register(map[int]string{})
	gob.Register(map[int]int{})
	gob.Register(map[int]int64{})
	err := Register("mysql", NewMysqlHandler)
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, s.provider))
	}
	err = Register("redis", NewRedisHandler)
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, s.provider))
	}
	return ctx, nil
}

func (s *Session) OnStartup(ctx context.Context) (context.Context, error) {
	if s.Kvstore == nil {
		return ctx, errors.Annotate(errors.New(errMissKvstore), fmt.Sprintf(errStartup, s.provider))
	}
	if s.Database == nil {
		return ctx, errors.Annotate(errors.New(errMissDatabase), fmt.Sprintf(errStartup, s.provider))
	}
	s.provider = s.Config.GetString("SESSION_PROVIDER")
	if s.provider == "" {
		return ctx, errors.Annotate(errors.New(errMissProvider), fmt.Sprintf(errStartup, s.provider))
	}
	s.CookieName = s.Config.GetString("SESSION_COOKIE_NAME")
	if s.CookieName == "" {
		return ctx, errors.Annotate(errors.New(errMissCookieName), fmt.Sprintf(errStartup, s.provider))
	}
	s.MaxAge = s.Config.GetInt64("SESSION_MAX_AGE")
	if s.MaxAge == 0 {
		return ctx, errors.Annotate(errors.New(errMissMaxAge), fmt.Sprintf(errStartup, s.provider))
	}
	var err error
	if s.provider != "" {
		err = s.Use(ctx, s.provider)
	} else {
		err = s.Use(ctx, "mysql")
	}
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errStartup, s.provider))
	}
	return ctx, nil
}
func (s *Session) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (s *Session) Getprovider() string {
	return s.provider
}
func (s *Session) Use(ctx context.Context, provider string) error {
	handler, err := Use(provider)
	if err != nil {
		return err
	}
	s.provider = provider
	s.handler = handler()
	switch s.provider {
	case "redis":
		redisDatabase := s.Config.GetString("SESSION_REDIS_DATABASE")
		if redisDatabase == "" {
			return errors.Annotate(errors.New(errMissRedisDatabase), fmt.Sprintf(errUseProvider, s.provider))
		}
		redisKeyPrefix := s.Config.GetString("SESSION_REDIS_KEY_PREFIX")
		if redisKeyPrefix == "" {
			return errors.New(errMissRedisKeyPrefix)
		}
		err = s.handler.Initiate(ctx, s.MaxAge, map[string]interface{}{
			"KVSTORE_INSTANCE":         s.Kvstore,
			"KVSTORE_REDIS_DATABASE":   redisDatabase,
			"KVSTORE_REDIS_KEY_PREFIX": redisKeyPrefix,
		})
		if err != nil {
			return errors.Annotate(err, fmt.Sprintf(errUseProvider, s.provider))
		}
	case "mysql":
		mysqlDatabase := s.Config.GetString("SESSION_MYSQL_DATABASE")
		if mysqlDatabase == "" {
			return errors.Annotate(errors.New(errMissMysqlDatabase), fmt.Sprintf(errUseProvider, s.provider))
		}
		err = s.handler.Initiate(ctx, s.MaxAge, map[string]interface{}{
			"DATABASE_INSTANCE":             s.Database,
			"DATABASE_MYSQL_HOST":           s.Config.GetString("DATABASE_MYSQL_HOST"),
			"DATABASE_MYSQL_PORT":           s.Config.GetString("DATABASE_MYSQL_PORT"),
			"DATABASE_MYSQL_USER":           s.Config.GetString("DATABASE_MYSQL_USER"),
			"DATABASE_MYSQL_PASSWORD":       s.Config.GetString("DATABASE_MYSQL_PASSWORD"),
			"DATABASE_MYSQL_DATABASE":       mysqlDatabase,
			"DATABASE_MYSQL_TIMEOUT":        s.Config.GetInt64("DATABASE_MYSQL_TIMEOUT"),
			"DATABASE_MYSQL_MAX_OPEN_CONNS": s.Config.GetString("DATABASE_MYSQL_MAX_OPEN_CONNS"),
			"DATABASE_MYSQL_MAX_IDLE_CONNS": s.Config.GetString("DATABASE_MYSQL_MAX_IDLE_CONNS"),
		})
		if err != nil {
			return errors.Annotate(err, fmt.Sprintf(errUseProvider, s.provider))
		}
	}

	return nil
}

type sessionHandler func() Handler

var sessionHandlers = make(map[string]sessionHandler)

func Register(name string, sessionHandler sessionHandler) error {
	if sessionHandler == nil {
		return errors.New("session: Register session is nil")
	}
	if _, ok := sessionHandlers[name]; !ok {
		sessionHandlers[name] = sessionHandler
	}
	return nil
}
func Use(name string) (sessionHandler, error) {
	if _, exist := sessionHandlers[name]; !exist {
		return nil, errors.New("session: unknown session " + name + " (forgotten register?)")
	}
	return sessionHandlers[name], nil
}

// Store contains all data for one session process with specific id.
type Store interface {
	Set(key, value interface{}) error //set session value
	Get(key interface{}) interface{}  //get session value
	Delete(key interface{}) error     //delete session value
	ID() string                       //back current sessionID
	Flush() error                     //delete all data
	Release() error                   //release all data
}

type Handler interface {
	Initiate(ctx context.Context, maxAge int64, config map[string]interface{}) error
	Read(sessionId string) (Store, error)
	Exist(sessionId string) (bool, error)
	Regenerate(oldId, sessionId string) (Store, error)
	Destroy(sessionId string) error
	All() (count int, err error)
	GC()
}

// getSid retrieves session identifier from HTTP Request.
// First try to retrieve id by reading from cookie, session cookie name is configurable,
// if not exist, then retrieve id from querying parameters.
//
// error is not nil when there is anything wrong.
// sid is empty when need to generate a new session id
// otherwise return an valid session id.
func (s *Session) getSid(r *http.Request) (string, error) {
	cookie, err := r.Cookie(s.CookieName)
	if err != nil || cookie.Value == "" {
		var sid string
		if s.EnableSidInUrlQuery {
			// err := r.ParseForm()
			// if err != nil {
			// 	return "", err
			// }
			sid = r.FormValue(s.CookieName)
		}

		// if not found in Cookie / param, then read it from request headers
		if s.EnableSidInHttpHeader && sid == "" {
			sids, isFound := r.Header[s.SessionNameInHttpHeader]
			if isFound && len(sids) != 0 {
				return sids[0], nil
			}
		}

		return sid, nil
	}

	// HTTP Request contains cookie for sessionid info.
	return url.QueryUnescape(cookie.Value)
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (s *Session) Start(w http.ResponseWriter, r *http.Request) (session Store, err error) {
	sid, err := s.getSid(r)
	if err != nil {
		return nil, errors.Annotate(err, errSessionStart)
	}
	exist, err := s.handler.Exist(sid)
	if err != nil {
		return nil, errors.Annotate(err, errSessionStart)
	}
	if sid != "" && exist {
		return s.handler.Read(sid)
	}

	// Generate a new session
	sid, err = s.sessionID()
	if err != nil {
		return nil, errors.Annotate(err, errSessionStart)
	}

	session, err = s.handler.Read(sid)
	if err != nil {
		return nil, errors.Annotate(err, errSessionStart)
	}
	cookie := &http.Cookie{
		Name:     s.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.isSecure(r),
		Domain:   s.Domain,
	}
	if s.CookieLifeTime > 0 {
		cookie.MaxAge = s.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(s.CookieLifeTime) * time.Second)
	}
	if s.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if s.EnableSidInHttpHeader {
		r.Header.Set(s.SessionNameInHttpHeader, sid)
		w.Header().Set(s.SessionNameInHttpHeader, sid)
	}

	return session, nil
}

// Destroy Destroy session by its id in http request cookie.
func (s *Session) Destroy(w http.ResponseWriter, r *http.Request) {
	if s.EnableSidInHttpHeader {
		r.Header.Del(s.SessionNameInHttpHeader)
		w.Header().Del(s.SessionNameInHttpHeader)
	}

	cookie, err := r.Cookie(s.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	s.handler.Destroy(sid)
	if s.EnableSetCookie {
		expiration := time.Now()
		cookie = &http.Cookie{
			Name:     s.CookieName,
			Path:     "/",
			HttpOnly: true,
			Expires:  expiration,
			MaxAge:   -1,
		}

		http.SetCookie(w, cookie)
	}
}

// GetSessionStore if session id exists, return SessionStore.
func (s *Session) GetStore(w http.ResponseWriter, r *http.Request) (Store, error) {
	sid, err := s.getSid(r)
	if err != nil {
		return nil, err
	}
	exist, err := s.handler.Exist(sid)
	if err != nil {
		return nil, errors.Annotate(err, errSessionGetStore)
	}
	if sid != "" && exist {
		return s.handler.Read(sid)
	}
	return nil, errors.Annotate(errors.New(errSessionNotExist), errSessionGetStore)
}

// GetSessionStore Get SessionStore by its id.
func (s *Session) GetStoreById(sid string) (Store, error) {
	return s.handler.Read(sid)
}

// GC Start session gc process.
// it can do gc in times after gc lifetime.
func (s *Session) GC() {
	s.handler.All()
	time.AfterFunc(time.Duration(s.Gclifetime)*time.Second, func() { s.GC() })
}

// RegenerateID Regenerate a session id for this SessionStore who's id is saving in http request.
func (s *Session) RegenerateID(ctx context.Context, w http.ResponseWriter, r *http.Request) (session Store) {
	sid, err := s.sessionID()
	if err != nil {
		return
	}
	cookie, err := r.Cookie(s.CookieName)
	if err != nil || cookie.Value == "" {
		//delete old cookie
		session, _ = s.handler.Read(sid)
		cookie = &http.Cookie{Name: s.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: true,
			Secure:   s.isSecure(r),
			Domain:   s.Domain,
		}
	} else {
		oldsid, _ := url.QueryUnescape(cookie.Value)
		session, _ = s.handler.Regenerate(oldsid, sid)
		cookie.Value = url.QueryEscape(sid)
		cookie.HttpOnly = true
		cookie.Path = "/"
	}
	if s.CookieLifeTime > 0 {
		cookie.MaxAge = s.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(s.CookieLifeTime) * time.Second)
	}
	if s.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if s.EnableSidInHttpHeader {
		r.Header.Set(s.SessionNameInHttpHeader, sid)
		w.Header().Set(s.SessionNameInHttpHeader, sid)
	}

	return
}

// GetActiveSession Get all active sessions count number.
func (s *Session) GetActiveSession() (int, error) {
	return s.handler.All()
}

// SetSecure Set cookie with https.
func (s *Session) SetSecure(secure bool) {
	s.Secure = secure
}

func (s *Session) sessionID() (string, error) {
	b := make([]byte, s.SessionIDLength)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", fmt.Errorf("Could not successfully read from the system CSPRNG.")
	}
	return hex.EncodeToString(b), nil
}

// Set cookie with https.
func (s *Session) isSecure(req *http.Request) bool {
	if !s.Secure {
		return false
	}
	if req.URL.Scheme != "" {
		return req.URL.Scheme == "https"
	}
	if req.TLS == nil {
		return false
	}
	return true
}

// EncodeGob encode the obj to gob
func EncodeGob(obj map[interface{}]interface{}) ([]byte, error) {
	for _, v := range obj {
		gob.Register(v)
	}
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(obj)
	if err != nil {
		return []byte(""), err
	}
	return buf.Bytes(), nil
}

// DecodeGob decode data to map
func DecodeGob(encoded []byte) (map[interface{}]interface{}, error) {
	buf := bytes.NewBuffer(encoded)
	dec := gob.NewDecoder(buf)
	var out map[interface{}]interface{}
	err := dec.Decode(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Encryption -----------------------------------------------------------------

// encrypt encrypts a value using the given block in counter mode.
//
// A random initialization vector (http://goo.gl/zF67k) with the length of the
// block size is prepended to the resulting ciphertext.
func encrypt(block cipher.Block, value []byte) ([]byte, error) {
	iv := libUtility.RandomBytes(block.BlockSize())
	if iv == nil {
		return nil, errors.New("encrypt: failed to generate random iv")
	}
	// Encrypt it.
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(value, value)
	// Return iv + ciphertext.
	return append(iv, value...), nil
}

// decrypt decrypts a value using the given block in counter mode.
//
// The value to be decrypted must be prepended by a initialization vector
// (http://goo.gl/zF67k) with the length of the block size.
func decrypt(block cipher.Block, value []byte) ([]byte, error) {
	size := block.BlockSize()
	if len(value) > size {
		// Extract iv.
		iv := value[:size]
		// Extract ciphertext.
		value = value[size:]
		// Decrypt it.
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(value, value)
		return value, nil
	}
	return nil, errors.New("decrypt: the value could not be decrypted")
}

func encodeCookie(block cipher.Block, hashKey, name string, value map[interface{}]interface{}) (string, error) {
	var err error
	var b []byte
	// 1. EncodeGob.
	if b, err = EncodeGob(value); err != nil {
		return "", err
	}
	// 2. Encrypt (optional).
	if b, err = encrypt(block, b); err != nil {
		return "", err
	}
	b = encode(b)
	// 3. Create MAC for "name|date|value". Extra pipe to be used later.
	b = []byte(fmt.Sprintf("%s|%d|%s|", name, time.Now().UTC().Unix(), b))
	h := hmac.New(sha1.New, []byte(hashKey))
	h.Write(b)
	sig := h.Sum(nil)
	// Append mac, remove name.
	b = append(b, sig...)[len(name)+1:]
	// 4. Encode to base64.
	b = encode(b)
	// Done.
	return libUtility.BytesToString(b), nil
}

func decodeCookie(block cipher.Block, hashKey, name, value string, gcmaxlifetime int64) (map[interface{}]interface{}, error) {
	// 1. Decode from base64.
	b, err := decode([]byte(value))
	if err != nil {
		return nil, err
	}
	// 2. Verify MAC. Value is "date|value|mac".
	parts := bytes.SplitN(b, []byte("|"), 3)
	if len(parts) != 3 {
		return nil, errors.New("Decode: invalid value %v")
	}

	b = append([]byte(name+"|"), b[:len(b)-len(parts[2])]...)
	h := hmac.New(sha1.New, []byte(hashKey))
	h.Write(b)
	sig := h.Sum(nil)
	if len(sig) != len(parts[2]) || subtle.ConstantTimeCompare(sig, parts[2]) != 1 {
		return nil, errors.New("Decode: the value is not valid")
	}
	// 3. Verify date ranges.
	var t1 int64
	if t1, err = strconv.ParseInt(libUtility.BytesToString(parts[0]), 10, 64); err != nil {
		return nil, errors.New("Decode: invalid timestamp")
	}
	t2 := time.Now().UTC().Unix()
	if t1 > t2 {
		return nil, errors.New("Decode: timestamp is too new")
	}
	if t1 < t2-gcmaxlifetime {
		return nil, errors.New("Decode: expired timestamp")
	}
	// 4. Decrypt (optional).
	b, err = decode(parts[1])
	if err != nil {
		return nil, err
	}
	if b, err = decrypt(block, b); err != nil {
		return nil, err
	}
	// 5. DecodeGob.
	dst, err := DecodeGob(b)
	if err != nil {
		return nil, err
	}
	return dst, nil
}

// Encoding -------------------------------------------------------------------

// encode encodes a value using base64.
func encode(value []byte) []byte {
	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(value)))
	base64.URLEncoding.Encode(encoded, value)
	return encoded
}

// decode decodes a cookie using base64.
func decode(value []byte) ([]byte, error) {
	decoded := make([]byte, base64.URLEncoding.DecodedLen(len(value)))
	b, err := base64.URLEncoding.Decode(decoded, value)
	if err != nil {
		return nil, err
	}
	return decoded[:b], nil
}

func NotExists(err error) bool {
	return strings.Contains(err.Error(), errMysqlSessionNotExists) ||
		strings.Contains(err.Error(), errRedisSessionNotExists) ||
		strings.Contains(err.Error(), errSessionNotExist)
}
