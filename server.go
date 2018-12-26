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

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/api"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/ltick/tick-routing/content"
	"github.com/ltick/tick-routing/cors"
	"github.com/ltick/tick-routing/fault"
	"github.com/ltick/tick-routing/file"
	"github.com/ltick/tick-routing/slash"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	defaultServerPort                        uint          = 80
	defaultServerLogWriter                   io.Writer     = os.Stdout
	defaultServerGracefulStopTimeoutDuration time.Duration = 120 * time.Second
)

type (
	ServerOptions struct {
		logWriter                              io.Writer
		Port                                   uint
		GracefulStopTimeout                    string
		GracefulStopTimeoutDuration            time.Duration
		MetricsHttpServerRequestsDurations     []prometheus.ObserverVec
		MetricsHttpServerRequestsResponseSizes []prometheus.ObserverVec
		MetricsHttpServerRequestsRequestSizes  []prometheus.ObserverVec
	}
	ServerBasicAuth struct {
		Username string
		Password string
	}

	ServerOption func(*ServerOptions)

	Server struct {
		*ServerOptions
		Router      *ServerRouter
		RouteGroups map[string]*ServerRouteGroup
		mutex       sync.RWMutex
	}
	ServerRouterProxy struct {
		Host     []string
		Group    string
		Path     string
		Upstream string
	}
	ServerRouterMetrics struct {
		Host      []string
		Group     string
		BasicAuth *ServerBasicAuth
	}
	ServerRouterPprof struct {
		Host      []string
		Group     string
		BasicAuth *ServerBasicAuth
	}
	ServerRouterRoute struct {
		Host      []string
		Group     string
		Method    []string
		Path      string
		BasicAuth *ServerBasicAuth
		Handlers  []api.Handler
	}
	routeHandler struct {
		Host      []string
		BasicAuth *ServerBasicAuth
		Handler   api.Handler
	}

	ServerRouterOptions struct {
		Timeout            string
		TimeoutHandlers    []routing.Handler
		Callbacks          []RouterCallback
		AccessLogFunc      access.LogWriterFunc
		ErrorLogFunc       fault.LogFunc
		ErrorHandler       fault.ConvertErrorFunc
		RecoveryLogFunc    fault.LogFunc
		RecoveryHandler    fault.ConvertErrorFunc
		PanicHandler       fault.LogFunc
		TypeNegotiator     []string
		SlashRemover       *int
		LanguageNegotiator []string
		Cors               *cors.Options
		RouteProviders     map[string]interface{}
	}

	ServerRouterOption func(*ServerRouterOptions)

	ServerRouter struct {
		*routing.Router
		Options     *ServerRouterOptions
		Middlewares []MiddlewareInterface
		Metrics     *ServerRouterMetrics
		Pprof       *ServerRouterPprof
		Proxys      []*ServerRouterProxy
		Routes      []*ServerRouterRoute
	}
	ServerRouteGroup struct {
		*routing.RouteGroup
	}
	RouterCallback interface {
		OnRequestStartup(*routing.Context) error
		OnRequestShutdown(*routing.Context) error
	}
)

