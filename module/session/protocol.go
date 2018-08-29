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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	libCache "github.com/ltick/tick-framework/module/cache"
	"github.com/ltick/tick-framework/module/config"
	libDatabase "github.com/ltick/tick-framework/module/database"
	libUtility "github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate                       = "session: initiate error"
	errStartup                        = "session: startup error"
	errMissSessionProvider            = "session: miss session provider"
	errInvalidSessionProvider         = "session: invalid session provider"
	errMissSessionRedisDatabase       = "session: miss session redis database"
	errInvalidSessionRedisDatabase    = "session: invalid session redis database"
	errMissSessionKeyPrefix           = "session: miss session Key prefix"
	errInvalidSessionKeyPrefix        = "session: invalid session key prefix"
	errMissSessionCookieId            = "session: miss session Cookie id"
	errInvalidSessionCookieId         = "session: invalid session cookie id"
	errMissSessionMaxAge              = "session: miss session max age"
	errInvalidSessionMaxAge           = "session: invalid session max age"
	errMissRedirectAccessKey          = "session: miss redirect access key"
	errInvalidRedirectAccessKey       = "session: invalid redirect access key"
	errMissRedirectSecretKey          = "session: miss redirect secret key"
	errInvalidRedirectSecretKey       = "session: invalid redirect secret key"
	errMissPermissionProvider         = "session: miss permission provider"
	errInvalidPermissionProvider      = "session: invalid permission provider"
	errMissPermissionMysqlDatabase    = "session: miss permission mysql database"
	errInvalidPermissionMysqlDatabase = "session: invalid permission mysql database"
	errMissCache                      = "session: miss cache"
	errMissDatabase                   = "session: miss database"
)

var debugLog libUtility.LogFunc
var systemLog libUtility.LogFunc

type Instance struct {
	Database *libDatabase.Instance `inject:"true"`
	Cache    *libCache.Instance    `inject:"true"`

	Config      *config.Instance
	Utility     *libUtility.Instance
	DebugLog    libUtility.LogFunc `inject:"true"`
	SystemLog   libUtility.LogFunc `inject:"true"`
	handlerName string
	handler     Handler

	provider         string
	sessionCookieId  string
	sessionMaxAge    int64
	sessionKeyPrefix string

	CookieName              string
	EnableSetCookie         bool
	Gclifetime              int64
	Maxlifetime             int64
	Secure                  bool
	CookieLifeTime          int
	ProviderConfig          string
	Domain                  string
	SessionIDLength         int64
	EnableSidInHttpHeader   bool
	SessionNameInHttpHeader string
	EnableSidInUrlQuery     bool
}

func NewInstance() *Instance {
	return &Instance{}
}

