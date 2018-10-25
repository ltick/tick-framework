package ltick

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/api"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/ltick/tick-routing/content"
	"github.com/ltick/tick-routing/cors"
	"github.com/ltick/tick-routing/fault"
	"github.com/ltick/tick-routing/file"
	"github.com/ltick/tick-routing/slash"
)

type (
	Server struct {
		gracefulStopTimeout time.Duration
		systemLogWriter     io.Writer
		Port                uint
		Router              *ServerRouter
		RouteGroups         map[string]*ServerRouteGroup
		mutex               sync.RWMutex
	}
	ServerRouterProxy struct {
		Host     string
		Group    string
		Path     string
		Upstream string
	}
	ServerRouterRoute struct {
		Host     string
		Group    string
		Method   string
		Path     string
		Handlers []routing.Handler
	}
	ServerRouter struct {
		*routing.Router
		middlewares []MiddlewareInterface
		proxys      []*ServerRouterProxy
		routes      []*ServerRouterRoute
	}
	ServerRouteGroup struct {
		*routing.RouteGroup
	}
	RouterCallback interface {
		OnRequestStartup(*routing.Context) error
		OnRequestShutdown(*routing.Context) error
	}
)

func (sp *ServerRouterProxy) Proxy(c *routing.Context) (*url.URL, error) {
	captures := make(map[string]string)
	r := regexp.MustCompile("<:(\\w+)>")
	match := r.FindStringSubmatch(sp.Upstream)
	if match == nil {
		return nil, errors.New("ltick: upstream not match")
	}
	for i, name := range r.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		if name != "group" {
			captures[name] = c.Param(name)
		}
	}
	captures["group"] = sp.Group
	if len(captures) != 0 {
		upstream := sp.Upstream
		//拼接配置文件指定中的uri，$符号分割
		for name, capture := range captures {
			upstream = strings.Replace(upstream, "<:"+name+">", capture, -1)
		}
		UpstreamURL, err := url.Parse(upstream)
		if err != nil {
			return nil, err
		}
		return UpstreamURL, nil
	}
	return nil, nil
}

