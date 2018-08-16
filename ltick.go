package ltick

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ltick/tick-framework/module"
	libConfig "github.com/ltick/tick-framework/module/config"
	libLogger "github.com/ltick/tick-framework/module/logger"
	libUtility "github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-graceful"
	"github.com/ltick/tick-routing"
)

var (
	errNew                        = "ltick: new error"
	errNewClassic                 = "ltick: new classic error"
	errNewServer                  = "ltick: new server error"
	errConfigure                  = "ltick: configure error"
	errStartup                    = "ltick: startup error"
	errGetLogger                  = "ltick: get logger error"
	errWithValues                 = "ltick: with values error"
	errStartupCallback            = "ltick: startup callback error"
	errStartupRouterCallback      = "ltick: startup router callback error"
	errStartupRouteGroupCallback  = "ltick: startup route group callback error"
	errStartupConfigureModule     = "ltick: startup configure module error"
	errStartupInjectModule        = "ltick: startup inject module error"
	errStartupInjectBuiltinModule = "ltick: startup inject builtin module error"

	errLoadSystemConfig  = "ltick: load system config error"
	errLoadEnvConfigFile = "ltick: load env config file error"
)

type State int8

const (
	STATE_INITIATE State = iota
	STATE_STARTUP
	STATE_SHUTDOWN
)

type (
	Engine struct {
		state           State
		executeFile     string
		systemLogWriter io.Writer
		callback        Callback
		option          *Option
		modules         []*module.Module
		Config          *libConfig.Instance
		Utility         *libUtility.Instance
		Logger          *libLogger.Instance

		Module  *module.Instance
		Context context.Context
		Servers map[string]*Server
	}
	Option struct {
		PathPrefix string
		EnvPrefix  string
	}
	Callback interface {
		OnStartup(*Engine) error  // Execute On After All Engine Module OnStartup
		OnShutdown(*Engine) error // Execute On After All Engine Module OnShutdown
	}
	LogHanlder struct {
		Name      string
		Formatter string
		Type      string
		Filename  string
		Writer    string // the writer name of writer (stdout, stderr, discard)
		MaxLevel  libLogger.Level
	}
)

var defaultConfigName = "ltick.json"
var defaultConfigReloadTime = 120 * time.Second
var configPlaceholdRegExp = regexp.MustCompile(`%\w+%`)

func NewClassic(modules []*module.Module, configOptions map[string]libConfig.Option, option *Option) (engine *Engine) {
	executeFile, err := exec.LookPath(os.Args[0])
	if err != nil {
		fmt.Printf(errNewClassic+": %s\r\n", err.Error())
		os.Exit(1)
	}
	executePath, err := filepath.Abs(executeFile)
	if err != nil {
		fmt.Printf(errNewClassic+": %s\r\n", err.Error())
		os.Exit(1)
	}
	if option.PathPrefix == "" {
		option.PathPrefix = filepath.Dir(filepath.Dir(executePath))
	}
	engine = New(executeFile, option.PathPrefix, defaultConfigName, option.EnvPrefix, modules, configOptions)
	logHandlers := make([]*LogHanlder, 0)
	loggerTargetsConfig := engine.Config.GetStringMap("modules.logger.targets")
	for loggerName, loggerTargetInterface := range loggerTargetsConfig {
		loggerTarget := loggerTargetInterface.(map[string]interface{})
		loggerTargetTypeInterface, ok := loggerTarget["type"]
		if ok {
			loggerTargetType, ok := loggerTargetTypeInterface.(string)
			if ok {
				loggerTargetMaxLevel := libLogger.LevelDebug
				for level, levelName := range libLogger.LevelNames {
					if levelName == loggerTarget["maxlevel"] {
						loggerTargetMaxLevel = level
						break
					}
				}
				switch loggerTargetType {
				case "file":
					loggerTargetFormatterInterface, ok := loggerTarget["formatter"]
					if !ok {
						continue
					}
					loggerTargetFormatter, ok := loggerTargetFormatterInterface.(string)
					if !ok {
						continue
					}
					loggerTargetFilenameInterface, ok := loggerTarget["filename"]
					if !ok {
						continue
					}
					loggerTargetFilename, ok := loggerTargetFilenameInterface.(string)
					if !ok {
						continue
					}
					logHandlers = append(logHandlers, &LogHanlder{
						Name:      loggerName,
						Type:      loggerTargetType,
						Formatter: loggerTargetFormatter,
						Filename:  loggerTargetFilename,
						MaxLevel:  loggerTargetMaxLevel,
					})
				case "console":
					loggerTargetFormatterInterface, ok := loggerTarget["formatter"]
					if !ok {
						continue
					}
					loggerTargetFormatter, ok := loggerTargetFormatterInterface.(string)
					if !ok {
						continue
					}
					loggerTargetWriterInterface, ok := loggerTarget["writer"]
					if !ok {
						continue
					}
					loggerTargetWriter, ok := loggerTargetWriterInterface.(string)
					if !ok {
						continue
					}
					logHandlers = append(logHandlers, &LogHanlder{
						Name:      loggerName,
						Type:      loggerTargetType,
						Formatter: loggerTargetFormatter,
						Writer:    loggerTargetWriter,
						MaxLevel:  loggerTargetMaxLevel,
					})
				}
			}
		}
	}
	engine.WithLoggers(logHandlers)
	return engine
}

