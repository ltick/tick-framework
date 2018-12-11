package ltick

import (
	"context"
	"fmt"
	libLog "log"
	"net/http"
	"time"

	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/ltick/tick-routing/content"
	"github.com/ltick/tick-routing/fault"
)

var (
	errNewDefaultServer = "ltick: new default server"
	errProxyConfig      = "ltick: proxy config '%v'"
)
var defaultEnvPrefix = "LTICK"
var defaultConfigFile = "etc/ltick.json"

var defaultEngineCallback Callback

func SetDefaultEngineCallback(c Callback) {
	defaultEngineCallback = c
}

func DefaultConfigFile() string {
	return defaultConfigFile
}

var defaultDotenvFile = ".env"

func DefaultDotenvFile() string {
	return defaultDotenvFile
}

var defaultConfigReloadTime = 120 * time.Second

func DefaultConfigReloadTime() time.Duration {
	return defaultConfigReloadTime
}

var CustomDefaultLogFunc utility.LogFunc

func SetDefaultLogFunc(defaultLogFunc utility.LogFunc) {
	CustomDefaultLogFunc = defaultLogFunc
}

func DefaultLogFunc(ctx context.Context, format string, data ...interface{}) {
	if CustomDefaultLogFunc != nil {
		CustomDefaultLogFunc(ctx, format, data...)
	} else {
		libLog.Printf(format, data...)
	}
}

func DefaultErrorHandler(c *routing.Context, err error) error {
	if c == nil {
		return routing.NewHTTPError(http.StatusInternalServerError)
	}
	if httpError, ok := err.(routing.HTTPError); ok {
		status := httpError.StatusCode()
		switch status {
		case http.StatusBadRequest:
			fallthrough
		case http.StatusForbidden:
			fallthrough
		case http.StatusNotFound:
			fallthrough
		case http.StatusRequestTimeout:
			fallthrough
		case http.StatusMethodNotAllowed:
			DefaultLogFunc(c.Context, `LTICK_CLIENT_ERROR|%s|%s|%s|%s|%s`, c.Get("forwardRequestId"), c.Get("requestId"), c.Get("serverAddress"), err.Error(), c.Get("errorStack"))
		case http.StatusNoContent:
		default:
			DefaultLogFunc(c.Context, `LTICK_SERVER_ERROR|%s|%s|%s|%s|%s`, c.Get("forwardRequestId"), c.Get("requestId"), c.Get("serverAddress"), err.Error(), c.Get("errorStack"))
		}
		content.TypeNegotiator(XML, XML2, JSON)(c)
		return routing.NewHTTPError(status, httpError.Error())
	} else {
		DefaultLogFunc(c.Context, `LTICK_SERVER_ERROR|%s|%s|%s|%s|%s`, c.Get("forwardRequestId"), c.Get("requestId"), c.Get("serverAddress"), err.Error(), c.Get("errorStack"))
		return routing.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return nil
}

var CustomDefaultErrorLogFunc fault.LogFunc

func SetDefaultErrorLogFunc(defaultErrorLogFunc fault.LogFunc) {
	CustomDefaultErrorLogFunc = defaultErrorLogFunc
}

func DefaultErrorLogFunc() fault.LogFunc {
	if CustomDefaultErrorLogFunc != nil {
		return CustomDefaultErrorLogFunc
	} else {
		return libLog.Printf
	}
}

func DefaultTimeoutHandler() routing.Handler {
	return defaultTimeoutHandler
}

func defaultTimeoutHandler(c *routing.Context) error {
	return routing.NewHTTPError(http.StatusRequestTimeout)
}

func DefaultAccessLogFunc(c *routing.Context, rw *access.LogResponseWriter, elapsed float64) {
	//来源请求ID
	forwardRequestId := c.Get("uniqid")
	//请求ID
	requestId := c.Get("requestId")
	//客户端IP
	clientIP := c.Get("clientIP")
	//服务端IP
	serverAddress := c.Get("serverAddress")
	requestLine := fmt.Sprintf("%s %s %s", c.Request.Method, c.Request.RequestURI, c.Request.Proto)
	debug := new(bool)
	if c.Get("DEBUG") != nil {
		*debug = c.Get("DEBUG").(bool)
	}
	if *debug {
		DefaultLogFunc(c.Context, `LTICK_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, forwardRequestId, requestId, serverAddress, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
	} else {
		DefaultLogFunc(c.Context, `LTICK_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, forwardRequestId, requestId, serverAddress, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
	}
	if *debug {
		DefaultLogFunc(c.Context, `%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
	} else {
		DefaultLogFunc(c.Context, `%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
	}
}