func (this *Instance) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	gob.Register([]interface{}{})
	gob.Register(map[int]interface{}{})
	gob.Register(map[string]interface{}{})
	gob.Register(map[interface{}]interface{}{})
	gob.Register(map[string]string{})
	gob.Register(map[int]string{})
	gob.Register(map[int]int{})
	gob.Register(map[int]int64{})
	var configs map[string]config.Option = map[string]config.Option{
		"MEDIA_SESSION_PROVIDER":         config.Option{Type: config.String, EnvironmentKey: "MEDIA_SESSION_PROVIDER"},
		"MEDIA_SESSION_REDIS_KEY_PREFIX": config.Option{Type: config.String, EnvironmentKey: "MEDIA_SESSION_REDIS_KEY_PREFIX"},
		"MEDIA_SESSION_REDIS_DATABASE":   config.Option{Type: config.Int, EnvironmentKey: "MEDIA_SESSION_REDIS_DATABASE"},
		"MEDIA_SESSION_COOKIE_ID":        config.Option{Type: config.String, EnvironmentKey: "MEDIA_SESSION_COOKIE_ID"},
		"MEDIA_SESSION_MAX_AGE":          config.Option{Type: config.Int64, EnvironmentKey: "MEDIA_SESSION_MAX_AGE"},
		"MEDIA_SESSION_MYSQL_DATABASE":   config.Option{Type: config.Int64, EnvironmentKey: "MEDIA_SESSION_MYSQL_DATABASE"},
	}
	newCtx, err = this.Config.SetOptions(ctx, configs)
	if err != nil {
		return newCtx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	err = Register("mysql", NewMysqlHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	err = Register("redis", NewRedisHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return ctx, nil
}

func (this *Instance) OnStartup(ctx context.Context) (context.Context, error) {
	if this.DebugLog != nil {
		debugLog = this.DebugLog
	} else {
		debugLog = this.Utility.DefaultLogFunc
	}
	if this.SystemLog != nil {
		systemLog = this.SystemLog
	} else {
		systemLog = this.Utility.DefaultLogFunc
	}
	if this.Cache == nil {
		return ctx, errors.New(errMissCache)
	}
	if this.Database == nil {
		return ctx, errors.New(errMissDatabase)
	}
	this.provider = this.Config.GetString("MEDIA_SESSION_PROVIDER")
	this.sessionKeyPrefix = this.Config.GetString("MEDIA_SESSION_REDIS_KEY_PREFIX")
	this.sessionCookieId = this.Config.GetString("MEDIA_SESSION_COOKIE_ID")
	this.sessionMaxAge = this.Config.GetInt64("MEDIA_SESSION_MAX_AGE")
	var err error
	if this.provider != "" {
		err = this.Use(ctx, this.provider)
	} else {
		err = this.Use(ctx, "mysql")
	}
	return ctx, err
}
func (this *Instance) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestStartup(c *routing.Context) error {
	return nil
}
func (this *Instance) OnRequestShutdown(c *routing.Context) error {
	return  nil
}

func (this *Instance) HandlerName() string {
	return this.handlerName
}
func (this *Instance) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	this.handlerName = handlerName
	this.handler = handler()
	switch this.provider {
	case "redis":
		err = this.handler.Initiate(ctx, this.sessionMaxAge, map[string]interface{}{
			"CACHE_INSTANCE":         this.Cache,
			"CACHE_REDIS_DATABASE":   this.Config.GetString("SESSION_REDIS_DATABASE"),
			"CACHE_REDIS_KEY_PREFIX": this.sessionKeyPrefix,
		})
		if err != nil {
			return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
		}
	case "mysql":
		err = this.handler.Initiate(ctx, this.sessionMaxAge, map[string]interface{}{
			"DATABASE_INSTANCE":             this.Database,
			"DATABASE_MYSQL_HOST":           this.Config.GetString("DATABASE_MYSQL_HOST"),
			"DATABASE_MYSQL_PORT":           this.Config.GetString("DATABASE_MYSQL_PORT"),
			"DATABASE_MYSQL_USER":           this.Config.GetString("DATABASE_MYSQL_USER"),
			"DATABASE_MYSQL_PASSWORD":       this.Config.GetString("DATABASE_MYSQL_PASSWORD"),
			"DATABASE_MYSQL_DATABASE":       this.Config.GetString("SESSION_MYSQL_DATABASE"),
			"DATABASE_MYSQL_TIMEOUT":        this.Config.GetInt64("DATABASE_MYSQL_TIMEOUT"),
			"DATABASE_MYSQL_MAX_OPEN_CONNS": this.Config.GetString("DATABASE_MYSQL_MAX_OPEN_CONNS"),
			"DATABASE_MYSQL_MAX_IDLE_CONNS": this.Config.GetString("DATABASE_MYSQL_MAX_IDLE_CONNS"),
		})
		if err != nil {
			return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
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
	Set(key, value interface{}) error     //set session value
	Get(key interface{}) interface{}      //get session value
	Delete(key interface{}) error         //delete session value
	SessionID() string                    //back current sessionID
	SessionRelease(w http.ResponseWriter) // release the resource & save data to provider & return the data
	Flush() error                         //delete all data
}

type Handler interface {
	Initiate(ctx context.Context, maxAge int64, config map[string]interface{}) error
	SessionRead(ctx context.Context, sessionId string) (Store, error)
	SessionExist(ctx context.Context, sessionId string) (bool, error)
	SessionRegenerate(ctx context.Context, oldSessionId, sessionId string) (Store, error)
	SessionDestroy(ctx context.Context, sessionId string) error
	SessionAll(ctx context.Context) (count int, err error)
	SessionGC(ctx context.Context)
}

// getSid retrieves session identifier from HTTP Request.
// First try to retrieve id by reading from cookie, session cookie name is configurable,
// if not exist, then retrieve id from querying parameters.
//
// error is not nil when there is anything wrong.
// sid is empty when need to generate a new session id
// otherwise return an valid session id.
func (this *Instance) getSid(r *http.Request) (string, error) {
	cookie, err := r.Cookie(this.CookieName)
	if err != nil || cookie.Value == "" {
		var sid string
		if this.EnableSidInUrlQuery {
			// err := r.ParseForm()
			// if err != nil {
			// 	return "", err
			// }
			sid = r.FormValue(this.CookieName)
		}

		// if not found in Cookie / param, then read it from request headers
		if this.EnableSidInHttpHeader && sid == "" {
			sids, isFound := r.Header[this.SessionNameInHttpHeader]
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
func (this *Instance) SessionStart(ctx context.Context, w http.ResponseWriter, r *http.Request) (session Store, err error) {
	sid, err := this.getSid(r)
	if err != nil {
		return nil, err
	}
	exist, err := this.handler.SessionExist(ctx, sid)
	if err != nil {
		return nil, err
	}
	if sid != "" && exist {
		return this.handler.SessionRead(ctx, sid)
	}

	// Generate a new session
	sid, err = this.sessionID()
	if err != nil {
		return nil, err
	}

	session, err = this.handler.SessionRead(ctx, sid)
	if err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:     this.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: true,
		Secure:   this.isSecure(r),
		Domain:   this.Domain,
	}
	if this.CookieLifeTime > 0 {
		cookie.MaxAge = this.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(this.CookieLifeTime) * time.Second)
	}
	if this.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if this.EnableSidInHttpHeader {
		r.Header.Set(this.SessionNameInHttpHeader, sid)
		w.Header().Set(this.SessionNameInHttpHeader, sid)
	}

	return
}

// SessionDestroy Destroy session by its id in http request cookie.
func (this *Instance) SessionDestroy(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if this.EnableSidInHttpHeader {
		r.Header.Del(this.SessionNameInHttpHeader)
		w.Header().Del(this.SessionNameInHttpHeader)
	}

	cookie, err := r.Cookie(this.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	this.handler.SessionDestroy(ctx, sid)
	if this.EnableSetCookie {
		expiration := time.Now()
		cookie = &http.Cookie{Name: this.CookieName,
			Path:     "/",
			HttpOnly: true,
			Expires:  expiration,
			MaxAge:   -1}

		http.SetCookie(w, cookie)
	}
}

var errNotExist = errors.New("The session ID does not exist")

// GetSessionStore if session id exists, return SessionStore.
func (this *Instance) GetSessionStore(ctx context.Context, w http.ResponseWriter, r *http.Request) (Store, error) {
	sid, err := this.getSid(r)
	if err != nil {
		return nil, err
	}
	exist, err := this.handler.SessionExist(ctx, sid)
	if err != nil {
		return nil, err
	}
	if sid != "" && exist {
		return this.handler.SessionRead(ctx, sid)
	}
	return nil, errNotExist
}

// GetSessionStore Get SessionStore by its id.
func (this *Instance) GetSessionStoreById(ctx context.Context, sid string) (Store, error) {
	return this.handler.SessionRead(ctx, sid)
}

// GC Start session gc process.
// it can do gc in times after gc lifetime.
func (this *Instance) GC(ctx context.Context) {
	this.handler.SessionGC(ctx)
	time.AfterFunc(time.Duration(this.Gclifetime)*time.Second, func() { this.GC(ctx) })
}

// SessionRegenerateID Regenerate a session id for this SessionStore who's id is saving in http request.
func (this *Instance) SessionRegenerateID(ctx context.Context, w http.ResponseWriter, r *http.Request) (session Store) {
	sid, err := this.sessionID()
	if err != nil {
		return
	}
	cookie, err := r.Cookie(this.CookieName)
	if err != nil || cookie.Value == "" {
		//delete old cookie
		session, _ = this.handler.SessionRead(ctx, sid)
		cookie = &http.Cookie{Name: this.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: true,
			Secure:   this.isSecure(r),
			Domain:   this.Domain,
		}
	} else {
		oldsid, _ := url.QueryUnescape(cookie.Value)
		session, _ = this.handler.SessionRegenerate(ctx, oldsid, sid)
		cookie.Value = url.QueryEscape(sid)
		cookie.HttpOnly = true
		cookie.Path = "/"
	}
	if this.CookieLifeTime > 0 {
		cookie.MaxAge = this.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(this.CookieLifeTime) * time.Second)
	}
	if this.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if this.EnableSidInHttpHeader {
		r.Header.Set(this.SessionNameInHttpHeader, sid)
		w.Header().Set(this.SessionNameInHttpHeader, sid)
	}

	return
}

// GetActiveSession Get all active sessions count number.
func (this *Instance) GetActiveSession(ctx context.Context) (int, error) {
	return this.handler.SessionAll(ctx)
}

// SetSecure Set cookie with https.
func (this *Instance) SetSecure(secure bool) {
	this.Secure = secure
}

func (this *Instance) sessionID() (string, error) {
	b := make([]byte, this.SessionIDLength)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", fmt.Errorf("Could not successfully read from the system CSPRNG.")
	}
	return hex.EncodeToString(b), nil
}

// Set cookie with https.
func (this *Instance) isSecure(req *http.Request) bool {
	if !this.Secure {
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

func (this *Instance) SetCookieSessionId(ctx context.Context, sessionId string, rw http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     this.sessionCookieId,
		Value:    sessionId,
		HttpOnly: false,
		MaxAge:   int(this.sessionMaxAge),
	}
	http.SetCookie(rw, cookie)
}

func SessionNotExists(err error) bool {
	return strings.Contains(err.Error(), errMysqlSessionNotExists) ||
		strings.Contains(err.Error(), errRedisSessionNotExists)
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
