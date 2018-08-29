package ltick

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/ltick/tick-framework/module/session"
	libSession "github.com/ltick/tick-framework/module/session"
	libUtility "github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-routing"
)

var errNotEnableSession = errors.New("before using the session, must set config `session::enable = true`...")

// Headers
const (
	HeaderAccept                        = "Accept"
	HeaderAcceptEncoding                = "Accept-Encoding"
	HeaderAuthorization                 = "Authorization"
	HeaderContentDisposition            = "Content-Disposition"
	HeaderContentEncoding               = "Content-Encoding"
	HeaderContentLength                 = "Content-Length"
	HeaderContentType                   = "Content-Type"
	HeaderContentDescription            = "Content-Description"
	HeaderContentTransferEncoding       = "Content-Transfer-Encoding"
	HeaderCookie                        = "Cookie"
	HeaderSetCookie                     = "Set-Cookie"
	HeaderIfModifiedSince               = "If-Modified-Since"
	HeaderLastModified                  = "Last-Modified"
	HeaderLocation                      = "Location"
	HeaderReferer                       = "Referer"
	HeaderUserAgent                     = "User-Agent"
	HeaderUpgrade                       = "Upgrade"
	HeaderVary                          = "Vary"
	HeaderWWWAuthenticate               = "WWW-Authenticate"
	HeaderXForwardedProto               = "X-Forwarded-Proto"
	HeaderXHTTPMethodOverride           = "X-HTTP-Method-Override"
	HeaderXForwardedFor                 = "X-Forwarded-For"
	HeaderXRealIP                       = "X-Real-IP"
	HeaderXRequestedWith                = "X-Requested-With"
	HeaderServer                        = "Server"
	HeaderOrigin                        = "Origin"
	HeaderAccessControlRequestMethod    = "Access-Control-Request-Method"
	HeaderAccessControlRequestHeaders   = "Access-Control-Request-Headers"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	HeaderExpires                       = "Expires"
	HeaderCacheControl                  = "Cache-Control"
	HeaderPragma                        = "Pragma"

	// Security
	HeaderStrictTransportSecurity = "Strict-Transport-Security"
	HeaderXContentTypeOptions     = "X-Content-Type-Options"
	HeaderXXSSProtection          = "X-XSS-Protection"
	HeaderXFrameOptions           = "X-Frame-Options"
	HeaderContentSecurityPolicy   = "Content-Security-Policy"
	HeaderXCSRFToken              = "X-CSRF-Token"
)

// MIME types
const (
	MIMEApplicationJSON                  = "application/json"
	MIMEApplicationJSONCharsetUTF8       = MIMEApplicationJSON + "; " + CharsetUTF8
	MIMEApplicationJavaScript            = "application/javascript"
	MIMEApplicationJavaScriptCharsetUTF8 = MIMEApplicationJavaScript + "; " + CharsetUTF8
	MIMEApplicationXML                   = "application/xml"
	MIMEApplicationXMLCharsetUTF8        = MIMEApplicationXML + "; " + CharsetUTF8
	MIMETextXML                          = "text/xml"
	MIMETextXMLCharsetUTF8               = MIMETextXML + "; " + CharsetUTF8
	MIMEApplicationForm                  = "application/x-www-form-urlencoded"
	MIMEApplicationProtobuf              = "application/protobuf"
	MIMEApplicationMsgpack               = "application/msgpack"
	MIMETextHTML                         = "text/html"
	MIMETextHTMLCharsetUTF8              = MIMETextHTML + "; " + CharsetUTF8
	MIMETextPlain                        = "text/plain"
	MIMETextPlainCharsetUTF8             = MIMETextPlain + "; " + CharsetUTF8
	MIMEMultipartForm                    = "multipart/form-data"
	MIMEOctetStream                      = "application/octet-stream"
)

const (
	//---------
	// Charset
	//---------

	CharsetUTF8 = "charset=utf-8"

	//---------
	// Headers
	//---------

	AcceptEncoding     = "Accept-Encoding"
	Authorization      = "Authorization"
	ContentDisposition = "Content-Disposition"
	ContentEncoding    = "Content-Encoding"
	ContentLength      = "Content-Length"
	ContentType        = "Content-Type"
	Location           = "Location"
	Upgrade            = "Upgrade"
	Vary               = "Vary"
	WWWAuthenticate    = "WWW-Authenticate"
	XForwardedFor      = "X-Forwarded-For"
	XRealIP            = "X-Real-IP"
)

type (
	// Map is just a conversion for a map[string]interface{}
	// should not be used inside Render when PongoEngine is used.
	Map map[string]interface{}

	// Context is resetting every time a ruest is coming to the server
	// it is not good practice to use this object in goroutines, for these cases use the .Clone()
	Context struct {
		routing.Context

		Session       *libSession.Instance
		sessionStore  session.Store
		enableSession bool // Note: Never reset!
	}
)

// startSession starts session and load old session data info this controller.
func (ctx *Context) startSession() (session.Store, error) {
	if ctx.sessionStore != nil {
		return ctx.sessionStore, nil
	}
	if !ctx.enableSession {
		return nil, errNotEnableSession
	}
	var err error
	ctx.sessionStore, err = ctx.Session.SessionStart(ctx, ctx.Response, ctx.Request)
	return ctx.sessionStore, err
}