func (e *Engine) NewDefaultServer(name string, routerCallback RouterCallback, requestTimeoutHandlers ...routing.Handler) (server *Server) {
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
	router.WithAccessLogger(utility.DefaultAccessLogFunc).
		WithErrorHandler(log.Printf, utility.DefaultErrorLogFunc).
		WithPanicLogger(log.Printf).
		WithTypeNegotiator(JSON, XML, XML2, HTML).
		WithSlashRemover(http.StatusMovedPermanently).
		WithLanguageNegotiator("zh-CN", "en-US").
		WithCors(CorsAllowAll).
		WithCallback(routerCallback)
	server = e.NewServer(name, port, gracefulStopTimeout, router)
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
func (e *Engine) NewServer(name string, port uint, gracefulStopTimeout time.Duration, router *ServerRouter) (server *Server) {
	if _, ok := e.ServerMap[name]; ok {
		fmt.Printf(errNewServer+": server '%s' already exists\r\n", name)
		os.Exit(1)
	}
	server = &Server{
		systemLogWriter:     e.systemLogWriter,
		Port:                port,
		gracefulStopTimeout: gracefulStopTimeout,
		Router:              router,
		RouteGroups:         map[string]*ServerRouteGroup{},
		mutex:               sync.RWMutex{},
	}
	middlewares := make([]MiddlewareInterface, 0)
	for _, sortedMiddleware := range e.Registry.GetSortedMiddlewares() {
		middleware, ok := sortedMiddleware.(MiddlewareInterface)
		if !ok {
			continue
		}
		middlewares = append(middlewares, middleware)
	}
	server.Router.WithMiddlewares(middlewares)
	e.ServerMap[name] = server
	e.SystemLog(fmt.Sprintf("ltick: new server [name:'%s', port:'%d', gracefulStopTimeout:'%.fs', requestTimeout:'%.fs']", name, port, gracefulStopTimeout.Seconds(), router.TimeoutDuration.Seconds()))
	return server
}
func (e *Engine) SetServerLogFunc(name string, accessLogFunc access.LogWriterFunc, faultLogFunc fault.LogFunc, recoveryHandler ...fault.ConvertErrorFunc) *Engine {
	server := e.GetServer(name)
	if server != nil {
		server.SetLogFunc(accessLogFunc, faultLogFunc, recoveryHandler...)
	}
	return e
}

func (e *Engine) SetServerReuqestSlashRemover(name string, status int) *Engine {
	server := e.GetServer(name)
	if server != nil {
		server.SetReuqestSlashRemover(status)
	}
	return e
}
func (e *Engine) SetServerReuqestCors(name string, corsOptions cors.Options) *Engine {
	server := e.GetServer(name)
	if server != nil {
		if &corsOptions != nil {
			server.Router.WithCors(corsOptions)
		} else {
			server.Router.WithCors(CorsAllowAll)
		}
	}
	return e
}
func (e *Engine) GetServer(name string) *Server {
	if _, ok := e.ServerMap[name]; ok {
		return e.ServerMap[name]
	}
	return nil
}
func (e *Engine) GetServerMap() map[string]*Server {
	return e.ServerMap
}

/********** Server **********/
func (s *Server) Get(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "GET",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Post(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "POST",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Put(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "PUT",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Patch(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "PATCH",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Delete(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "DELETE",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Connect(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "CONNECT",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Options(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "OPTIONS",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Trace(host string, group string, path string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "TRACE",
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Proxy(host string, group string, path string, upstream string) *Server {
	s.Router.proxys = append(s.Router.proxys, &ServerRouterProxy{
		Host:     host,
		Group:    group,
		Path:     path,
		Upstream: upstream,
	})
	return s
}
func (s *Server) Pprof(host string, group string) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method: "ANY",
		Host:   host,
		Group:  group,
		Path:   "pprof",
		Handlers: []routing.Handler{
			routing.HTTPHandlerFunc(pprof.Index),
		},
	})
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method: "ANY",
		Host:   host,
		Group:  group,
		Path:   "pprof/cmdline",
		Handlers: []routing.Handler{
			routing.HTTPHandlerFunc(pprof.Cmdline),
		},
	})
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method: "ANY",
		Host:   host,
		Group:  group,
		Path:   "pprof/profile",
		Handlers: []routing.Handler{
			routing.HTTPHandlerFunc(pprof.Profile),
		},
	})
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method: "ANY",
		Host:   host,
		Group:  group,
		Path:   "pprof/symbol",
		Handlers: []routing.Handler{
			routing.HTTPHandlerFunc(pprof.Symbol),
		},
	})
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method: "ANY",
		Host:   host,
		Group:  group,
		Path:   "pprof/trace",
		Handlers: []routing.Handler{
			routing.HTTPHandlerFunc(pprof.Trace),
		},
	})
	return s
}
func (s *Server) SetLogFunc(accessLogFunc access.LogWriterFunc, faultLogFunc fault.LogFunc, recoveryHandler ...fault.ConvertErrorFunc) *Server {
	s.Router.WithAccessLogger(accessLogFunc).
		WithRecoveryHandler(faultLogFunc, recoveryHandler...)
	return s
}
func (s *Server) SetReuqestSlashRemover(status int) *Server {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound:
		s.Router.WithSlashRemover(status)
	}
	return s
}
func (s *Server) SetServerReuqestCors(corsOptions cors.Options) *Server {
	if &corsOptions != nil {
		s.Router.WithCors(corsOptions)
	} else {
		s.Router.WithCors(CorsAllowAll)
	}
	return s
}
func (s *Server) AddRouteGroup(group string) *ServerRouteGroup {
	// Router Handlers (Global)
	startupHandlers := combineHandlers(s.Router.GetStartupHandlers(), s.Router.GetAnteriorHandlers())
	shutdownHandlers := combineHandlers(s.Router.GetPosteriorHandlers(), s.Router.GetShutdownHandlers())
	s.RouteGroups[group] = s.Router.AddRouteGroup(group, startupHandlers, shutdownHandlers)
	return s.RouteGroups[group]
}
func (s *Server) AddRoute(method string, path string, handlers ...routing.Handler) *Server {
	paths := strings.Split(path, "/")
	prefix := "/"
	for _, routePrefix := range paths {
		if _, ok := s.RouteGroups[routePrefix]; ok {
			prefix = routePrefix
			break
		}
	}
	s.RouteGroups[prefix].AddRoute(method, strings.Replace(path, prefix, "", 1), handlers...)
	return s
}
func (s *Server) GetRouter() *ServerRouter {
	return s.Router
}
func (s *Server) GetRouteGroup(name string) *ServerRouteGroup {
	if _, ok := s.RouteGroups[name]; !ok {
		return nil
	}
	return s.RouteGroups[name]
}
func (s *Server) SystemLog(args ...interface{}) {
	fmt.Fprintln(s.systemLogWriter, args...)
}
func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.Router.ServeHTTP(res, req)
}

