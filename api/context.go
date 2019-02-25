package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ltick/tick-framework/session"
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
	// Context is resetting every time a request is coming to the server
	// it is not good practice to use this object in goroutines, for these cases use the .Clone()
	Context struct {
		*routing.Context

		apiParams ApiParams // The parameter values on the URL path
		enableGzip bool

		// create param name from struct field name
		paramNameMapper ParamNameMapper
		// multipart max memory
		multipartMaxMemory int64
		// memory store byte
		memoryStoreByte []byte
		// save file dir
		fileStoreDir string

		Session       *session.Session
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
	ctx.sessionStore, err = ctx.Session.Start(ctx.ResponseWriter, ctx.Request)
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
	ctx.sessionStore, err = ctx.Session.GetStore(ctx.ResponseWriter, ctx.Request)
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
	ctx.Session.Destroy(ctx.ResponseWriter, ctx.Request)
	ctx.sessionStore = ctx.Session.RegenerateID(ctx, ctx.ResponseWriter, ctx.Request)
}

// DestroySession cleans session data and session cookie.
func (ctx *Context) DestroySession() {
	if _, err := ctx.getSessionStore(); err != nil {
		return
	}
	ctx.sessionStore.Flush()
	ctx.sessionStore = nil
	ctx.Session.Destroy(ctx.ResponseWriter, ctx.Request)
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
	http.Redirect(ctx.ResponseWriter, ctx.Request, urlStr, status)
	return nil
}

func (ctx *Context) beforeWriteHeader() {
	if ctx.enableSession {
		if ctx.sessionStore != nil {
			ctx.Session.Destroy(ctx.ResponseWriter, ctx.Request)
			ctx.sessionStore = nil
		}
	}
}