// getSessionStore return SessionStore.
func (ctx *Context) getSessionStore() (session.Store, error) {
	if ctx.sessionStore != nil {
		return ctx.sessionStore, nil
	}
	if !ctx.enableSession {
		return nil, errNotEnableSession
	}
	var err error
	ctx.sessionStore, err = ctx.Session.GetSessionStore(ctx, ctx.Response, ctx.Request)
	return ctx.sessionStore, err
}

// SetSession puts value into session.
func (ctx *Context) SetSession(key interface{}, value interface{}) {
	if _, err := ctx.startSession(); err != nil {
		return
	}
	ctx.sessionStore.Set(key, value)
}

// GetSession gets value from session.
func (ctx *Context) GetSession(key interface{}) interface{} {
	if _, err := ctx.getSessionStore(); err != nil {
		return nil
	}
	return ctx.sessionStore.Get(key)
}

// DelSession removes value from session.
func (ctx *Context) DelSession(key interface{}) {
	if _, err := ctx.getSessionStore(); err != nil {
		return
	}
	ctx.sessionStore.Delete(key)
}

// SessionRegenerateID regenerates session id for this session.
// the session data have no changes.
func (ctx *Context) SessionRegenerateID() {
	if _, err := ctx.getSessionStore(); err != nil {
		return
	}
	ctx.sessionStore.SessionRelease(ctx.Response)
	ctx.sessionStore = ctx.Session.SessionRegenerateID(ctx, ctx.Response, ctx.Request)
}

// DestroySession cleans session data and session cookie.
func (ctx *Context) DestroySession() {
	if _, err := ctx.getSessionStore(); err != nil {
		return
	}
	ctx.sessionStore.Flush()
	ctx.sessionStore = nil
	ctx.Session.SessionDestroy(ctx, ctx.Response, ctx.Request)
}

// Redirect replies to the request with a redirect to url,
// which may be a path relative to the request path.
//
// The provided status code should be in the 3xx range and is usually
// StatusMovedPermanently, StatusFound or StatusSeeOther.
func (ctx *Context) Redirect(status int, urlStr string) error {
	if status < http.StatusMultipleChoices || status > http.StatusPermanentRedirect {
		return fmt.Errorf("The provided status code should be in the 3xx range and is usually 301, 302 or 303, yours: %d", status)
	}
	http.Redirect(ctx.Response, ctx.Request, urlStr, status)
	return nil
}

var proxyList = &struct {
	m map[string]*httputil.ReverseProxy
	sync.RWMutex
}{
	m: map[string]*httputil.ReverseProxy{},
}

// ReverseProxy routes URLs to the scheme, host, and base path provided in targetUrlBase.
// If pathAppend is "true" and the targetUrlBase's path is "/base" and the incoming ruest was for "/dir",
// the target ruest will be for /base/dir.
func (ctx *Context) ReverseProxy(targetUrlBase string, pathAppend bool) error {
	proxyList.RLock()
	var rp = proxyList.m[targetUrlBase]
	proxyList.RUnlock()
	if rp == nil {
		proxyList.Lock()
		defer proxyList.Unlock()
		rp = proxyList.m[targetUrlBase]
		if rp == nil {
			target, err := url.Parse(targetUrlBase)
			if err != nil {
				return err
			}
			targetQuery := target.RawQuery
			rp = &httputil.ReverseProxy{
				Director: func(r *http.Request) {
					r.Host = target.Host
					r.URL.Scheme = target.Scheme
					r.URL.Host = target.Host
					r.URL.Path = path.Join(target.Path, r.URL.Path)
					if targetQuery == "" || r.URL.RawQuery == "" {
						r.URL.RawQuery = targetQuery + r.URL.RawQuery
					} else {
						r.URL.RawQuery = targetQuery + "&" + r.URL.RawQuery
					}
				},
			}
			proxyList.m[targetUrlBase] = rp
		}
	}

	if !pathAppend {
		ctx.Request.URL.Path = ""
	}
	rp.ServeHTTP(ctx.Response, ctx.Request)
	return nil
}

func (ctx *Context) beforeWriteHeader() {
	if ctx.enableSession {
		if ctx.sessionStore != nil {
			ctx.sessionStore.SessionRelease(ctx.Response)
			ctx.sessionStore = nil
		}
	}
}

// SecureCookieParam Get secure cookie from request by a given key.
func (ctx *Context) SecureCookieParam(secret, key string) (string, bool) {
	val := ctx.CookieParam(key)
	if val == "" {
		return "", false
	}

	parts := strings.SplitN(val, "|", 3)

	if len(parts) != 3 {
		return "", false
	}

	vs := parts[0]
	timestamp := parts[1]
	sig := parts[2]

	h := hmac.New(sha1.New, []byte(secret))
	fmt.Fprintf(h, "%s%s", vs, timestamp)

	if fmt.Sprintf("x", h.Sum(nil)) != sig {
		return "", false
	}
	res, _ := base64.URLEncoding.DecodeString(vs)
	return libUtility.BytesToString(res), true
}

// CookieParam returns request cookie item string by a given key.
// if non-existed, return empty string.
func (ctx *Context) CookieParam(key string) string {
	cookie, err := ctx.Context.Request.Cookie(key)
	if err != nil {
		return ""
	}
	return cookie.Value
}
