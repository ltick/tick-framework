package ltick

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/logger"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-graceful"
	libLog "github.com/ltick/tick-log"
	"github.com/ltick/tick-routing"
)

var (
	errNew                       = "ltick: new error"
	errEngineOption              = "ltick: set engine option error"
	errEngineConfigOption        = "ltick: set engine config option error"
	errNewDefault                = "ltick: new classic error"
	errNewServer                 = "ltick: new server error"
	errGetLogger                 = "ltick: get logger error"
	errWithValues                = "ltick: with values set value of key '%s' error"
	errConfigureServer           = "ltick: configure server error"
	errStartupCallback           = "ltick: startup callback error"
	errStartupInjectComponent    = "ltick: startup inject component error"
	errStartupComponentInitiate  = "ltick: startup component '%s' initiate error"
	errStartupComponentStartup   = "ltick: startup component '%s' startup error"
	errShutdownCallback          = "ltick: shutdown callback error"
	errShutdownComponentShutdown = "ltick: shutdown component '%s' shutdown error"
	errLoadCachedConfig          = "ltick: load cached config error"
	errLoadConfig                = "ltick: load config error [path:'%s', name:'%s']"
	errLoadEnv                   = "ltick: load env error [env_prefix:'%s', binded_environment_keys:'%v']"
	errLoadSystemConfig          = "ltick: load system config error"
	errLoadEnvFile               = "ltick: load env file error"
	errGetCacheFile              = "ltick: get cache file error"
)

type State int8

const (
	STATE_INITIATE State = iota
	STATE_STARTUP
	STATE_SHUTDOWN
)

type (
	EngineOptions struct {
		callback  Callback
		logWriter io.Writer
	}

	EngineOption func(*EngineOptions)

	EngineConfigOptions struct {
		configFile      string
		configCacheFile string
		dotenvFile      string
		envPrefix       string
		configs         map[string]config.Option
	}

	EngineConfigOption func(*EngineConfigOptions)

	Engine struct {
		*EngineOptions
		*EngineConfigOptions
		state       State
		executeFile string

		configer  *config.Config
		Registry  *Registry
		Context   context.Context
		ServerMap map[string]*Server
	}
	Callback interface {
		OnStartup(*Engine) error  // Execute On After All Engine Component OnStartup
		OnShutdown(*Engine) error // Execute On After All Engine Component OnShutdown
	}
)

