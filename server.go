package ltick

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ltick/tick-framework/module"
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
		Route    string
		Upstream string
	}
	ServerRouterRoute struct {
		Host     string
		Method   string
		Route    string
		Handlers []routing.Handler
	}
	ServerRouter struct {
		*routing.Router
		proxys   []*ServerRouterProxy
		routes   []*ServerRouterRoute
		callback RouterCallback
	}
	ServerRouteGroup struct {
		*routing.RouteGroup
		callback RouterCallback
	}
	RouterCallback interface {
		OnRequestStartup(context.Context, *routing.Context) (context.Context, error)
		OnRequestShutdown(context.Context, *routing.Context) (context.Context, error)
	}
)

func (sp *ServerRouterProxy) FindStringSubmatchMap(s string) map[string]string {
	captures := make(map[string]string)
	r := regexp.MustCompile(sp.Route)
	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}
	for i, name := range r.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		captures[name] = match[i]
	}
	return captures
}

func (sp *ServerRouterProxy) MatchProxy(r *http.Request) (*url.URL, error) {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	RequestURL := host + r.RequestURI
	HitRule := sp.FindStringSubmatchMap(RequestURL)
	if len(HitRule) != 0 {
		//命中
		upstreamURL, err := url.Parse(sp.Upstream)
		if err != nil {
			return nil, err
		}
		UpstreamRequestURI := "/"
		//拼接配置文件指定中的uri，$符号分割
		pathArr := strings.Split(strings.TrimLeft(upstreamURL.Path, "/"), "$")
		for _, path := range pathArr {
			if _, ok := HitRule[path]; ok {
				UpstreamRequestURI += strings.TrimLeft(HitRule[path], "/") + "/"
			}
		}
		Upstream := upstreamURL.Scheme + "://" + upstreamURL.Host + strings.TrimRight(UpstreamRequestURI, "/")
		UpstreamURL, err := url.Parse(Upstream)
		if err != nil {
			return nil, err
		}
		return UpstreamURL, nil
	}
	return nil, nil
}

