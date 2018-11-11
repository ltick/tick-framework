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

func NewDefaultServer(e *Engine, routerCallback RouterCallback, handlers map[string]routing.Handler, timeoutHandlers ...routing.Handler) (server *Server) {
	// configer
	port := uint(e.configer.GetInt("server.port"))
	if port == 0 {
		errors.Annotatef(errors.New("ltick: server port is 0"), errNewDefaultServer)
		os.Exit(1)
	}
	gracefulStopTimeout := e.configer.GetDuration("server.graceful_stop_timeout")
	requestTimeout := e.configer.GetDuration("server.request_timeout")
	router := &ServerRouter{
		Router: routing.New(e.Context),
		routes: make([]*ServerRouterRoute, 0),
		proxys: make([]*ServerRouterProxy, 0),
	}
	if len(timeoutHandlers) > 0 {
		router.Router.Timeout(requestTimeout, timeoutHandlers[0])
	}
	middlewares := make([]MiddlewareInterface, 0)
	for _, sortedMiddleware := range e.Registry.GetSortedMiddlewares() {
		middleware, ok := sortedMiddleware.(MiddlewareInterface)
		if !ok {
			continue
		}
		middlewares = append(middlewares, middleware)
	}
	router.WithAccessLogger(DefaultAccessLogFunc).
		WithErrorHandler(DefaultFaultLogFunc(), DefaultErrorLogFunc).
		WithPanicLogger(DefaultFaultLogFunc()).
		WithTypeNegotiator(JSON, XML, XML2, HTML).
		WithSlashRemover(http.StatusMovedPermanently).
		WithLanguageNegotiator("zh-CN", "en-US").
		WithCors(CorsAllowAll).
		WithCallback(routerCallback).
		WithMiddlewares(middlewares)
	server = NewServer(port, gracefulStopTimeout, router, e.logWriter)
	server.AddRouteGroup("/")
	server.Pprof("*", "debug")
	err := e.configer.ConfigureFileConfig(server.Router.proxys, e.configFile, nil, "router.proxy")
	if err != nil {
		err := errors.Annotatef(err, errProxyConfig, e.configer.GetStringSlice("router.proxy"))
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	routes := e.configer.GetStringSlice("router.route")
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
	err = e.configer.ConfigureJsonConfig(server.Router.routes, routesConfig, handlerProviders)
	if err != nil {
		err := errors.Annotatef(err, errProxyConfig, routes)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	return server
}
