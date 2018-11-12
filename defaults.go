package ltick

import (
	"context"
	"encoding/json"
	"fmt"
	libLog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/ltick/tick-routing/fault"
)

var (
	errNewDefaultServer = "ltick: new default server"
	errProxyConfig      = "ltick: proxy config '%v'"
)
var defaultEnvPrefix = "LTICK"
var defaultConfigFile = "etc/ltick.json"

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

func DefaultErrorLogFunc(c *routing.Context, err error) error {
	DefaultLogFunc(c.Context, `LTICK|%s|%s|%s|%s|%s`, c.Get("forwardRequestId"), c.Get("requestId"), c.Get("serverAddress"), err.Error(), c.Get("errorStack"))
	return nil
}

var CustomDefaultFaultLogFunc fault.LogFunc

func SetDefaultFaultLogFunc(defaultErrorLogFunc fault.LogFunc) {
	CustomDefaultFaultLogFunc = defaultErrorLogFunc
}

func DefaultFaultLogFunc() fault.LogFunc {
	if CustomDefaultFaultLogFunc != nil {
		return CustomDefaultFaultLogFunc
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
func Default(registry *Registry, handlers map[string]routing.Handler, handlerCallbacks ...RouterCallback) (e *Engine) {
	fmt.Println("asdasda")
	// configer
	e = New(registry)
	s := e.NewDefaultServer()
	err := e.configer.ConfigureFileConfig(s, e.configFile, nil, "server")
	if err != nil {
		err := errors.Annotatef(err, errProxyConfig, e.configer.GetStringMap("server"))
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	routes := e.configer.GetStringSlice("server.router.route")
	for i, route := range routes {
		routeConfig := make(map[string]interface{})
		err := json.Unmarshal([]byte(route), routeConfig)
		if err != nil {
			err := errors.Annotatef(err, errProxyConfig, routes)
			fmt.Println(errors.ErrorStack(err))
			os.Exit(1)
		}
		routeConfigHandlers, ok := routeConfig["Handlers"].(string)
		if ok {
			routeHandlers := make([]string, 0)
			err = json.Unmarshal([]byte(routeConfigHandlers), routeHandlers)
			if err != nil {
				err := errors.Annotatef(err, errProxyConfig, routes)
				fmt.Println(errors.ErrorStack(err))
				os.Exit(1)
			}
			handlerProviderConfigs := make([]string, len(routeHandlers))
			for index, handlerName := range routeHandlers {
				handlerProviderConfigs[index] = `{"type": "` + handlerName + `"}`
			}
			routeConfig["Handlers"] = `[` + strings.Join(handlerProviderConfigs, ",") + `]`
		}
		routeConfigByte, err := json.Marshal(routeConfig)
		if err != nil {
			err := errors.Annotatef(err, errProxyConfig, routes)
			fmt.Println(errors.ErrorStack(err))
			os.Exit(1)
		}
		routes[i] = string(routeConfigByte)
	}
	routesConfig, err := json.Marshal(routes)
	if err != nil {
		err := errors.Annotatef(err, errProxyConfig, routes)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	handlerProviders := make(map[string]interface{}, len(handlers))
	for handlerName, handler := range handlers {
		handlerProviders[handlerName] = handler
	}
	fmt.Println(string(routesConfig))
	err = e.configer.ConfigureJsonConfig(s.Router.routes, routesConfig, handlerProviders)
	if err != nil {
		err := errors.Annotatef(err, errProxyConfig, routes)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	return e
}
