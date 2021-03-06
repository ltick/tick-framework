package ltick

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/api"
	"github.com/ltick/tick-framework/metrics"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-framework/utility/datatypes"
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

type (
	ServerOptions struct {
		logWriter io.Writer
		Port      uint
		// ReadTimeout is the maximum duration for reading the entire
		// request, including the body.
		//
		// Because ReadTimeout does not let Handlers make per-request
		// decisions on each request body's acceptable deadline or
		// upload rate, most users will prefer to use
		// ReadHeaderTimeout. It is valid to use them both.
		ReadTimeout         string
		ReadTimeoutDuration time.Duration
		// ReadHeaderTimeout is the amount of time allowed to read
		// request headers. The connection's read deadline is reset
		// after reading the headers and the Handler can decide what
		// is considered too slow for the body.
		ReadHeaderTimeout         string
		ReadHeaderTimeoutDuration time.Duration
		// WriteTimeout is the maximum duration before timing out
		// writes of the response. It is reset whenever a new
		// request's header is read. Like ReadTimeout, it does not
		// let Handlers make decisions on a per-request basis.
		WriteTimeout         string
		WriteTimeoutDuration time.Duration
		// IdleTimeout is the maximum amount of time to wait for the
		// next request when keep-alives are enabled. If IdleTimeout
		// is zero, the value of ReadTimeout is used. If both are
		// zero, ReadHeaderTimeout is used.
		IdleTimeout                            string
		IdleTimeoutDuration                    time.Duration
		GracefulStopTimeout                    string
		GracefulStopTimeoutDuration            time.Duration
		MetricsHttpServerRequests              []prometheus.ObserverVec
		MetricsHttpServerRequestsTrace         []prometheus.ObserverVec
		MetricsHttpServerRequestsResponseSizes []prometheus.ObserverVec
		MetricsHttpServerRequestsRequestSizes  []prometheus.ObserverVec
		MetricsHttpServerRequestLabelFuncs      []metrics.HttpServerRequestLabelFunc
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
		RequestTimeout         datatypes.Duration
		RequestTimeoutHandlers []routing.Handler
		Callbacks              []RouterCallback
		AccessLogFunc          access.LogWriterFunc
		ErrorLogFunc           fault.LogFunc
		ErrorHandler           fault.ConvertErrorFunc
		RecoveryLogFunc        fault.LogFunc
		RecoveryHandler        fault.ConvertErrorFunc
		PanicHandler           fault.LogFunc
		TypeNegotiator         []string
		SlashRemover           *int
		LanguageNegotiator     []string
		Cors                   *cors.Options
		RouteProviders         map[string]interface{}
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
func ServerMetricsHttpServerRequests(observers []prometheus.ObserverVec) ServerOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpServerRequests}
	}
	return func(options *ServerOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpServerRequests = observers
	}
}
func ServerMetricsHttpServerRequestsResponseSize(observers []prometheus.ObserverVec) ServerOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpServerRequestsResponseSize}
	}
	return func(options *ServerOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpServerRequestsResponseSizes = observers
	}
}
func ServerMetricsHttpServerRequestsRequestSize(observers []prometheus.ObserverVec) ServerOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpServerRequestsRequestSize}
	}
	return func(options *ServerOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpServerRequestsRequestSizes = observers
	}
}
func ServerMetricsHttpServerRequestsTrace(observers []prometheus.ObserverVec) ServerOption {
	if observers == nil {
		observers = []prometheus.ObserverVec{defaultMetricsHttpServerRequestsTrace}
	}
	return func(options *ServerOptions) {
		for _, observer := range observers {
			prometheus.Register(observer)
		}
		options.MetricsHttpServerRequestsTrace = observers
	}
}
func ServerMetricsHttpServerRequestLabelFunc(httpServerRequestLabelFuncs ...metrics.HttpServerRequestLabelFunc) ServerOption {
	return func(options *ServerOptions) {
		options.MetricsHttpServerRequestLabelFuncs = httpServerRequestLabelFuncs
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
func ServerReadTimeout(readTimeout string) ServerOption {
	return func(options *ServerOptions) {
		options.ReadTimeout = readTimeout
	}
}
func ServerReadTimeoutDuration(readTimeoutDuration time.Duration) ServerOption {
	return func(options *ServerOptions) {
		options.ReadTimeoutDuration = readTimeoutDuration
	}
}
func ServerReadHeaderTimeout(readHeaderTimeout string) ServerOption {
	return func(options *ServerOptions) {
		options.ReadHeaderTimeout = readHeaderTimeout
	}
}
func ServerReadHeaderTimeoutDuration(readHeaderTimeoutDuration time.Duration) ServerOption {
	return func(options *ServerOptions) {
		options.ReadHeaderTimeoutDuration = readHeaderTimeoutDuration
	}
}
func ServerWriteTimeout(writeTimeout string) ServerOption {
	return func(options *ServerOptions) {
		options.WriteTimeout = writeTimeout
	}
}
func ServerWriteTimeoutDuration(writeTimeoutDuration time.Duration) ServerOption {
	return func(options *ServerOptions) {
		options.WriteTimeoutDuration = writeTimeoutDuration
	}
}
func ServerRouterRequestTimeoutHandlers(requestTimeoutHandlers []routing.Handler) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		options.RequestTimeoutHandlers = requestTimeoutHandlers
	}
}
func ServerRouterRequestTimeout(requestTimeout string) ServerRouterOption {
	return func(options *ServerRouterOptions) {
		if timeout, err := time.ParseDuration(requestTimeout); err == nil {
			options.RequestTimeout = datatypes.NewDuration(timeout)
		}
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
		Router:  routing.New(),
		Options: serverRouterOptions,
		Routes:  make([]*ServerRouterRoute, 0),
		Proxys:  make([]*ServerRouterProxy, 0),
	}
	return
}