func New(executeFile string, pathPrefix string, configName string, envPrefix string, modules []*module.Module, configOptions map[string]libConfig.Option) (engine *Engine) {
	engine = &Engine{
		option:          &Option{},
		state:           STATE_INITIATE,
		executeFile:     executeFile,
		systemLogWriter: os.Stdout,
		Context:         context.Background(),
		Servers:         make(map[string]*Server, 0),
	}
	ctx, module, err := module.NewInstance(engine.Context)
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	configModule, err := module.GetBuiltinModule("Config")
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	config, ok := configModule.(*libConfig.Instance)
	if !ok {
		fmt.Printf(errNew+": %s\r\n", "invalid 'Config' module type")
		os.Exit(1)
	}
	utilityModule, err := module.GetBuiltinModule("Utility")
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	utility, ok := utilityModule.(*libUtility.Instance)
	if !ok {
		fmt.Printf(errNew+": %s\r\n", "invalid 'Utility' module type")
		os.Exit(1)
	}
	loggerModule, err := module.GetBuiltinModule("logger")
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	logger, ok := loggerModule.(*libLogger.Instance)
	if !ok {
		fmt.Printf(errNew+": %s\r\n", "invalid 'Logger' module type")
		os.Exit(1)
	}
	engine.modules = modules
	engine.Context = ctx
	engine.Module = module
	engine.Config = config
	engine.Utility = utility
	engine.Logger = logger
	engine.SetPathPrefix(pathPrefix)
	engine.Context, err = engine.Config.SetOptions(engine.Context, configOptions)
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	engine.LoadSystemConfig(pathPrefix+"/etc/"+configName, envPrefix, pathPrefix+"/.env")
	return engine
}
func (e *Engine) LoadSystemConfig(configFilePath string, envPrefix string, dotEnvFile string) *Engine {
	configCachedFile, err := e.Utility.GetCachedFile(configFilePath)
	if err != nil {
		fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
	}
	defer configCachedFile.Close()
	cachedConfigFilePath := configCachedFile.Name()
	e.LoadEnvConfigFile(envPrefix, dotEnvFile)
	e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
	go func() {
		// 刷新缓存
		for {
			dotEnvFileInfo, err := os.Stat(dotEnvFile)
			if err != nil {
				fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
			}
			configFileInfo, err := os.Stat(configFilePath)
			if err != nil {
				fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
			}
			cachedConfigFileInfo, err := os.Stat(cachedConfigFilePath)
			if err != nil {
				fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
			}
			if cachedConfigFileInfo.ModTime().Before(dotEnvFileInfo.ModTime()) {
				e.LoadEnvConfigFile(envPrefix, dotEnvFile)
				e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
			}
			if cachedConfigFileInfo.ModTime().Before(configFileInfo.ModTime()) {
				e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
			}
			time.Sleep(defaultConfigReloadTime)
		}
	}()
	return e
}
func (e *Engine) SetConfigOptions(configOptions map[string]libConfig.Option) (err error) {
	e.Context, err = e.Config.SetOptions(e.Context, configOptions)
	if err != nil {
		fmt.Printf("ltick: load from config file error: %s\n", err.Error())
		os.Exit(1)
	}
	return nil
}
func (e *Engine) LoadCachedConfig(configFilePath string, cachedConfigFilePath string) {
	configFile, err := os.OpenFile(configFilePath, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
		os.Exit(1)
	}
	defer configFile.Close()
	cachedFileByte, err := ioutil.ReadAll(configFile)
	if err != nil {
		fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
		os.Exit(1)
	}
	matches := configPlaceholdRegExp.FindAll(cachedFileByte, -1)
	for _, match := range matches {
		replaceKey := string(match)
		replaceConfigKey := strings.Trim(replaceKey, "%")
		cachedFileByte = bytes.Replace(cachedFileByte, []byte(replaceKey), []byte(e.Config.GetString(replaceConfigKey)), -1)
	}
	err = ioutil.WriteFile(cachedConfigFilePath, cachedFileByte, 0644)
	if err != nil {
		fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
		os.Exit(1)
	}
	e.LoadConfig(filepath.Dir(cachedConfigFilePath), strings.Replace(filepath.Base(cachedConfigFilePath), filepath.Ext(cachedConfigFilePath), "", 1))
}
func (e *Engine) LoadConfig(configPath string, configName string) *Engine {
	var err error
	if configPath == "" || configName == "" {
		fmt.Printf("ltick: load config [path:'%s', name:'%s', error:'config_path or config_name is empty']\n", configPath, configPath)
		os.Exit(1)
	}
	if !strings.HasPrefix(configPath, "/") {
		configPath = strings.TrimRight(configPath, "/") + "/" + configPath
	}
	_, err = os.Stat(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("ltick: load config [path:'%s', name:'%s', error:'%s']\n", configPath, configName, err.Error())
			os.Exit(1)
		}
	}
	e.Config.AddConfigPath(configPath)
	err = e.Config.LoadFromConfigPath(configName)
	if err != nil {
		fmt.Printf("ltick: load config [path:'%s', name:'%s', error:'%s']\n", configPath, configName, err.Error())
		os.Exit(1)
	}
	return e
}
func (e *Engine) LoadEnv(envPrefix string) *Engine {
	e.Config.SetEnvPrefix(envPrefix)
	err := e.Config.LoadFromEnv()
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("ltick: load env [env_prefix:'%s', binded_environment_keys:'%v', error:'%s']\n", envPrefix, e.Config.BindedEnvironmentKeys(), err.Error())
			os.Exit(1)
		}
	}
	return nil
}
func (e *Engine) LoadEnvConfigFile(envPrefix string, dotEnvFile string) *Engine {
	e.option.EnvPrefix = envPrefix
	e.Config.SetPathPrefix(e.option.PathPrefix)
	e.Config.SetEnvPrefix(envPrefix)
	err := e.Config.LoadFromEnvFile(dotEnvFile)
	if err != nil {
		fmt.Printf(errLoadEnvConfigFile+": %s\r\n", err.Error())
		os.Exit(1)
	}
	return e
}
func (e *Engine) WithValues(values map[string]interface{}) *Engine {
	for key, value := range values {
		err := e.RegisterValue(key, value)
		if err != nil {
			fmt.Printf(errWithValues+": %s [key:'%s']\r\n", err.Error(), key)
			os.Exit(1)
		}
	}
	return e
}
func (e *Engine) WithCallback(callback Callback) *Engine {
	e.callback = callback
	return e
}
func (e *Engine) GetLogger(name string) (*libLogger.Logger, error) {
	logger, err := e.Logger.GetLogger(name)
	if err != nil {
		return nil, errors.New(errGetLogger + ": " + err.Error())
	}
	return &libLogger.Logger{logger}, nil
}
func (e *Engine) WithLoggers(handlers []*LogHanlder) *Engine {
	var logProviders map[string]interface{} = make(map[string]interface{})
	logTargetProviderConfigs := make([]string, len(handlers))
	for index, hanlder := range handlers {
		switch hanlder.Type {
		case "file":
			logFilename := hanlder.Filename
			if !strings.HasPrefix(logFilename, "/") {
				logFilename = e.option.PathPrefix + "/" + logFilename
			}
			_, err := os.Stat(logFilename)
			if err != nil {
				if os.IsNotExist(err) {
					_, err = os.Create(logFilename)
					if err != nil {
						fmt.Printf("ltick: fail to create %s log file '%s' error:%s\r\n", hanlder.Name, logFilename, err.Error())
						os.Exit(1)
					}
				} else {
					fmt.Printf("ltick: check system log file '%s' error:%s\r\n", hanlder.Name, logFilename, err.Error())
					os.Exit(1)
				}
			}
			logTargetProviderName := hanlder.Name + "FileTarget"
			logProviders[logTargetProviderName] = libLogger.NewFileTarget
			logTargetProviderConfigs[index] = `"` + hanlder.Name + `":{"type": "` + logTargetProviderName + `","Filename":"` + logFilename + `","Rotate":true,"MaxBytes":` + strconv.Itoa(1<<22) + `}`
			e.SystemLog("ltick: register log [name: '" + hanlder.Name + "', target: 'file', file: '" + logFilename + "']")
		case "console":
			logWriter := hanlder.Writer
			logTargetProviderName := hanlder.Name + "ConsoleTarget"
			logProviders[logTargetProviderName] = libLogger.NewConsoleTarget
			logTargetProviderConfigs[index] = `"` + hanlder.Name + `":{"type": "` + logTargetProviderName + `","Writer":"` + logWriter + `"}`
			index++
			e.SystemLog("ltick: register log [name: '" + hanlder.Name + "', target:'console', writer:'" + logWriter + "']")
		}
	}
	loggerConfig := `{`
	if len(logTargetProviderConfigs) > 0 {
		loggerConfig = loggerConfig + `"Targets": {` + strings.Join(logTargetProviderConfigs, ",") + `}`
	}
	loggerConfig = loggerConfig + `}`
	e.Logger.LoadModuleJsonConfig([]byte(loggerConfig), logProviders)
	for _, hanlder := range handlers {
		e.Logger.NewLogger(hanlder.Name)
		switch hanlder.Formatter {
		case "raw":
			e.Logger.SetLoggerFormatter(hanlder.Name, libLogger.RawLogFormatter())
		case "sys":
			e.Logger.SetLoggerFormatter(hanlder.Name, libLogger.SysLogFormatter())
		default:
			e.Logger.SetLoggerFormatter(hanlder.Name, libLogger.DefaultLogFormatter())
		}
		e.Logger.SetLoggerTarget(hanlder.Name, hanlder.Name)
		e.Logger.SetLoggerMaxLevel(hanlder.Name, hanlder.MaxLevel)
		e.Logger.OpenLogger(hanlder.Name)
	}
	return e
}
func (e *Engine) SetSystemLogWriter(systemLogWriter io.Writer) {
	e.systemLogWriter = systemLogWriter
}
func (e *Engine) SystemLog(args ...interface{}) {
	fmt.Fprintln(e.systemLogWriter, args...)
}
func (e *Engine) SetPathPrefix(pathPrefix string) {
	e.Context = context.WithValue(e.Context, "PATH_PREFIX", pathPrefix)
	e.option.PathPrefix = pathPrefix
}
func (e *Engine) GetConfigString(key string) string {
	return e.Config.GetString(key)
}
func (e *Engine) GetConfigBool(key string) bool {
	return e.Config.GetBool(key)
}
func (e *Engine) GetConfigInt(key string) int {
	return e.Config.GetInt(key)
}
func (e *Engine) GetConfigInt64(key string) int64 {
	return e.Config.GetInt64(key)
}
func (e *Engine) Startup() (err error) {
	if e.state != STATE_INITIATE {
		return nil
	}
	e.SystemLog("ltick: Engine start.")
	e.SystemLog("ltick: Execute file \"" + e.executeFile + "\"")
	e.SystemLog("ltick: Startup")
	if e.callback != nil {
		err = e.Module.InjectModuleTo([]interface{}{e.callback})
		if err != nil {
			return fmt.Errorf(errStartupCallback+": %s", err.Error())
		}
		err = e.callback.OnStartup(e)
		if err != nil {
			return fmt.Errorf(errStartupCallback+": %s", err.Error())
		}
	}
	for _, server := range e.Servers {
		if server.Router.routes != nil && len(server.Router.routes) > 0 {
			for _, route := range server.Router.routes {
				server.addRoute(route.Method, route.Host, route.Handlers...)
			}
		}
		// proxy
		if server.Router.proxys != nil && len(server.Router.proxys) > 0 {
			server.addRoute("ANY", "/", func(c *routing.Context) error {
				for _, proxy := range server.Router.proxys {
					upstreamURL, err := proxy.MatchProxy(c.Request)
					if err != nil {
						return routing.NewHTTPError(http.StatusInternalServerError, err.Error())
					}
					if upstreamURL != nil {
						director := func(req *http.Request) {
							req = c.Request
							req.URL.Scheme = upstreamURL.Scheme
							req.URL.Host = upstreamURL.Host
							req.RequestURI = upstreamURL.RequestURI()
						}
						proxy := &httputil.ReverseProxy{Director: director}
						proxy.ServeHTTP(c.Response, c.Request)
						c.Abort()
						return nil
					}
				}
				return nil
			})
		}
		if server.RouteGroups != nil {
			for _, routeGroup := range server.RouteGroups {
				if routeGroup.callback != nil {
					err = e.Module.InjectModuleTo([]interface{}{routeGroup.callback})
					if err != nil {
						return fmt.Errorf(errStartupRouteGroupCallback+": %s", err.Error())
					}
				}
			}
		}
		if server.Router.callback != nil {
			err = e.Module.InjectModuleTo([]interface{}{server.Router.callback})
			if err != nil {
				return fmt.Errorf(errStartupRouterCallback+": %s", err.Error())
			}
		}
	}
	// 内置模块注入
	for index, sortedBuiltinModule := range e.Module.GetSortedBuiltinModules() {
		sortedBuiltinModuleInstance, ok := sortedBuiltinModule.(module.ModuleInterface)
		if !ok {
			return fmt.Errorf(errStartupInjectBuiltinModule+": invalid '%s' module type", e.Module.SortedBuiltinModules[index])
		}
		e.Context, err = sortedBuiltinModuleInstance.OnStartup(e.Context)
		if err != nil {
			return fmt.Errorf(errStartupInjectBuiltinModule+": %s", err.Error())
		}
	}
	sortedModules := e.Module.GetSortedModules()
	// 模块启动
	for index, sortedModule := range sortedModules {
		sortedModuleInstance, ok := sortedModule.(module.ModuleInterface)
		if !ok {
			return fmt.Errorf(errStartupInjectModule+": invalid '%s' module type", e.Module.SortedModules[index])
		}
		e.Context, err = sortedModuleInstance.OnStartup(e.Context)
		if err != nil {
			return fmt.Errorf(errStartupInjectModule+": %s", err.Error())
		}
	}
	// 注册模块
	for _, module := range e.modules {
		err := e.RegisterUserModule(module.Name, module.Module)
		if err != nil {
			return fmt.Errorf(errStartupInjectModule+": %s [module:'%s']", err.Error(), module.Name)
		}
	}
	// 注入模块
	err = e.Module.InjectModule()
	if err != nil {
		return fmt.Errorf(errStartupInjectModule+": %s", err.Error())
	}
	e.state = STATE_STARTUP
	return nil
}