// Middlewares
func (r *ServerRouter) WithMiddlewares(middlewares []MiddlewareInterface) *ServerRouter {
	r.middlewares = middlewares
	return r
}

// The LogWriterFunc is provided with the http.Request and LogResponseWriter objects for the
// request, as well as the elapsed time since the request first came through the middleware.
// LogWriterFunc can then do whatever logging it needs to do.
//
//     import (
//         "log"
//         "github.com/ltick/tick-framework"
//         "net/http"
//     )
//
//     func myCustomLogger(req http.Context, res access.LogResponseWriter, elapsed int64) {
//         // Do something with the request, response, and elapsed time data here
//     }
//     r := routing.New()
//     r.UseAccessLogger(AccessLogger(myCustomLogger))
func (r *ServerRouter) WithAccessLogger(loggerFunc access.LogWriterFunc) *ServerRouter {
	r.AppendStartupHandler(access.CustomLogger(loggerFunc))
	return r
}

// The method takes a list of languages (locale IDs) that are supported by the application.
// The negotiator will determine the best language to use by checking the Accept-Language request header.
// If no match is found, the first language will be used.
//
// In a handler, you can access the chosen language through routing.Context like the following:
//
//     func(c *routing.Context) error {
//         language := c.Get(content.Language).(string)
//     }
//
// If you do not specify languages, the negotiator will set the language to be "en-US".
func (r *ServerRouter) WithLanguageNegotiator(languages ...string) *ServerRouter {
	r.AppendStartupHandler(content.LanguageNegotiator(languages...))
	return r
}

// TypeNegotiator returns a content type negotiation handler.
//
// The method takes a list of response MIME types that are supported by the application.
// The negotiator will determine the best response MIME type to use by checking the "Accept" HTTP header.
// If no match is found, the first MIME type will be used.
//
// The negotiator will set the "Content-Type" response header as the chosen MIME type. It will call routing.Context.SetDataWriter()
// to set the appropriate data writer that can write data in the negotiated format.
//
// If you do not specify any supported MIME types, the negotiator will use "text/html" as the response MIME type.
const (
	JSON = content.JSON
	XML  = content.XML
	XML2 = content.XML2
	HTML = content.HTML
)

func (r *ServerRouter) AddTypeNegotiator(mime string, writer routing.DataWriter) *ServerRouter {
	content.DataWriters[mime] = writer
	return r
}

func (r *ServerRouter) WithTypeNegotiator(formats ...string) *ServerRouter {
	r.AppendStartupHandler(content.TypeNegotiator(formats...))
	return r
}

func (r *ServerRouter) WithPanicLogger(logf fault.LogFunc) *ServerRouter {
	r.AppendStartupHandler(fault.PanicHandler(logf))
	return r
}

func (r *ServerRouter) WithErrorHandler(logf fault.LogFunc, errorf ...fault.ConvertErrorFunc) *ServerRouter {
	r.AppendStartupHandler(fault.ErrorHandler(logf, errorf...))
	return r
}

func (r *ServerRouter) WithRecoveryHandler(logf fault.LogFunc, errorf ...fault.ConvertErrorFunc) *ServerRouter {
	r.AppendStartupHandler(fault.Recovery(logf, errorf...))
	return r
}

var CorsAllowAll = cors.Options{
	AllowOrigins: "*",
	AllowHeaders: "*",
	AllowMethods: "*",
}

func (r *ServerRouter) WithCors(opts cors.Options) *ServerRouter {
	r.AppendStartupHandler(cors.Handler(opts))
	return r
}

// The handler will redirect the browser to the new URL without the trailing slash.
// The status parameter should be either http.StatusMovedPermanently (301) or http.StatusFound (302), which is to
// be used for redirecting GET requests. For other requests, the status code will be http.StatusTemporaryRedirect (307).
// If the original URL has no trailing slash, the handler will do nothing. For example,
//
//     import (
//         "net/http"
//         "github.com/ltick/tick-framework"
//     )
//
//     r := routing.New()
//     r.AppendStartupHandler(slash.WithSlashRemover(http.StatusMovedPermanently))
//
// Note that Remover relies on HTTP redirection to remove the trailing slashes.
// If you do not want redirection, please set `Router.IgnoreTrailingSlash` to be true without using Remover.
func (r *ServerRouter) WithSlashRemover(status int) *ServerRouter {
	r.AppendStartupHandler(slash.Remover(status))
	return r
}