var defaultConfigs map[string]config.Option = map[string]config.Option{
	"APP_ENV":     config.Option{Type: config.String, Default: "local", EnvironmentKey: "APP_ENV"},
	"PREFIX_PATH": config.Option{Type: config.String, EnvironmentKey: "PREFIX_PATH"},
	"TMP_PATH":    config.Option{Type: config.String, Default: "/tmp", EnvironmentKey: "TMP_PATH"},
	"DEBUG":       config.Option{Type: config.String, Default: false},

	"ACCESS_LOG_TYPE":      config.Option{Type: config.String, Default: "console", EnvironmentKey: "ACCESS_LOG_TYPE"},
	"ACCESS_LOG_FILENAME":  config.Option{Type: config.String, Default: "/tmp/access.log", EnvironmentKey: "ACCESS_LOG_FILENAME"},
	"ACCESS_LOG_WRITER":    config.Option{Type: config.String, Default: "discard", EnvironmentKey: "ACCESS_LOG_WRITER"},
	"ACCESS_LOG_MAX_LEVEL": config.Option{Type: config.String, Default: log.LevelInfo, EnvironmentKey: "ACCESS_LOG_MAX_LEVEL"},
	"ACCESS_LOG_FORMATTER": config.Option{Type: config.String, Default: "raw", EnvironmentKey: "ACCESS_LOG_FORMATTER"},

	"DEBUG_LOG_TYPE":      config.Option{Type: config.String, Default: "console", EnvironmentKey: "DEBUG_LOG_TYPE"},
	"DEBUG_LOG_FILENAME":  config.Option{Type: config.String, Default: "/tmp/debug.log", EnvironmentKey: "DEBUG_LOG_FILENAME"},
	"DEBUG_LOG_WRITER":    config.Option{Type: config.String, Default: "discard", EnvironmentKey: "DEBUG_LOG_WRITER"},
	"DEBUG_LOG_MAX_LEVEL": config.Option{Type: config.String, Default: log.LevelInfo, EnvironmentKey: "DEBUG_LOG_MAX_LEVEL"},
	"DEBUG_LOG_FORMATTER": config.Option{Type: config.String, Default: "default", EnvironmentKey: "DEBUG_LOG_FORMATTER"},

	"SYSTEM_LOG_TYPE":      config.Option{Type: config.String, Default: "console", EnvironmentKey: "SYSTEM_LOG_TYPE"},
	"SYSTEM_LOG_FILENAME":  config.Option{Type: config.String, Default: "/tmp/system.log", EnvironmentKey: "SYSTEM_LOG_FILENAME"},
	"SYSTEM_LOG_WRITER":    config.Option{Type: config.String, Default: "discard", EnvironmentKey: "SYSTEM_LOG_WRITER"},
	"SYSTEM_LOG_MAX_LEVEL": config.Option{Type: config.String, Default: log.LevelInfo, EnvironmentKey: "SYSTEM_LOG_MAX_LEVEL"},
	"SYSTEM_LOG_FORMATTER": config.Option{Type: config.String, Default: "sys", EnvironmentKey: "SYSTEM_LOG_FORMATTER"},
}

var defaultlogWriter io.Writer = os.Stdout

func EngineConfigFile(configFile string) EngineConfigOption {
	return func(options *EngineConfigOptions) {
		configFile, err := filepath.Abs(configFile)
		if err != nil {
			err = errors.Annotatef(err, errEngineConfigOption)
			fmt.Println(errors.ErrorStack(err))
			os.Exit(1)
		}
		options.configFile = configFile
	}
}

func EngineConfigDotenvFile(dotenvFile string) EngineConfigOption {
	return func(options *EngineConfigOptions) {
		dotenvFile, err := filepath.Abs(dotenvFile)
		if err != nil {
			err = errors.Annotatef(err, errEngineConfigOption)
			fmt.Println(errors.ErrorStack(err))
			os.Exit(1)
		}
		options.dotenvFile = dotenvFile
	}
}

func EngineConfigEnvPrefix(envPrefix string) EngineConfigOption {
	return func(options *EngineConfigOptions) {
		options.envPrefix = envPrefix
	}
}

func EngineConfigConfigs(configs map[string]config.Option) EngineConfigOption {
	return func(options *EngineConfigOptions) {
		options.configs = configs
	}
}

func EngineConfigCacheFile(configCacheFile string) EngineConfigOption {
	return func(options *EngineConfigOptions) {
		options.configCacheFile = configCacheFile
	}
}

func EngineCallback(callback Callback) EngineOption {
	return func(options *EngineOptions) {
		options.callback = callback
	}
}

func EngineLogWriter(logWriter io.Writer) EngineOption {
	return func(options *EngineOptions) {
		options.logWriter = logWriter
	}
}

func (e *Engine) GetLogWriter() io.Writer {
	return e.EngineOptions.logWriter
}

func (e *Engine) GetConfigDotenvFile() string {
	return e.EngineConfigOptions.dotenvFile
}

func (e *Engine) GetConfigEnvPrefix() string {
	return e.EngineConfigOptions.envPrefix
}

func (e *Engine) GetConfigCacheFile() string {
	return e.EngineConfigOptions.configCacheFile
}

func (e *Engine) GetConfigConfigs() map[string]config.Option {
	return e.EngineConfigOptions.configs
}

var configPlaceholdRegExp = regexp.MustCompile(`%\w+%`)

