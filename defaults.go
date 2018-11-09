package ltick

import (
	"context"
	"fmt"
	libLog "log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/ltick/tick-routing/fault"
)

var defaultEnvPrefix = "LTICK"
var defaultConfigPath = "etc/ltick.json"

func DefaultConfigPath() string {
	return defaultConfigPath
}

var defaultDotenvPath = ".env"

func DefaultDotenvPath() string {
	return defaultDotenvPath
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

func DefaultEngine(registry *Registry, options map[string]config.Option) (engine *Engine) {
	defaultConfigFile, err := filepath.Abs(defaultConfigPath)
	if err != nil {
		e := errors.Annotatef(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	defaultDotenvFile, err := filepath.Abs(defaultDotenvPath)
	if err != nil {
		e := errors.Annotatef(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	return New(defaultConfigFile, defaultDotenvFile, defaultEnvPrefix, registry, options)
}

func NewDefaultServer(e *Engine, routerCallback RouterCallback, requestTimeoutHandlers ...routing.Handler) (server *Server) {
	// configer
	configComponent, err := e.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errLoadCachedConfig)
		fmt.Println(errors.ErrorStack(e))
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errLoadCachedConfig)
		fmt.Println(errors.ErrorStack(e))
	}
	port := uint(configer.GetInt("server.port"))
	if port == 0 {
		fmt.Printf("ltick: new classic server [error: 'server port is empty']\n")
		os.Exit(1)
	}
	gracefulStopTimeout := configer.GetDuration("server.graceful_stop_timeout")
	requestTimeout := configer.GetDuration("server.request_timeout")
	router := &ServerRouter{
		Router: routing.New(e.Context),
		routes: make([]*ServerRouterRoute, 0),
		proxys: make([]*ServerRouterProxy, 0),
	}
	if len(requestTimeoutHandlers) > 0 {
		router.Router.Timeout(requestTimeout, requestTimeoutHandlers[0])
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
	proxys := configer.GetStringMap("router.proxy")
	if proxys != nil {
		if len(proxys) != 0 {
			for proxyHost, proxyInterface := range proxys {
				proxyConfigs, ok := proxyInterface.([]interface{})
				if !ok {
					fmt.Println("request: read all proxy config to array error")
					os.Exit(1)
				}
				for _, proxyConfig := range proxyConfigs {
					proxy, ok := proxyConfig.(map[string]interface{})
					if !ok {
						fmt.Println("request: read proxy config to map error")
						os.Exit(1)
					}
					proxyUpstream, ok := proxy["upstream"].(string)
					if !ok {
						fmt.Println("request: read one proxy config upstream error")
						os.Exit(1)
					}
					proxyGroup, ok := proxy["group"].(string)
					if !ok {
						fmt.Println("request: read proxy config group error")
						os.Exit(1)
					}
					proxyPath, ok := proxy["path"].(string)
					if !ok {
						fmt.Println("request: read proxy config path error")
						os.Exit(1)
					}
					server.Proxy(proxyHost, proxyGroup, proxyPath, proxyUpstream)
				}
			}
		}
	}
	return server
}