func (r *ServerRouter) Resolve() {
	// AccessLog
	if r.Options.AccessLogFunc != nil {
		r.WithAccessLogger(r.Options.AccessLogFunc)
	} else {
		r.WithAccessLogger(DefaultAccessLogFunc)
	}
	// Error
	if r.Options.ErrorLogFunc != nil {
		if r.Options.ErrorHandler != nil {
			r.WithErrorHandler(r.Options.ErrorLogFunc, r.Options.ErrorHandler)
		} else {
			r.WithErrorHandler(r.Options.ErrorLogFunc)
		}
	} else if r.Options.ErrorHandler != nil {
		r.WithErrorHandler(DefaultErrorLogFunc(), r.Options.ErrorHandler)
	}
	// Panic
	if r.Options.PanicHandler != nil {
		r.WithPanicHandler(r.Options.PanicHandler)
	} else {
		r.WithPanicHandler(DefaultErrorLogFunc())
	}
	// Recovery
	if r.Options.RecoveryLogFunc != nil {
		if r.Options.RecoveryHandler != nil {
			r.WithRecoveryHandler(r.Options.RecoveryLogFunc, r.Options.RecoveryHandler)
		} else {
			r.WithRecoveryHandler(r.Options.RecoveryLogFunc)
		}
	} else if r.Options.RecoveryHandler != nil {
		r.WithRecoveryHandler(DefaultErrorLogFunc(), r.Options.RecoveryHandler)
	}
	// Type
	if r.Options.TypeNegotiator != nil {
		r.WithTypeNegotiator(r.Options.TypeNegotiator...)
	} else {
		r.WithTypeNegotiator(JSON, XML, XML2, HTML)
	}
	// Slash
	if r.Options.SlashRemover != nil {
		r.WithSlashRemover(*r.Options.SlashRemover)
	}
	// Language
	if r.Options.LanguageNegotiator != nil {
		r.WithLanguageNegotiator(r.Options.LanguageNegotiator...)
	}
	// CORS
	if r.Options.Cors != nil {
		r.WithCors(*r.Options.Cors)
	}
	// Timeout
	if r.Options.RequestTimeout.Duration > 0 {
		r.WithTimeout(r.Options.RequestTimeout.Duration)
	}
	// Callbacks
	for _, c := range r.Options.Callbacks {
		r.AddCallback(c)
	}
	// Middlewares
	for _, m := range r.Middlewares {
		r.AddMiddlewares(m)
	}
}
func (sp *ServerRouterProxy) Serve(c *api.Context) (error) {
	captures := make(map[string]string)
	r := regexp.MustCompile("<:(\\w+)>")
	match := r.FindStringSubmatch(sp.Upstream)
	if match == nil {
		return errors.New("ltick: upstream not match")
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
		upstreamURL, err := url.Parse(upstream)
		if err != nil {
			return err
		}
		if upstreamURL != nil {
			director := func(req *http.Request) {
				req.URL.Scheme = upstreamURL.Scheme
				req.URL.Host = upstreamURL.Host
				req.Host = upstreamURL.Host
				req.RequestURI = upstreamURL.RequestURI()
			}
			proxy := &httputil.ReverseProxy{Director: director}
			proxy.ServeHTTP(c.ResponseWriter, c.Request)
			c.Context.Abort()
		}
	}
	return nil
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
	err := e.ConfigureServerFromFile(server, e.GetConfigCachedFile(), server.Router.Options.RouteProviders, "servers."+name)
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
	if s.IdleTimeout != "" {
		idleTimeoutDuration, err := time.ParseDuration(s.IdleTimeout)
		if err == nil {
			s.IdleTimeoutDuration = idleTimeoutDuration
		}
	}
	if s.ReadTimeout != "" {
		readTimeoutDuration, err := time.ParseDuration(s.ReadTimeout)
		if err == nil {
			s.ReadTimeoutDuration = readTimeoutDuration
		}
	}
	if s.ReadHeaderTimeout != "" {
		readHeaderTimeoutDuration, err := time.ParseDuration(s.ReadHeaderTimeout)
		if err == nil {
			s.ReadHeaderTimeoutDuration = readHeaderTimeoutDuration
		}
	}
	if s.WriteTimeout != "" {
		writeTimeoutDuration, err := time.ParseDuration(s.WriteTimeout)
		if err == nil {
			s.WriteTimeoutDuration = writeTimeoutDuration
		}
	}
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
func (s *Server) Pprof(host []string, basicAuth *ServerBasicAuth) *Server {
	s.Router.Pprof = &ServerRouterPprof{
		Host:      host,
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
}

func (h metricsHandler) Serve(ctx *api.Context) error {
	promhttp.Handler().ServeHTTP(ctx.ResponseWriter, ctx.Request)
	return nil
}

type pprofHandlerFunc struct {
	httpHandlerFunc http.HandlerFunc
}

func (h pprofHandlerFunc) Serve(ctx *api.Context) error {
	ctx.ResponseWriter.Header().Set("Content-Type", "text/html")
	h.httpHandlerFunc(ctx.ResponseWriter, ctx.Request)
	return nil
}

type pprofHandler struct {
	httpHandler http.Handler
	basicAuth   *ServerBasicAuth
}

func (h pprofHandler) Serve(ctx *api.Context) error {
	if h.basicAuth != nil {
		ctx.Request.SetBasicAuth(h.basicAuth.Username, h.basicAuth.Username)
	}
	ctx.ResponseWriter.Header().Set("Content-Type", "text/html")
	h.httpHandler.ServeHTTP(ctx.ResponseWriter, ctx.Request)
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
	r.AppendStartupHandler(func(c *routing.Context) (err error) {
		callback.OnRequestStartup(c)
		err = c.Next()
		callback.OnRequestShutdown(c)
		return
	})
	return r
}

// Middlewares
func (r *ServerRouter) WithMiddlewares(middlewares []MiddlewareInterface) *ServerRouter {
	r.Middlewares = middlewares
	return r
}

func (r *ServerRouter) AddMiddlewares(middleware RouterCallback) *ServerRouter {
	r.AppendAnteriorHandler(func(c *routing.Context) (err error) {
		if err = middleware.OnRequestStartup(c); err != nil {
			c.Abort()
			goto end
		}
		err = c.Next()
	end:
		middleware.OnRequestShutdown(c)
		return
	})
	return r
}

func (r *ServerRouter) WithTimeout(timeout time.Duration) *ServerRouter {
	r.AppendStartupHandler(fault.TimeoutHandler(timeout))
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
	r.AppendStartupHandler(CustomLogger(loggerFunc))
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

// 添加API路由
// 可进行参数校验
func (g *ServerRouteGroup) AddApiRoute(method string, path string, handlerRoutes []*routeHandler) {
	routeHandler := func(ctx *routing.Context) error {
		requestHost := ctx.Request.Host
		if requestHost == "" {
			requestHost = ctx.Request.URL.Host
		}
		for _, route := range handlerRoutes {
			for _, host := range route.Host {
				if utility.WildcardMatch(host, requestHost) {
					if route.BasicAuth != nil {
						ctx.Request.SetBasicAuth(route.BasicAuth.Username, route.BasicAuth.Password)
					}
					ctx.Context = utility.MergeContext(ctx.Request.Context(), ctx.Context)
					apiCtx := &api.Context{
						Context:  ctx,
						Response: api.NewResponse(ctx),
					}
					return route.Handler.Serve(apiCtx)
				}
			}
		}
		// TODO custom NotFoundHandler
		return routing.NotFoundHandler(ctx)
	}
	g.AddRoute(method, path, routeHandler)
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
