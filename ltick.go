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
	"sync"
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
	errStartup                   = "ltick: startup error"
	errEngineOption              = "ltick: set engine option error"
	errEngineConfigOption        = "ltick: set engine config option error"
	errNewDefault                = "ltick: new classic error"
	errNewServer                 = "ltick: new server error"
	errGetLogger                 = "ltick: get logger error"
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
		*EngineConfigOptions
		callback  Callback
		logWriter io.Writer
	}

	EngineOption func(*EngineOptions)

	EngineConfigOptions struct {
		configFile string
		dotenvFile string
		envPrefix  string
		configs    map[string]config.Option
	}

	Engine struct {
		*EngineOptions
		state       State
		executeFile string

		cachedConfigFileName string
		configer             *config.Config
		Registry             *Registry
		Context              context.Context
		ServerMap            map[string]*Server
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

	"ACCESS_LOG_TYPE":                    config.Option{Type: config.String, Default: "console", EnvironmentKey: "ACCESS_LOG_TYPE"},
	"ACCESS_LOG_FILE_NAME":               config.Option{Type: config.String, Default: "/tmp/access.log", EnvironmentKey: "ACCESS_LOG_FILE_NAME"},
	"LTICK_ACCESS_LOG_FILE_ROTATE":       config.Option{Type: config.Bool, Default: "true", EnvironmentKey: "LTICK_ACCESS_LOG_FILE_ROTATE"},
	"LTICK_ACCESS_LOG_FILE_BACKUP_COUNT": config.Option{Type: config.Int, Default: "-1", EnvironmentKey: "LTICK_ACCESS_LOG_FILE_BACKUP_COUNT"},
	"ACCESS_LOG_WRITER":                  config.Option{Type: config.String, Default: "discard", EnvironmentKey: "ACCESS_LOG_WRITER"},
	"ACCESS_LOG_MAX_LEVEL":               config.Option{Type: config.String, Default: log.LevelInfo, EnvironmentKey: "ACCESS_LOG_MAX_LEVEL"},
	"ACCESS_LOG_FORMATTER":               config.Option{Type: config.String, Default: "raw", EnvironmentKey: "ACCESS_LOG_FORMATTER"},

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

func EngineConfigFile(configFile string) EngineOption {
	return func(options *EngineOptions) {
		configFile, err := filepath.Abs(configFile)
		if err != nil {
			err = errors.Annotatef(err, errEngineConfigOption)
			fmt.Println(errors.ErrorStack(err))
			os.Exit(1)
		}
		options.EngineConfigOptions.configFile = configFile
	}
}

func EngineConfigDotenvFile(dotenvFile string) EngineOption {
	return func(options *EngineOptions) {
		dotenvFile, err := filepath.Abs(dotenvFile)
		if err != nil {
			err = errors.Annotatef(err, errEngineConfigOption)
			fmt.Println(errors.ErrorStack(err))
			os.Exit(1)
		}
		options.EngineConfigOptions.dotenvFile = dotenvFile
	}
}

func EngineConfigEnvPrefix(envPrefix string) EngineOption {
	return func(options *EngineOptions) {
		options.EngineConfigOptions.envPrefix = envPrefix
	}
}

func EngineConfigConfigs(configs map[string]config.Option) EngineOption {
	return func(options *EngineOptions) {
		options.EngineConfigOptions.configs = configs
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
		EngineConfigOptions: &EngineConfigOptions{
			dotenvFile: defaultDotenvFile,
			configFile: defaultConfigFile,
			envPrefix:  defaultEnvPrefix,
			configs:    defaultConfigs,
		},
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
	// 注册内置 Config 模块
	err = e.Registry.RegisterComponent(&Component{
		Name:      "Config",
		Component: &config.Config{},
	}, true)
	if err != nil {
		err = errors.Annotate(err, errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	configComponent, err := e.Registry.GetComponentByName("Config")
	// Config 模块初始化
	ci, ok := configComponent.Component.(ComponentInterface)
	if !ok {
		err = errors.Annotate(errors.Errorf("invalid type"), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	if e.Registry.ComponentStates[configComponent.Name] == COMPONENT_STATE_INIT {
		e.Registry.ComponentStates[configComponent.Name] = COMPONENT_STATE_PREPARED
		e.Context, err = ci.Prepare(e.Context)
		if err != nil {
			err = errors.Annotate(err, errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	if e.Registry.ComponentStates[configComponent.Name] == COMPONENT_STATE_PREPARED {
		e.Registry.ComponentStates[configComponent.Name] = COMPONENT_STATE_INITIATED
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			err = errors.Annotate(err, errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
	}
	// 注入模块
	err = e.Registry.InjectMiddleware()
	if err != nil {
		err = errors.Annotatef(err, errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	err = e.Registry.InjectComponent()
	if err != nil {
		err = errors.Annotatef(err, errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	componentMap := e.Registry.GetComponentMap()
	for _, name := range e.Registry.GetSortedComponentName() {
		ci, ok := componentMap[name].Component.(ComponentInterface)
		if !ok {
			err = errors.Annotate(errors.Errorf("invalid type"), errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
		if e.Registry.ComponentStates[name] == COMPONENT_STATE_INIT {
			e.Registry.ComponentStates[name] = COMPONENT_STATE_PREPARED
			e.Context, err = ci.Prepare(e.Context)
			if err != nil {
				err = errors.Annotate(err, errNew)
				e.Log(errors.ErrorStack(err))
				os.Exit(1)
			}
		}
	}
	// configer
	configComponent, err = e.Registry.GetComponentByName("Config")
	if err != nil {
		err = errors.Annotate(err, errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	e.configer, ok = configComponent.Component.(*config.Config)
	if !ok {
		err = errors.Annotate(errors.New("ltick: invalid 'Config' type"), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	e.loadConfig(setters...)
	for _, component := range e.Registry.GetComponentMap() {
		err = e.ConfigureComponentFileConfig(component, e.configer.ConfigFileUsed(), make(map[string]interface{}))
		fmt.Println(err)
		// ignore error
		/*if err != nil {
			err = errors.Annotate(err, errNew)
			e.Log(errors.ErrorStack(err))
		}*/
	}
	// 模块初始化
	for _, c := range e.Registry.GetSortedComponents() {
		ci, ok := c.Component.(ComponentInterface)
		if !ok {
			err = errors.Annotate(errors.Errorf("invalid type"), errNew)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
		if e.Registry.ComponentStates[c.Name] == COMPONENT_STATE_PREPARED {
			e.Registry.ComponentStates[c.Name] = COMPONENT_STATE_INITIATED
			e.Context, err = ci.Initiate(e.Context)
			if err != nil {
				err = errors.Annotate(err, errNew)
				e.Log(errors.ErrorStack(err))
				os.Exit(1)
			}
		}
	}
	return e
}

func (e *Engine) ConfigureServerFromFile(s *Server, configFile string, providers map[string]interface{}, configTag string) error {
	err := e.configer.ConfigureFileConfig(s, configFile, providers, "server")
	if err != nil {
		return errors.Annotate(err, errConfigureServer)
	}
	return nil
}

func (e *Engine) ConfigureServerFromJson(s *Server, configJson []byte, providers map[string]interface{}, configTag string) error {
	err := e.configer.ConfigureJsonConfig(s, configJson, providers, "server")
	if err != nil {
		return errors.Annotate(err, errConfigureServer)
	}
	return nil
}

func (e *Engine) NewServer(router *ServerRouter, setters ...ServerOption) *Server {
	middlewares := make([]MiddlewareInterface, 0)
	for _, sortedMiddleware := range e.Registry.GetSortedMiddlewares() {
		middleware, ok := sortedMiddleware.Middleware.(MiddlewareInterface)
		if !ok {
			continue
		}
		middlewares = append(middlewares, middleware)
	}
	setters = append(setters, ServerLogWriter(e.logWriter))
	serverOptions := &ServerOptions{
		logWriter: defaultServerLogWriter,
		Port:      defaultServerPort,
	}
	for _, setter := range setters {
		setter(serverOptions)
	}
	router.WithMiddlewares(middlewares)
	server := &Server{
		ServerOptions: serverOptions,
		Router:        router,
		RouteGroups:   make(map[string]*ServerRouteGroup),
		mutex:         sync.RWMutex{},
	}
	server.Log(fmt.Sprintf("ltick: new server [serverOptions:'%v', serverRouterOptions:'%v', handlerTimeout:'%.fs']", server.ServerOptions, server.Router.ServerRouterOptions, router.TimeoutDuration.Seconds()))
	server.AddRouteGroup("/")
	return server
}

func (e *Engine) loadConfig(setters ...EngineOption) *Engine {
	var err error
	for _, setter := range setters {
		setter(e.EngineOptions)
	}
	err = e.configer.SetOptions(e.EngineOptions.EngineConfigOptions.configs)
	if err != nil {
		err = errors.Annotate(err, errNewDefault)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	// 加载系统配置
	if !path.IsAbs(e.EngineOptions.EngineConfigOptions.configFile) {
		err = errors.Annotate(fmt.Errorf("ltick: '%s' is not a valid config path", e.EngineOptions.EngineConfigOptions.configFile), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	if !path.IsAbs(e.EngineOptions.EngineConfigOptions.dotenvFile) {
		err = errors.Annotate(fmt.Errorf("ltick: '%s' is not a valid dotenv path", e.EngineOptions.EngineConfigOptions.dotenvFile), errNew)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	fileExtension := filepath.Ext(e.EngineOptions.EngineConfigOptions.configFile)
	configCacheFileName := strings.Replace(e.EngineOptions.EngineConfigOptions.configFile, fileExtension, "", -1) + ".cached" + fileExtension
	configCachedFile, err := e.openCacheConfigFile(configCacheFileName)
	if err != nil {
		err = errors.Annotate(err, errLoadSystemConfig)
		e.Log(errors.ErrorStack(err))
		os.Exit(1)
	}
	defer configCachedFile.Close()
	e.cachedConfigFileName = configCachedFile.Name()
	if e.EngineOptions.EngineConfigOptions.dotenvFile != "" {
		e.LoadEnvFile(e.EngineOptions.EngineConfigOptions.envPrefix, e.EngineOptions.EngineConfigOptions.dotenvFile)
	} else {
		e.LoadEnv(e.EngineOptions.EngineConfigOptions.envPrefix)
	}
	e.loadCachedFileConfig(e.EngineOptions.EngineConfigOptions.configFile, e.cachedConfigFileName)
	go func() {
		// 刷新缓存
		for {
			cachedConfigFileInfo, err := os.Stat(e.cachedConfigFileName)
			if err != nil {
				err = errors.Annotate(err, errLoadSystemConfig)
				e.Log(errors.ErrorStack(err))
				return
			}
			if e.EngineOptions.EngineConfigOptions.dotenvFile != "" {
				dotenvFileInfo, err := os.Stat(e.EngineOptions.EngineConfigOptions.dotenvFile)
				if err != nil {
					err = errors.Annotate(err, errLoadSystemConfig)
					e.Log(errors.ErrorStack(err))
					return
				}
				if cachedConfigFileInfo.ModTime().Before(dotenvFileInfo.ModTime()) {
					e.LoadEnvFile(e.EngineOptions.EngineConfigOptions.envPrefix, e.EngineOptions.EngineConfigOptions.dotenvFile)
					e.loadCachedFileConfig(e.EngineOptions.EngineConfigOptions.configFile, e.cachedConfigFileName)
				}
			}
			configFileInfo, err := os.Stat(e.EngineOptions.EngineConfigOptions.configFile)
			if err != nil {
				err = errors.Annotate(err, errLoadSystemConfig)
				e.Log(errors.ErrorStack(err))
				return
			}
			if cachedConfigFileInfo.ModTime().Before(configFileInfo.ModTime()) {
				e.loadCachedFileConfig(e.EngineOptions.EngineConfigOptions.configFile, e.cachedConfigFileName)
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

func (e *Engine) loadCachedFileConfig(configPath string, cachedConfigFileName string) {
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
	e.loadFileConfig(filepath.Dir(cachedConfigFileName), strings.Replace(filepath.Base(cachedConfigFileName), filepath.Ext(cachedConfigFileName), "", 1))
}
func (e *Engine) loadFileConfig(configPath string, configName string) *Engine {
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
func (e *Engine) GetConfigCachedFileName() string {
	return e.cachedConfigFileName
}

func (e *Engine) GetLogger(name string) (*libLog.Logger, error) {
	loggerComponent, err := e.Registry.GetComponentByName("Log")
	if err != nil {
		return nil, errors.Annotate(err, errGetLogger)
	}
	log, ok := loggerComponent.Component.(*log.Logger)
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
		mi, ok := m.Middleware.(MiddlewareInterface)
		if !ok {
			err = errors.Annotate(errors.Errorf("invalid type"), errStartup)
			e.Log(errors.ErrorStack(err))
			os.Exit(1)
		}
		e.Context, err = mi.Initiate(e.Context)
		if err != nil {
			err = errors.Annotate(err, errStartup)
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
			if server.Router == nil {
				continue
			}
			if server.Router.Routes != nil && len(server.Router.Routes) > 0 {
				handlerRoutes := make(map[string]*ServerRouterHandlerRoute)
				for _, route := range server.Router.Routes {
					if route == nil {
						return errors.Annotatef(errors.New("ltick: route does not exists"), errStartup)
					}
					if server.RouteGroups[route.Group] == nil {
						server.RouteGroups[route.Group] = server.AddRouteGroup(route.Group)
					} else if _, ok := server.RouteGroups[route.Group]; !ok {
						server.RouteGroups[route.Group] = server.AddRouteGroup(route.Group)
					}
					for _, method := range route.Method {
						for _, host := range route.Host {
							routeId := route.Group + "|" + method + "|" + route.Path
							if _, ok := handlerRoutes[routeId]; !ok {
								handlerRoutes[routeId] = &ServerRouterHandlerRoute{
									Handlers:route.Handlers,
									Host:[]string{host},
								}
							} else {
								handlerRoutes[routeId].Handlers = append(handlerRoutes[routeId].Handlers, route.Handlers...)
								handlerRoutes[routeId].Host = append(handlerRoutes[routeId].Host, host)
							}
						}
					}
				}
				for routeId, handlerRoute := range handlerRoutes {
					routeIds := strings.SplitN(routeId, "|", 4)
					routeGroup := routeIds[0]
					routeMethod := routeIds[2]
					routePath := routeIds[3]
					server.RouteGroups[routeGroup].AddApiRoute(routeMethod, routePath, handlerRoute.Host, handlerRoute.Handlers...)
				}
			}
			// proxy
			if server.Router.Proxys != nil && len(server.Router.Proxys) > 0 {
				for _, proxy := range server.Router.Proxys {
					if proxy == nil {
						return errors.Annotatef(errors.New("ltick: route does not exists"), errStartup)
					}
					if server.RouteGroups[proxy.Group] == nil {
						server.RouteGroups[proxy.Group] = server.AddRouteGroup(proxy.Group)
					} else if _, ok := server.RouteGroups[proxy.Group]; !ok {
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
	sortedComponents := e.Registry.GetSortedComponents()
	// 模块初始化
	for index, c := range sortedComponents {
		ci, ok := c.Component.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errStartupComponentInitiate, sortedComponenetName[index])
		}
		if e.Registry.ComponentStates[c.Name] == COMPONENT_STATE_PREPARED {
			e.Registry.ComponentStates[c.Name] = COMPONENT_STATE_INITIATED
			e.Context, err = ci.Initiate(e.Context)
			if err != nil {
				return errors.Annotatef(err, errStartupComponentInitiate, sortedComponenetName[index])
			}
		}
	}
	// 模块启动
	for index, c := range sortedComponents {
		ci, ok := c.Component.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errStartupComponentStartup, sortedComponenetName[index])
		}
		if e.Registry.ComponentStates[c.Name] == COMPONENT_STATE_INITIATED {
			e.Registry.ComponentStates[c.Name] = COMPONENT_STATE_STARTUP
			e.Context, err = ci.OnStartup(e.Context)
			if err != nil {
				return errors.Annotatef(err, errStartupComponentStartup, sortedComponenetName[index])
			}
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
		component, ok := c.Component.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errShutdownComponentShutdown, sortedComponenetName[index])
		}
		if e.Registry.ComponentStates[c.Name] == COMPONENT_STATE_STARTUP {
			e.Registry.ComponentStates[c.Name] = COMPONENT_STATE_SHUTDOWN
			e.Context, err = component.OnShutdown(e.Context)
			if err != nil {
				return errors.Annotatef(err, errShutdownComponentShutdown, sortedComponenetName[index])
			}
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

func (e *Engine) ConfigureComponentFileConfig(component *Component, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	// configer
	if len(configTag) > 0 {
		// create a Config object
		err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders, configTag...)
		if err != nil {
			return errors.Annotatef(err, errConfigureComponentFileConfig, component)
		}
	} else if component.ConfigurePath != "" {
		err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders, component.ConfigurePath)
		if err != nil {
			return errors.Annotatef(err, errConfigureComponentFileConfig, component)
		}
	} else {
		err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders)
		if err != nil {
			return errors.Annotatef(err, errConfigureComponentFileConfig, component)
		}
	}
	return nil
}

func (e *Engine) ConfigureComponentFileConfigByName(name string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	// configer
	component, err := e.Registry.GetComponentByName(name)
	if err != nil {
		return errors.Annotatef(err, errConfigureComponentFileConfigByName, name)
	}
	if len(configTag) > 0 {
		// create a Config object
		err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders, configTag...)
		if err != nil {
			return errors.Annotatef(err, errConfigureComponentFileConfigByName, name)
		}
	} else if component.ConfigurePath != "" {
		err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders, component.ConfigurePath)
		if err != nil {
			return errors.Annotatef(err, errConfigureComponentFileConfigByName, name)
		}
	} else {
		err = e.configer.ConfigureFileConfig(component.Component, configFile, configProviders)
		if err != nil {
			return errors.Annotatef(err, errConfigureComponentFileConfigByName, name)
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