func ServerPort(port uint) ServerOption {
	return func(options *ServerOptions) {
		options.Port = port
	}
}
func ServerMetricsHttpServerRequestsDuration(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ServerOption {
	if histogram == nil {
		histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_server_requests_duration_seconds",
			Help:    "A histogram of request latencies for requests.",
			Buckets: prometheus.DefBuckets,
		},
			[]string{"server_addr", "host", "method", "path", "status"},
		)
	}
	if summary == nil {
		summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_server_requests_duration_seconds_summary",
				Help:       "A summary of request latencies for requests.",
				Objectives: map[float64]float64{0.9: 0.001, 0.95: 0.001, 0.99: 0.001, 0.999: 0.001, 0.9999: 0.001},
			},
			[]string{"server_addr", "host", "method", "path", "status"},
		)
	}
	return func(options *ServerOptions) {
		options.MetricsHttpServerRequestsDurations = []prometheus.ObserverVec{histogram, summary}
		prometheus.MustRegister(histogram, summary)
	}
}
func ServerMetricsHttpServerRequestsResponseSize(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ServerOption {
	if histogram == nil {
		histogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_requests_response_size_bytes",
				Help:    "A histogram of response size for requests.",
				Buckets: []float64{200, 500, 900, 1500},
			},
			[]string{"server_addr", "host", "method", "path", "status"},
		)
	}
	if summary == nil {
		summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_server_requests_response_size_bytes_summary",
				Help:       "A summary of response size for requests.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"server_addr", "host", "method", "path", "status"},
		)
	}
	return func(options *ServerOptions) {
		options.MetricsHttpServerRequestsResponseSizes = []prometheus.ObserverVec{histogram, summary}
		prometheus.MustRegister(histogram, summary)
	}
}
func ServerMetricsHttpServerRequestsRequestSize(histogram *prometheus.HistogramVec, summary *prometheus.SummaryVec) ServerOption {
	if histogram == nil {
		histogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_server_requests_request_size_bytes",
				Help:    "A histogram of request size for requests.",
				Buckets: []float64{200, 500, 900, 1500},
			},
			[]string{"server_addr", "host", "method", "path", "status"},
		)
	}
	if summary == nil {
		summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       "http_server_requests_request_size_bytes_summary",
				Help:       "A summary of request size for requests.",
				Objectives: map[float64]float64{0.9: 0, 0.95: 0, 0.99: 0, 0.999: 0, 0.9999: 0},
			},
			[]string{"server_addr", "host", "method", "path", "status"},
		)
	}
	return func(options *ServerOptions) {
		options.MetricsHttpServerRequestsRequestSizes = []prometheus.ObserverVec{histogram, summary}
		prometheus.MustRegister(histogram, summary)
	}
}
func ServerLogWriter(logWriter io.Writer) ServerOption {
	return func(options *ServerOptions) {
		options.logWriter = logWriter
	}
}
func ServerGracefulStopTimeout(gracefulStopTimeout string) ServerOption {
	return func(options *ServerOptions) {
		options.GracefulStopTimeout = gracefulStopTimeout
	}
}
func ServerGracefulStopTimeoutDuration(gracefulStopTimeoutDuration time.Duration) ServerOption {
	return func(options *ServerOptions) {
		options.GracefulStopTimeoutDuration = gracefulStopTimeoutDuration
	}
}
func ServerRouterTimeoutHandlers(timeoutHandlers []routing.Handler) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.TimeoutHandlers = timeoutHandlers
	}
}
func ServerRouterTimeout(timeout string) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.Timeout = timeout
	}
}

func ServerRouterCallback(callbacks []RouterCallback) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.Callbacks = callbacks
	}
}
func ServerRouterAccessLogFunc(accessLogFunc access.LogWriterFunc) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.AccessLogFunc = accessLogFunc
	}
}
func ServerRouterErrorLogFunc(faultLogFunc fault.LogFunc) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.ErrorLogFunc = faultLogFunc
	}
}
func ServerRouterErrorHandler(errorHandler fault.ConvertErrorFunc) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.ErrorHandler = errorHandler
	}
}
func ServerRouterPanicHandler(panicLogFunc fault.LogFunc) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.PanicHandler = panicLogFunc
	}
}
func ServerRouterRecoveryLogFunc(faultLogFunc fault.LogFunc) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.RecoveryLogFunc = faultLogFunc
	}
}
func ServerRouterRecoveryHandler(errorHandler fault.ConvertErrorFunc) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.RecoveryHandler = errorHandler
	}
}
func ServerRouterTypeNegotiator(typeNegotiator ...string) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.TypeNegotiator = typeNegotiator
	}
}
func ServerRouterSlashRemover(slashRemover *int) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.SlashRemover = slashRemover
	}
}
func ServerRouterLanguageNegotiator(languageNegotiator ...string) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.LanguageNegotiator = languageNegotiator
	}
}
func ServerRouterCors(cors *cors.Options) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		if cors != nil {
			options.Cors = cors
		}
	}
}
func ServerRouterRouteProviders(routeProviders map[string]interface{}) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.RouteProviders = routeProviders
	}
}
func (e *Engine) NewServerRouter(setters ...ServerRouterOption) (router *ServerRouter) {
	serverRouterOptions := &ServerRouterOptions{}
	for _, setter := range setters {
		setter(serverRouterOptions)
	}
	router = &ServerRouter{
		Router:  routing.New(e.Context),
		Options: serverRouterOptions,
		Routes:  make([]*ServerRouterRoute, 0),
		Proxys:  make([]*ServerRouterProxy, 0),
	}
	return
}