func (e *Engine) Shutdown() (err error) {
	if e.state != STATE_STARTUP {
		return nil
	}
	e.SystemLog("ltick: Shutdown")
	for _, sortedModule := range e.Module.GetSortedModules(true) {
		module, ok := sortedModule.(module.ModuleInterface)
		if !ok {
			e.SystemLog("ltick: Shutdown module error: invalid module type")
		}
		e.Context, err = module.OnShutdown(e.Context)
		if err != nil {
			e.SystemLog("ltick: Shutdown module error: " + err.Error())
			return err
		}
	}
	if e.callback != nil {
		err = e.callback.OnShutdown(e)
		if err != nil {
			e.SystemLog("ltick: Shutdown callback error: " + err.Error())
			return err
		}
	}
	e.state = STATE_SHUTDOWN
	return nil
}

func (e *Engine) ListenAndServe() {
	// server
	if e.Servers != nil {
		serverCount := len(e.Servers)
		for _, server := range e.Servers {
			serverCount--
			if serverCount == 0 {
				e.ServerListenAndServe(server)
			} else {
				go e.ServerListenAndServe(server)
			}
		}
	} else {
		e.SystemLog("ltick: Server not set")
	}
}

func (e *Engine) ServerListenAndServe(server *Server) {
	e.SystemLog("ltick: Server start listen ", server.Port, "...")
	g := graceful.New().Server(
		&http.Server{
			Addr:    fmt.Sprintf(":%d", server.Port),
			Handler: server.Router,
		}).Timeout(server.gracefulStopTimeout).Build()
	if err := g.ListenAndServe(); err != nil {
		if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
			e.SystemLog("ltick: Server stop error: ", err.Error())
			os.Exit(1)
		}
	}
	e.SystemLog("ltick: Server stop listen ", server.Port, "...")
}