func (e *Engine) NewClassicServer(name string, requestTimeoutHandlers ...routing.Handler) (server *Server) {
	port := uint(e.Config.GetInt("server.port"))
	if port == 0 {
		fmt.Printf("ltick: new classic server [error: 'server port is empty']\n")
		os.Exit(1)
	}
	gracefulStopTimeout := e.Config.GetDuration("server.graceful_stop_timeout")
	requestTimeout := e.Config.GetDuration("server.request_timeout")
	server = e.NewServer(name, port, gracefulStopTimeout, requestTimeout, requestTimeoutHandlers...)
	proxyMaps := e.Config.GetStringMap("router.proxy")
	if proxyMaps != nil {
		if len(proxyMaps) != 0 {
			for proxyHost, proxyMap := range proxyMaps {
				proxyMap, ok := proxyMap.(map[string]interface{})
				if !ok {
					fmt.Printf("request: read all proxy config to map error\n")
					os.Exit(1)
				}
				proxyUpstream, ok := proxyMap["upstream"].(string)
				if !ok {
					fmt.Printf("request: read one proxy Config upstream error\n")
					os.Exit(1)
				}
				proxyRules, ok := proxyMap["rule"].(map[string]interface{})
				if !ok {
					fmt.Printf("request: read proxy config rule error\n")
					os.Exit(1)
				}
				for _, routeInterface := range proxyRules {
					proxyRoute, ok := routeInterface.(string)
					if ok {
						server.Proxy(proxyHost, proxyRoute, proxyUpstream)
					}
				}
			}
		}
	}
	return server
}
func (e *Engine) NewServer(name string, port uint, gracefulStopTimeout time.Duration, requestTimeout time.Duration, requestTimeoutHandlers ...routing.Handler) (server *Server) {
	if _, ok := e.Servers[name]; ok {
		fmt.Printf(errNewServer+": server '%s' already exists\r\n", name)
		os.Exit(1)
	}
	server = &Server{
		systemLogWriter:     e.systemLogWriter,
		Port:                port,
		gracefulStopTimeout: gracefulStopTimeout,
		Router: &ServerRouter{
			Router: routing.New(e.Context).Timeout(requestTimeout, requestTimeoutHandlers...),
			routes: make([]*ServerRouterRoute, 0),
			proxys: make([]*ServerRouterProxy, 0),
		},
		mutex: sync.RWMutex{},
	}
	modules := make([]module.ModuleInterface, 0)
	for _, sortedModule := range e.Module.GetSortedModules() {
		module, ok := sortedModule.(*module.Module)
		if !ok {
			continue
		}
		modules = append(modules, module.Module)
	}
	server.RouteGroups["/"] = server.Router.AddRouteGroup("/", modules)
	e.Servers[name] = server
	e.SystemLog(fmt.Sprintf("ltick: new server [name:'%s', port:'%d', gracefulStopTimeout:'%.fs', requestTimeout:'%.fs']", name, port, gracefulStopTimeout.Seconds(), requestTimeout.Seconds()))
	return server
}
func (e *Engine) SetServerLogFunc(name string, accessLogFunc access.LogWriterFunc, faultLogFunc fault.LogFunc, recoveryHandler ...fault.ConvertErrorFunc) *Engine {
	server := e.GetServer(name)
	if server != nil {
		server.SetLogFunc(accessLogFunc, faultLogFunc, recoveryHandler...)
	}
	return e
}
func (e *Engine) SetServerReuqestCallback(name string, reuqestCallback RouterCallback) *Engine {
	server := e.GetServer(name)
	if server != nil {
		server.SetReuqestCallback(reuqestCallback)
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
	if _, ok := e.Servers[name]; ok {
		return e.Servers[name]
	}
	return nil
}
func (s *Server) Get(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "GET",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Post(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "POST",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Put(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "PUT",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Patch(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "PATCH",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Delete(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "DELETE",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Connect(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "CONNECT",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Options(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "OPTIONS",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Trace(host string, route string, handlers ...routing.Handler) *Server {
	s.Router.routes = append(s.Router.routes, &ServerRouterRoute{
		Method:   "TRACE",
		Host:     host,
		Route:    route,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Proxy(host string, route string, upstream string) *Server {
	s.Router.proxys = append(s.Router.proxys, &ServerRouterProxy{
		Host:     host,
		Route:    route,
		Upstream: upstream,
	})
	return s
}
func (s *Server) SetLogFunc(accessLogFunc access.LogWriterFunc, faultLogFunc fault.LogFunc, recoveryHandler ...fault.ConvertErrorFunc) *Server {
	s.Router.WithAccessLogger(accessLogFunc).
		WithRecoveryHandler(faultLogFunc, recoveryHandler...)
	return s
}
func (s *Server) SetReuqestCallback(reuqestCallback RouterCallback) *Server {
	s.Router.WithCallback(reuqestCallback)
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
func (s *Server) addRoute(method string, path string, handlers ...routing.Handler) *Server {
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
	if _, ok := s.RouteGroups[name]; ok {
		return nil
	}
	return s.RouteGroups[name]
}
func (s *Server) SystemLog(args ...interface{}) {
	fmt.Fprintln(s.systemLogWriter, args...)
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
	r.PrependStartupHandler(callback.OnRequestStartup)
	r.AppendShutdownHandler(callback.OnRequestShutdown)
	return r
}

func (r *ServerRouter) AddRouteGroup(groupName string, modules []module.ModuleInterface, handlers ...routing.Handler) *ServerRouteGroup {
	g := &ServerRouteGroup{
		RouteGroup: r.Router.Group(groupName, handlers, nil),
	}
	for _, m := range modules {
		g.AppendAnteriorHandler(m.OnRequestStartup)
		g.PrependPosteriorHandler(m.OnRequestShutdown)
	}
	return g
}

func (g *ServerRouteGroup) WithCallback(callback RouterCallback) *ServerRouteGroup {
	g.PrependStartupHandler(callback.OnRequestStartup)
	g.AppendShutdownHandler(callback.OnRequestShutdown)
	return g
}

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