func (r *ServerRouter) Resolve() {
	if r.Options.Timeout != "" {
		handlerTimeoutDuration, err := time.ParseDuration(r.Options.Timeout)
		if err == nil {
			r.TimeoutDuration = handlerTimeoutDuration
		}
	}
	r.Timeout(r.TimeoutDuration, r.TimeoutHandlers...)
	if r.Options.Callbacks != nil {
		for _, c := range r.Options.Callbacks {
			r.AddCallback(c)
		}
	}
	if r.Options.AccessLogFunc != nil {
		r.WithAccessLogger(r.Options.AccessLogFunc)
	} else {
		r.WithAccessLogger(DefaultAccessLogFunc)
	}
	if r.Options.ErrorLogFunc != nil {
		if r.Options.ErrorHandler != nil {
			r.WithErrorHandler(r.Options.ErrorLogFunc, r.Options.ErrorHandler)
		} else {
			r.WithErrorHandler(r.Options.ErrorLogFunc)
		}
	} else if r.Options.ErrorHandler != nil {
		r.WithErrorHandler(DefaultErrorLogFunc(), r.Options.ErrorHandler)
	}
	if r.Options.PanicHandler != nil {
		r.WithPanicHandler(r.Options.PanicHandler)
	} else {
		r.WithPanicHandler(DefaultErrorLogFunc())
	}
	if r.Options.RecoveryLogFunc != nil {
		if r.Options.RecoveryHandler != nil {
			r.WithRecoveryHandler(r.Options.RecoveryLogFunc, r.Options.RecoveryHandler)
		} else {
			r.WithRecoveryHandler(r.Options.RecoveryLogFunc)
		}
	} else if r.Options.RecoveryHandler != nil {
		r.WithRecoveryHandler(DefaultErrorLogFunc(), r.Options.RecoveryHandler)
	}
	if r.Options.TypeNegotiator != nil {
		r.WithTypeNegotiator(r.Options.TypeNegotiator...)
	} else {
		r.WithTypeNegotiator(JSON, XML, XML2, HTML)
	}
	if r.Options.SlashRemover != nil {
		r.WithSlashRemover(*r.Options.SlashRemover)
	} else {
		r.WithSlashRemover(http.StatusMovedPermanently)
	}
	if r.Options.LanguageNegotiator != nil {
		r.WithLanguageNegotiator(r.Options.LanguageNegotiator...)
	} else {
		r.WithLanguageNegotiator("en-US")
	}
	if r.Options.Cors != nil {
		r.WithCors(*r.Options.Cors)
	}
}

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
		if name != "group" && name != "path" && name != "fragment" && name != "query" {
			captures[name] = c.Param(name)
		}
	}
	captures["group"] = sp.Group
	path := c.Request.URL.Path
	if path == "" {
		path = c.Request.URL.RawPath
	}
	captures["fragment"] = c.Request.URL.Fragment
	captures["query"] = c.Request.URL.RawQuery
	captures["path"] = strings.TrimLeft(path, sp.Group)
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