func (e *Engine) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// server
	if e.Servers != nil {
		serverCount := len(e.Servers)
		for name, server := range e.Servers {
			serverCount--
			if serverCount == 0 {
				e.ServerServeHTTP(name, server, res, req)
			} else {
				go e.ServerServeHTTP(name, server, res, req)
			}
		}
	} else {
		e.SystemLog("ltick: Server not set")
	}
}
func (e *Engine) ServerServeHTTP(name string, server *Server, res http.ResponseWriter, req *http.Request) {
	server.Router.ServeHTTP(res, req)
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
func (e *Engine) UseModule(moduleNames ...string) (err error) {
	e.Context, err = e.Module.UseModule(e.Context, moduleNames...)
	return err
}

// Register As Module
func (e *Engine) RegisterUserModule(moduleName string, module module.ModuleInterface, forceOverwrites ...bool) (err error) {
	e.Context, err = e.Module.RegisterUserModule(e.Context, moduleName, module, forceOverwrites...)
	return err
}

// Unregister As Module
func (e *Engine) UnregisterUserModule(moduleNames ...string) (err error) {
	e.Context, err = e.Module.UnregisterUserModule(e.Context, moduleNames...)
	return err
}

func (e *Engine) GetBuiltinModule(moduleName string) (interface{}, error) {
	return e.Module.GetBuiltinModule(moduleName)
}

func (e *Engine) GetBuiltinModules() map[string]interface{} {
	return e.Module.GetBuiltinModules()
}

func (e *Engine) GetModule(moduleName string) (interface{}, error) {
	return e.Module.GetModule(moduleName)
}

func (e *Engine) GetModules() map[string]interface{} {
	return e.Module.GetModules()
}

func (e *Engine) LoadModuleFileConfig(moduleName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	return e.Module.LoadModuleFileConfig(moduleName, configFile, configProviders, configTag...)
}

// Register As Module
func (e *Engine) LoadModuleJsonConfig(moduleName string, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
	return e.Module.LoadModuleJsonConfig(moduleName, configData, configProviders, configTag...)
}

// Register As Value
func (e *Engine) RegisterValue(key string, value interface{}, forceOverwrites ...bool) (err error) {
	e.Context, err = e.Module.RegisterValue(e.Context, key, value, forceOverwrites...)
	return err
}

// Unregister As Value
func (e *Engine) UnregisterValue(keys ...string) (context.Context, error) {
	return e.Module.UnregisterValue(e.Context, keys...)
}

func (e *Engine) GetValue(key string) (interface{}, error) {
	return e.Module.GetValue(key)
}

func (e *Engine) GetValues() map[string]interface{} {
	return e.Module.GetValues()
}

func (e *Engine) InjectModule() error {
	return e.Module.InjectModule()
}
func (e *Engine) InjectModuleByName(moduleNames ...string) error {
	return e.Module.InjectModuleByName(moduleNames)
}