// The files being served are determined using the current URL path and the specified path map.
// For example, if the path map is {"/css": "/www/css", "/js": "/www/js"} and the current URL path
// "/css/main.css", the file "<working dir>/www/css/main.css" will be served.
// If a URL path matches multiple prefixes in the path map, the most specific prefix will take precedence.
// For example, if the path map contains both "/css" and "/css/img", and the URL path is "/css/img/logo.gif",
// then the path mapped by "/css/img" will be used.
//
//     import (
//         "log"
//         "github.com/ltick/tick-framework"
//     )
//
//     a := New("app1", "Test Application 1", &AppInitFunc{})
//     server := a.AddServer(8080, 30*time.Second, 3*time.Second)
//     server.AddRoute("/*", server.FileServer(file.PathMap{
//          "/css": "/ui/dist/css",
//          "/js": "/ui/dist/js",
//     }))
func (r *ServerRouter) NewFileHandler(pathMap file.PathMap, opts ...file.ServerOptions) routing.Handler {
	return file.Server(pathMap, opts...)
}

func (r *ServerRouter) WithCallback(callback RouterCallback) *ServerRouter {
	if callback != nil {
		r.PrependStartupHandler(callback.OnRequestStartup)
		r.AppendShutdownHandler(callback.OnRequestShutdown)
	}
	return r
}

func (r *ServerRouter) AddRouteGroup(groupName string, startupHandlers []routing.Handler, shutdownHandlers []routing.Handler) *ServerRouteGroup {
	g := &ServerRouteGroup{
		RouteGroup: r.Group(groupName, startupHandlers, shutdownHandlers),
	}
	for _, m := range r.middlewares {
		g.AppendAnteriorHandler(m.OnRequestStartup)
		g.PrependPosteriorHandler(m.OnRequestShutdown)
	}
	return g
}

func (g *ServerRouteGroup) WithCallback(callback RouterCallback) *ServerRouteGroup {
	if callback != nil {
		g.PrependStartupHandler(callback.OnRequestStartup)
		g.AppendShutdownHandler(callback.OnRequestShutdown)
	}
	return g
}

// 添加API路由
// 可进行参数校验
func (g *ServerRouteGroup) AddApiRoute(method string, path string, handlers ...api.APIHandler) {
	routeHandlers := make([]routing.Handler, len(handlers))
	for index, handler := range handlers {
		routeHandlers[index] = func(ctx *routing.Context) error {
			apiCtx := &api.Context{
				Context: ctx,
				Response: api.NewResponse(ctx.ResponseWriter),
			}
			err := handler.Serve(apiCtx)
			if err != nil {
				if httpError, ok := err.(routing.HTTPError); ok {
					return routing.NewHTTPError(httpError.StatusCode(), httpError.Error())
				}
				return err
			}
			return nil
		}
	}
	g.AddRoute(method, path, routeHandlers...)
}

// 添加API路由
func (g *ServerRouteGroup) AddRoute(method string, path string, handlers ...routing.Handler) {
	switch strings.ToUpper(method) {
	case "GET":
		g.Get(path, handlers...)
	case "POST":
		g.Post(path, handlers...)
	case "PUT":
		g.Put(path, handlers...)
	case "PATCH":
		g.Patch(path, handlers...)
	case "DELETE":
		g.Delete(path, handlers...)
	case "CONNECT":
		g.Connect(path, handlers...)
	case "HEAD":
		g.Head(path, handlers...)
	case "OPTIONS":
		g.Options(path, handlers...)
	case "TRACE":
		g.Trace(path, handlers...)
	case "ANY":
		g.Any(path, handlers...)
	default:
		g.To(method, path, handlers...)
	}
}

// combineHandlers merges two lists of handlers into a new list.
func combineHandlers(h1 []routing.Handler, h2 []routing.Handler) []routing.Handler {
	hh := make([]routing.Handler, len(h1)+len(h2))
	copy(hh, h1)
	copy(hh[len(h1):], h2)
	return hh
}
