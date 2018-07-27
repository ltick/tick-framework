package ltick

import (
	"context"
	"fmt"
	"io"
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
	"os"
)

type (
	Server struct {
		systemLogWriter     io.Writer
		Port                uint
		gracefulStopTimeout time.Duration
		Router              *ServerRouter
		RouteGroups         map[string]*ServerRouteGroup
		mutex               sync.RWMutex
	}
	ServerRouter struct {
		*routing.Router
		Module   *module.Instance
		callback RouterCallback
	}
	ServerRouteGroup struct {
		*routing.RouteGroup
		router   *ServerRouter
		callback RouterCallback
	}
	RouterCallback interface {
		OnRequestStartup(context.Context, *routing.Context) (context.Context, error)
		OnRequestShutdown(context.Context, *routing.Context) (context.Context, error)
	}
)

func (e *Engine) WithClassicServer(name string, requestTimeoutHandlers ...routing.Handler) *Engine {
	port := uint(e.Config.GetInt("server.port"))
	if port == 0 {
		fmt.Printf("ltick: new classic server [error: 'server port is empty']\n")
		os.Exit(1)
	}
	gracefulStopTimeout := e.Config.GetDuration("server.graceful_stop_timeout")
	requestTimeout := e.Config.GetDuration("server.request_timeout")
	return e.WithServer(name, port, gracefulStopTimeout, requestTimeout, requestTimeoutHandlers...)
}
func (e *Engine) WithServer(name string, port uint, gracefulStopTimeout time.Duration, requestTimeout time.Duration, requestTimeoutHandlers ...routing.Handler) *Engine {
	if _, ok := e.Servers[name]; ok {
		return e
	}
	router := routing.New(e.Context)
	router.Timeout(requestTimeout, requestTimeoutHandlers...)
	server := &Server{
		systemLogWriter:     e.systemLogWriter,
		Port:                port,
		gracefulStopTimeout: gracefulStopTimeout,
		Router: &ServerRouter{
			router,
			e.Module,
			nil,
		},
		mutex: sync.RWMutex{},
	}
	e.Servers[name] = server
	e.SystemLog(fmt.Sprintf("ltick: new server [name:'%s', port:'%d', gracefulStopTimeout:'%.fs', requestTimeout:'%.fs']", name, port, gracefulStopTimeout.Seconds(), requestTimeout.Seconds()))
	return e
}

func (e *Engine) GetServer(name string) *Server {
	return e.Servers[name]
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

func (r *ServerRouter) AddRouteGroup(groupName string, handlers ...routing.Handler) *ServerRouteGroup {
	g := &ServerRouteGroup{
		r.Router.Group(groupName, handlers, nil),
		r,
		nil,
	}
	for _, sortedModule := range r.Module.GetSortedModules() {
		module, ok := sortedModule.(module.ModuleInterface)
		if !ok {
			continue
		}
		g.AppendAnteriorHandler(module.OnRequestStartup)
		g.PrependPosteriorHandler(module.OnRequestShutdown)
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
	default:
		g.To(method, path, handlers...)
	}
}