func (e *Engine) RegisterServer(name string, server *Server) {
	if e.ServerMap == nil {
		e.ServerMap = make(map[string]*Server, 0)
	}
	if _, ok := e.ServerMap[name]; ok {
		fmt.Printf(errRegisterServer+": server '%s' already exists\r\n", name)
		os.Exit(1)
	}
	e.ServerMap[name] = server
	// configure
	err := e.ConfigureServerFromFile(server, e.GetConfigCachedFileName(), server.Router.Options.RouteProviders, "servers."+name)
	if err != nil {
		err = errors.Annotate(err, errStartup)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
}

func (e *Engine) SetServerReuqestSlashRemover(name string, status int) *Engine {
	server := e.GetServer(name)
	if server != nil {
		server.SetReuqestSlashRemover(status)
	}
	return e
}
func (e *Engine) SetServerReuqestCors(name string, corsOptions *cors.Options) *Engine {
	server := e.GetServer(name)
	if server != nil {
		server.SetServerReuqestCors(corsOptions)
	}
	return e
}
func (e *Engine) GetServer(name string) *Server {
	if e.ServerMap != nil {
		if _, ok := e.ServerMap[name]; ok {
			return e.ServerMap[name]
		}
	}
	return nil
}
func (e *Engine) GetServerMap() map[string]*Server {
	return e.ServerMap
}

/********** Server **********/
func (s *Server) Resolve() {
	if s.GracefulStopTimeout != "" {
		gracefulStopTimeoutDuration, err := time.ParseDuration(s.GracefulStopTimeout)
		if err == nil {
			s.GracefulStopTimeoutDuration = gracefulStopTimeoutDuration
		}
	}
}
func (s *Server) Get(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"GET"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Post(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"POST"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Put(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"PUT"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Patch(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"PATCH"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Delete(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"DELETE"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Connect(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"CONNECT"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Options(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"OPTIONS"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Trace(host []string, group string, path string, handlers ...api.Handler) *Server {
	s.Router.Routes = append(s.Router.Routes, &ServerRouterRoute{
		Method:   []string{"TRACE"},
		Host:     host,
		Group:    group,
		Path:     path,
		Handlers: handlers,
	})
	return s
}
func (s *Server) Proxy(host []string, group string, path string, upstream string) *Server {
	s.Router.Proxys = append(s.Router.Proxys, &ServerRouterProxy{
		Host:     host,
		Group:    group,
		Path:     path,
		Upstream: upstream,
	})
	return s
}
func (s *Server) Pprof(host []string, group string, basicAuth *ServerBasicAuth) *Server {
	s.Router.Pprof = &ServerRouterPprof{
		Host:      host,
		Group:     group,
		BasicAuth: basicAuth,
	}
	return s
}
func (s *Server) Metrics(host []string, group string, basicAuth *ServerBasicAuth) *Server {
	s.Router.Metrics = &ServerRouterMetrics{
		Host:      host,
		Group:     group,
		BasicAuth: basicAuth,
	}
	return s
}

type metricsHandler struct {
	basicAuth *ServerBasicAuth
}

func (h metricsHandler) Serve(ctx *api.Context) error {
	if h.basicAuth != nil {
		ctx.Request.SetBasicAuth(h.basicAuth.Username, h.basicAuth.Username)
	}
	promhttp.Handler().ServeHTTP(ctx.ResponseWriter, ctx.Request)
	return nil
}

type pprofHandler struct {
	httpHandlerFunc http.HandlerFunc
	basicAuth       *ServerBasicAuth
}

func (h pprofHandler) Serve(ctx *api.Context) error {
	if h.basicAuth != nil {
		ctx.Request.SetBasicAuth(h.basicAuth.Username, h.basicAuth.Username)
	}
	h.httpHandlerFunc(ctx.ResponseWriter, ctx.Request)
	return nil
}

func (s *Server) SetReuqestSlashRemover(status int) *Server {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound:
		s.Router.WithSlashRemover(status)
	}
	return s
}
func (s *Server) SetServerReuqestCors(corsOptions *cors.Options) *Server {
	if corsOptions != nil {
		s.Router.WithCors(*corsOptions)
	} else {
		s.Router.WithCors(CorsAllowAll)
	}
	return s
}
func (s *Server) AddRouteGroup(group string) *ServerRouteGroup {
	// Router Handlers (Global)
	s.RouteGroups[group] = s.Router.AddRouteGroup(group)
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
func (s *Server) Log(args ...interface{}) {
	fmt.Fprintln(s.logWriter, args...)
}
func (s *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	s.Router.ServeHTTP(res, req)
}

func (r *ServerRouter) AddCallback(callback RouterCallback) *ServerRouter {
	if callback != nil {
		r.PrependStartupHandler(callback.OnRequestStartup)
		r.AppendShutdownHandler(callback.OnRequestShutdown)
	}
	return r
}

// Middlewares
func (r *ServerRouter) WithMiddlewares(middlewares []MiddlewareInterface) *ServerRouter {
	r.Middlewares = middlewares
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

func (r *ServerRouter) WithPanicHandler(logf fault.LogFunc) *ServerRouter {
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

func (r *ServerRouter) AddRouteGroup(groupName string) *ServerRouteGroup {
	g := &ServerRouteGroup{
		RouteGroup: r.Group(groupName),
	}
	for _, m := range r.Middlewares {
		g.AppendAnteriorHandler(m.OnRequestStartup)
		g.PrependPosteriorHandler(m.OnRequestShutdown)
	}
	return g
}

func (g *ServerRouteGroup) AddCallback(callback RouterCallback) *ServerRouteGroup {
	if callback != nil {
		g.PrependStartupHandler(callback.OnRequestStartup)
		g.AppendShutdownHandler(callback.OnRequestShutdown)
	}
	return g
}

// 添加API路由
// 可进行参数校验
func (g *ServerRouteGroup) AddApiRoute(method string, path string, handlerRoutes []*routeHandler) {
	routeHandlers := make([]routing.Handler, len(handlerRoutes))
	routeCnt := len(handlerRoutes)
	for index, handlerRoute := range handlerRoutes {
		route := handlerRoute
		routeHandlers[index] = func(ctx *routing.Context) error {
			select {
			case <-ctx.Context.Done():
				ctx.Abort()
				switch ctx.Context.Err() {
				case context.DeadlineExceeded:
					return routing.NewHTTPError(http.StatusRequestTimeout, http.StatusText(http.StatusRequestTimeout))
				case context.Canceled:
					return routing.NewHTTPError(http.StatusNoContent, http.StatusText(http.StatusNoContent))
				}
				return routing.NewHTTPError(http.StatusNoContent, http.StatusText(http.StatusNoContent))
			default:
				requestHost := ctx.Request.Host
				if requestHost == "" {
					requestHost = ctx.Request.URL.Host
				}
				for _, host := range route.Host {
					if utility.WildcardMatch(host, requestHost) {
						// Jump After NotFoundHandler
						ctx.Jump(routeCnt - index + 3)
						if route.BasicAuth != nil {
							ctx.Request.SetBasicAuth(route.BasicAuth.Username, route.BasicAuth.Password)
						}
						apiCtx := &api.Context{
							Context:  ctx,
							Response: api.NewResponse(ctx.ResponseWriter),
						}
						return route.Handler.Serve(apiCtx)
					}
				}
				return nil
			}
			return nil
		}
	}
	// TODO custom NotFoundHandler
	routeHandlers = combineHandlers(routeHandlers, []routing.Handler{routing.NotFoundHandler})
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