func New(registry *Registry, setters ...EngineOption) (e *Engine) {
	var err error
	var ok bool
	defaultConfigFile, err = filepath.Abs(defaultConfigFile)
	if err != nil {
		err = errors.Annotatef(err, errNew)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	defaultDotenvFile, err = filepath.Abs(defaultDotenvFile)
	if err != nil {
		err = errors.Annotatef(err, errNew)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	engineOptions := &EngineOptions{
		logWriter: defaultlogWriter,
	}
	for _, setter := range setters {
		setter(engineOptions)
	}
	e = &Engine{
		EngineOptions: engineOptions,
		state:         STATE_INITIATE,
		Registry:      registry,
		Context:       context.Background(),
	}
	e.executeFile, err = exec.LookPath(os.Args[0])
	if err != nil {
		err = errors.Annotate(err, errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	// configer
	configComponent, err := registry.GetComponentByName("Config")
	if err != nil {
		err = errors.Annotate(err, errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	e.configer, ok = configComponent.(*config.Config)
	if !ok {
		err = errors.Annotate(errors.New("ltick: invalid 'Config' type"), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	// 模块初始化
	componentMap := e.Registry.GetComponentMap()
	for _, name := range e.Registry.GetSortedComponentName() {
		ci, ok := componentMap[name].(ComponentInterface)
		if !ok {
			err = errors.Annotate(errors.Errorf("invalid type"), errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			err = errors.Annotate(err, errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	return e
}

func (e *Engine) ConfigureServerFromFile(s *Server, configFile string, providers map[string]interface{}, configTag string) *Server {
	err := e.configer.ConfigureFileConfig(s, configFile, providers, "server")
	if err != nil {
		err := errors.Annotate(err, errConfigureServer)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	return s
}

func (e *Engine) ConfigureServerFromJson(s *Server, configJson []byte, providers map[string]interface{}, configTag string) *Server {
	err := e.configer.ConfigureJsonConfig(s, configJson, providers, "server")
	if err != nil {
		err := errors.Annotate(err, errConfigureServer)
		fmt.Println(errors.ErrorStack(err))
		os.Exit(1)
	}
	return s
}

func (e *Engine) NewDefaultServer(setters ...ServerOption) *Server {
	middlewares := make([]MiddlewareInterface, 0)
	for _, sortedMiddleware := range e.Registry.GetSortedMiddlewares() {
		middleware, ok := sortedMiddleware.(MiddlewareInterface)
		if !ok {
			continue
		}
		middlewares = append(middlewares, middleware)
	}
	var router *ServerRouter = NewServerRouter(e.Context)
	router.WithAccessLogger(DefaultAccessLogFunc).
		WithErrorHandler(DefaultErrorLogFunc(), DefaultErrorHandler).
		WithPanicLogger(DefaultErrorLogFunc()).
		WithTypeNegotiator(JSON, XML, XML2, HTML).
		WithSlashRemover(http.StatusMovedPermanently).
		WithLanguageNegotiator("zh-CN", "en-US").
		WithCors(CorsAllowAll).
		WithMiddlewares(middlewares)
	setters = append(setters, ServerLogWriter(e.logWriter))
	server := NewServer(router, setters...)
	server.AddRouteGroup("/")
	server.Pprof("*", "debug")
	return server
}

func (e *Engine) LoadConfig(setters ...EngineConfigOption) *Engine {
	var err error
	engineConfigOptions := &EngineConfigOptions{
		dotenvFile: defaultDotenvFile,
		configFile: defaultConfigFile,
		envPrefix:  defaultEnvPrefix,
		configs:    defaultConfigs,
	}
	for _, setter := range setters {
		setter(engineConfigOptions)
	}
	err = e.configer.SetOptions(engineConfigOptions.configs)
	if err != nil {
		err = errors.Annotate(err, errNewDefault)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	// 加载系统配置
	if !path.IsAbs(engineConfigOptions.configFile) {
		err = errors.Annotate(fmt.Errorf("ltick: '%s' is not a valid config path", engineConfigOptions.configFile), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	if !path.IsAbs(engineConfigOptions.dotenvFile) {
		err = errors.Annotate(fmt.Errorf("ltick: '%s' is not a valid dotenv path", engineConfigOptions.dotenvFile), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	if e.configCacheFile == "" {
		fileExtension := filepath.Ext(engineConfigOptions.configCacheFile)
		engineConfigOptions.configCacheFile = strings.Replace(engineConfigOptions.configCacheFile, fileExtension, "", -1) + ".cached" + fileExtension
	}
	configCachedFile, err := e.openCacheConfigFile(engineConfigOptions.configCacheFile)
	if err != nil {
		err = errors.Annotate(err, errLoadSystemConfig)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	defer configCachedFile.Close()
	cachedConfigFileName := configCachedFile.Name()
	if engineConfigOptions.dotenvFile != "" {
		e.LoadEnvFile(engineConfigOptions.envPrefix, engineConfigOptions.dotenvFile)
	} else {
		e.LoadEnv(engineConfigOptions.envPrefix)
	}
	e.loadCachedConfig(engineConfigOptions.configFile, cachedConfigFileName)
	go func() {
		// 刷新缓存
		for {
			cachedConfigFileInfo, err := os.Stat(cachedConfigFileName)
			if err != nil {
				err = errors.Annotate(err, errLoadSystemConfig)
				e.Log(errors.ErrorStack(err))
				return
			}
			if engineConfigOptions.dotenvFile != "" {
				dotenvFileInfo, err := os.Stat(engineConfigOptions.dotenvFile)
				if err != nil {
					err = errors.Annotate(err, errLoadSystemConfig)
					e.Log(errors.ErrorStack(err))
					return
				}
				if cachedConfigFileInfo.ModTime().Before(dotenvFileInfo.ModTime()) {
					e.LoadEnvFile(engineConfigOptions.envPrefix, engineConfigOptions.dotenvFile)
					e.loadCachedConfig(engineConfigOptions.configFile, cachedConfigFileName)
				}
			}
			configFileInfo, err := os.Stat(engineConfigOptions.configFile)
			if err != nil {
				err = errors.Annotate(err, errLoadSystemConfig)
				e.Log(errors.ErrorStack(err))
				return
			}
			if cachedConfigFileInfo.ModTime().Before(configFileInfo.ModTime()) {
				e.loadCachedConfig(engineConfigOptions.configFile, cachedConfigFileName)
			}
			time.Sleep(defaultConfigReloadTime)
		}
	}()
	return e
}

func (e *Engine) openCacheConfigFile(cacheFile string) (file *os.File, err error) {
	_, err = os.Stat(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = utility.NewFile(cacheFile, 0644, bytes.NewReader([]byte{}), 0)
			if err != nil {
				return nil, errors.New(errGetCacheFile + ": " + err.Error())
			}
		} else {
			return nil, errors.New(errGetCacheFile + ": " + err.Error())
		}
	} else {
		file, err = os.OpenFile(cacheFile, os.O_RDWR, 0644)
		if err != nil {
			return nil, errors.New(errGetCacheFile + ": " + err.Error())
		}
	}
	return file, err
}

func (e *Engine) loadCachedConfig(configPath string, cachedConfigFileName string) {
	configFile, err := os.OpenFile(configPath, os.O_RDONLY, 0644)
	if err != nil {
		err = errors.Annotate(err, errLoadCachedConfig)
		e.Log(errors.ErrorStack(err))
		return
	}
	defer configFile.Close()
	cachedFileByte, err := ioutil.ReadAll(configFile)
	if err != nil {
		err = errors.Annotate(err, errLoadCachedConfig)
		e.Log(errors.ErrorStack(err))
		return
	}
	// configer
	matches := configPlaceholdRegExp.FindAll(cachedFileByte, -1)
	for _, match := range matches {
		replaceKey := string(match)
		replaceConfigKey := strings.Trim(replaceKey, "%")
		cachedFileByte = bytes.Replace(cachedFileByte, []byte(replaceKey), []byte(e.configer.GetString(replaceConfigKey)), -1)
	}
	err = ioutil.WriteFile(cachedConfigFileName, cachedFileByte, 0644)
	if err != nil {
		err = errors.Annotate(err, errLoadCachedConfig)
		e.Log(errors.ErrorStack(err))
		return
	}
	e.loadConfig(filepath.Dir(cachedConfigFileName), strings.Replace(filepath.Base(cachedConfigFileName), filepath.Ext(cachedConfigFileName), "", 1))
}
func (e *Engine) loadConfig(configPath string, configName string) *Engine {
	var err error
	if configPath == "" || configName == "" {
		err = errors.Annotatef(errors.Errorf("configPath or configName is empty"), errLoadConfig, configPath, configPath)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	if !strings.HasPrefix(configPath, "/") {
		configPath = strings.TrimRight(configPath, "/") + "/" + configPath
	}
	_, err = os.Stat(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			err := errors.Annotatef(err, errLoadConfig, configPath, configPath)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	// configer
	e.configer.AddConfigPath(configPath)
	err = e.configer.LoadFromConfigPath(configName)
	if err != nil {
		err := errors.Annotatef(err, errLoadConfig, configPath, configPath)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	for _, name := range e.Registry.GetSortedComponentName() {
		err = e.ConfigureComponentFileConfig(name, e.configer.ConfigFileUsed(), make(map[string]interface{}))
		if err != nil {
			err = errors.Annotate(err, errNew)
			e.Log(errors.ErrorStack(err))
		}
	}
	return e
}
func (e *Engine) LoadEnv(envPrefix string) *Engine {
	// configer
	e.configer.SetEnvPrefix(envPrefix)
	err := e.configer.LoadFromEnv()
	if err != nil {
		if !os.IsNotExist(err) {
			err := errors.Annotatef(err, errLoadEnv, envPrefix, e.configer.BindedEnvironmentKeys())
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	return e
}
func (e *Engine) LoadEnvFile(envPrefix string, dotenvFile string) *Engine {
	// configer
	e.configer.SetEnvPrefix(envPrefix)
	err := e.configer.LoadFromEnvFile(dotenvFile)
	if err != nil {
		err := errors.Annotatef(err, errLoadEnvFile)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	return e
}

func (e *Engine) WithCallback(callback Callback) *Engine {
	if callback != nil {
		e.EngineOptions.callback = callback
	}
	return e
}
func (e *Engine) WithValues(values map[string]interface{}) *Engine {
	for key, value := range values {
		err := e.Registry.RegisterValue(key, value)
		if err != nil {
			err := errors.Annotatef(err, errWithValues, key)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	return e
}

func (e *Engine) GetLogger(name string) (*libLog.Logger, error) {
	loggerComponent, err := e.Registry.GetComponentByName("Log")
	if err != nil {
		return nil, errors.Annotate(err, errGetLogger)
	}
	log, ok := loggerComponent.(*log.Logger)
	if !ok {
		return nil, errors.Annotate(errors.Errorf("invalid 'Logger' component type"), errGetLogger)
	}
	logger, err := log.GetLogger(name)
	if err != nil {
		return nil, errors.Annotate(err, errGetLogger)
	}
	return logger, nil
}
func (e *Engine) SetLogWriter(logWriter io.Writer) {
	e.EngineOptions.logWriter = logWriter
}
func (e *Engine) Log(args ...interface{}) {
	fmt.Fprintln(e.logWriter, args...)
}
func (e *Engine) Startup() (err error) {
	if e.state != STATE_INITIATE {
		return nil
	}
	e.Log("ltick: Execute file \"" + e.executeFile + "\"")
	e.Log("ltick: Startup")
	// 中间件初始化
	for _, m := range e.Registry.GetMiddlewareMap() {
		mi, ok := m.(MiddlewareInterface)
		if !ok {
			err = errors.Annotate(errors.Errorf("invalid type"), errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
		e.Context, err = mi.Initiate(e.Context)
		if err != nil {
			err = errors.Annotate(err, errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	if e.EngineOptions.callback != nil {
		err = e.Registry.InjectComponentTo([]interface{}{e.EngineOptions.callback})
		if err != nil {
			return errors.Annotatef(err, errStartupCallback)
		}
		err = e.EngineOptions.callback.OnStartup(e)
		if err != nil {
			return errors.Annotatef(err, errStartupCallback)
		}
	}
	if e.ServerMap != nil {
		for _, server := range e.ServerMap {
			if server.Router.Routes != nil && len(server.Router.Routes) > 0 {
				for _, route := range server.Router.Routes {
					if _, ok := server.RouteGroups[route.Group]; !ok {
						server.RouteGroups[route.Group] = server.AddRouteGroup(route.Group)
					}
					server.RouteGroups[route.Group].AddApiRoute(route.Method, route.Path, route.Handlers...)
				}
			}
			// proxy
			if server.Router.Proxys != nil && len(server.Router.Proxys) > 0 {
				for _, proxy := range server.Router.Proxys {
					if _, ok := server.RouteGroups[proxy.Group]; !ok {
						server.RouteGroups[proxy.Group] = server.AddRouteGroup(proxy.Group)
					}
					server.RouteGroups[proxy.Group].AddRoute("ANY", proxy.Path, func(c *routing.Context) error {
						upstreamURL, err := proxy.Proxy(c)
						e.Log(upstreamURL)
						e.Log(err)
						if err != nil {
							return routing.NewHTTPError(http.StatusInternalServerError, err.Error())
						}
						if upstreamURL != nil {
							e.Log("===")
							e.Log(upstreamURL)
							director := func(req *http.Request) {
								req = c.Request
								req.URL.Scheme = upstreamURL.Scheme
								req.URL.Host = upstreamURL.Host
								req.RequestURI = upstreamURL.RequestURI()
							}
							proxy := &httputil.ReverseProxy{Director: director}
							proxy.ServeHTTP(c.ResponseWriter, c.Request)
							c.Abort()
						}
						return nil
					})
				}
			}
		}
	}
	sortedComponenetName := e.Registry.GetSortedComponentName()
	// 模块初始化
	for index, c := range e.Registry.GetSortedComponents() {
		ci, ok := c.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errStartupComponentInitiate, sortedComponenetName[index])
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			return errors.Annotatef(err, errStartupComponentInitiate, sortedComponenetName[index])
		}
	}
	// 模块启动
	for index, c := range e.Registry.GetSortedComponents() {
		ci, ok := c.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errStartupComponentStartup, sortedComponenetName[index])
		}
		e.Context, err = ci.OnStartup(e.Context)
		if err != nil {
			return errors.Annotatef(err, errStartupComponentStartup, sortedComponenetName[index])
		}
	}
	// 注入模块
	err = e.Registry.InjectComponent()
	if err != nil {
		return errors.Annotatef(err, errStartupInjectComponent)
	}
	e.state = STATE_STARTUP
	return nil
}

func (e *Engine) Shutdown() (err error) {
	if e.state != STATE_STARTUP {
		return nil
	}
	e.Log("ltick: Shutdown")
	sortedComponenetName := e.Registry.GetSortedComponentName()
	for index, c := range e.Registry.GetSortedComponents() {
		component, ok := c.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errShutdownComponentShutdown, sortedComponenetName[index])
		}
		e.Context, err = component.OnShutdown(e.Context)
		if err != nil {
			return errors.Annotatef(err, errShutdownComponentShutdown, sortedComponenetName[index])
		}
	}
	if e.EngineOptions.callback != nil {
		err = e.EngineOptions.callback.OnShutdown(e)
		if err != nil {
			return errors.Annotatef(err, errShutdownCallback)
		}
	}
	e.state = STATE_SHUTDOWN
	return nil
}

func (e *Engine) ListenAndServe() {
	// server
	if e.ServerMap != nil {
		serverCount := len(e.ServerMap)
		for _, server := range e.ServerMap {
			serverCount--
			if serverCount == 0 {
				e.ServerListenAndServe(server)
			} else {
				go e.ServerListenAndServe(server)
			}
		}
	} else {
		e.Log("ltick: Server not set")
	}
}

func (e *Engine) ServerListenAndServe(server *Server) {
	e.Log("ltick: Server start listen ", server.Port, "...")
	g := graceful.New().Server(
		&http.Server{
			Addr:    fmt.Sprintf(":%d", server.Port),
			Handler: server.Router,
		}).Timeout(server.GetGracefulStopTimeout()).Build()
	if err := g.ListenAndServe(); err != nil {
		if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
			e.Log("ltick: Server stop error: ", err.Error())
			return
		}
	}
	e.Log("ltick: Server stop listen ", server.Port, "...")
}

func (e *Engine) SetContextValue(key, val interface{}) {
	e.Context = context.WithValue(e.Context, key, val)
}
func (e *Engine) GetContextValue(key interface{}) interface{} {
	return e.Context.Value(key)
}
func (e *Engine) GetContextValueString(key string) string {
	value := e.GetContextValue(key)
	if value == nil {
		value = ""
	}
	switch value.(type) {
	case string:
		return value.(string)
	default:
		return ""
	}
}

func (e *Engine) ConfigureComponentFileConfig(componentName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	// configer
	for _, component := range OptionalComponents {
		canonicalExistsComponentName := strings.ToUpper(component.Name[0:1]) + component.Name[1:]
		if canonicalComponentName == canonicalExistsComponentName {
			if len(configTag) > 0 {
				// create a Config object
				err = e.configer.ConfigureFileConfig(component, configFile, configProviders, configTag...)
				if err != nil {
					return errors.Annotatef(err, errConfigureComponentFileConfig, canonicalComponentName)
				}
			} else if component.ConfigurePath != "" {
				err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders, component.ConfigurePath)
				if err != nil {
					return errors.Annotatef(err, errConfigureComponentFileConfig, canonicalComponentName)
				}
			}
		}
	}
	return nil
}

func (e *Engine) GetConfig(key string) interface{} {
	return e.configer.Get(key)
}

func (e *Engine) GetConfigString(key string) string {
	return e.configer.GetString(key)
}

func (e *Engine) GetConfigBool(key string) bool {
	return e.configer.GetBool(key)
}

func (e *Engine) GetConfigInt(key string) int {
	return e.configer.GetInt(key)
}

func (e *Engine) GetConfigInt64(key string) int64 {
	return e.configer.GetInt64(key)
}

func (e *Engine) GetConfigFloat64(key string) float64 {
	return e.configer.GetFloat64(key)
}

func (e *Engine) GetConfigTime(key string) time.Time {
	return e.configer.GetTime(key)
}

func (e *Engine) GetConfigDuration(key string) time.Duration {
	return e.configer.GetDuration(key)
}

func (e *Engine) GetConfigStringSlice(key string) []string {
	return e.configer.GetStringSlice(key)
}

func (e *Engine) GetConfigStringMap(key string) map[string]interface{} {
	return e.configer.GetStringMap(key)
}

func (e *Engine) GetConfigStringMapString(key string) map[string]string {
	return e.configer.GetStringMapString(key)
}

func (e *Engine) GetConfigStringMapStringSlice(key string) map[string][]string {
	return e.configer.GetStringMapStringSlice(key)
}

func (e *Engine) GetConfigSizeInBytes(key string) uint {
	return e.configer.GetSizeInBytes(key)
}
