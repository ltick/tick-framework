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
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	libConfig "github.com/ltick/tick-framework/config"
	libLogger "github.com/ltick/tick-framework/logger"
	libUtility "github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-graceful"
)

var (
	errNew                       = "ltick: new error"
	errNewClassic                = "ltick: new classic error"
	errNewServer                 = "ltick: new server error"
	errConfigure                 = "ltick: configure error"
	errStartup                   = "ltick: startup error"
	errGetLogger                 = "ltick: get logger error"
	errWithValues                = "ltick: with values error"
	errInitiateComponent         = "ltick: initiate component '%s' error"
	errStartupCallback           = "ltick: startup callback error"
	errStartupRouterCallback     = "ltick: startup router callback error"
	errStartupRouteGroupCallback = "ltick: startup route group callback error"
	errStartupConfigureComponent = "ltick: startup configure component error"
	errStartupInjectComponent    = "ltick: startup inject component error"
	errStartupInitiateComponent  = "ltick: startup initiate component '%s' error"

	errLoadCachedConfig = "ltick: load cached config error"
	errLoadSystemConfig = "ltick: load system config error"
	errLoadEnvFile      = "ltick: load env file error"
)

type State int8

const (
	STATE_INITIATE State = iota
	STATE_STARTUP
	STATE_SHUTDOWN
)

type (
	Engine struct {
		state            State
		executeFile      string
		systemLogWriter  io.Writer
		callback         Callback
		option           *Option
		Components       []interface{}
		ComponentMap     map[string]interface{}
		SortedComponents []string
		Values           map[string]interface{}
		Config           *libConfig.Config `inject:true`
		Logger           *libLogger.Logger `inject:true`

		Context context.Context
		Servers map[string]*Server
	}
	Option struct {
		PathPrefix string
		EnvPrefix  string
	}
	Callback interface {
		OnStartup(*Engine) error  // Execute On After All Engine Component OnStartup
		OnShutdown(*Engine) error // Execute On After All Engine Component OnShutdown
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

var defaultConfigPath = "etc/ltick.json"
var defaultConfigReloadTime = 120 * time.Second
var configPlaceholdRegExp = regexp.MustCompile(`%\w+%`)

func NewClassic(components []*Component, configOptions map[string]libConfig.Option, option *Option) (engine *Engine) {
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
	engine = New(executeFile, option.PathPrefix, defaultConfigPath, option.EnvPrefix, components, configOptions)
	logHandlers := make([]*LogHanlder, 0)
	loggerTargetsConfig := engine.Config.GetStringMap("components.logger.targets")
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

func New(executeFile string, pathPrefix string, configPath string, envPrefix string, components []*Component, configOptions map[string]libConfig.Option) (e *Engine) {
	e = &Engine{
		option:           &Option{},
		state:            STATE_INITIATE,
		executeFile:      executeFile,
		systemLogWriter:  os.Stdout,
		Context:          context.Background(),
		Servers:          make(map[string]*Server, 0),
		Components:       make([]interface{}, 0),
		ComponentMap:     make(map[string]interface{}),
		SortedComponents: make([]string, 0),
		Values:           make(map[string]interface{}),
	}
	var err error
	// 注册内置模块
	for _, component := range BuiltinComponents {
		err = e.RegisterComponent(component.Name, component.Component, true)
		if err != nil {
			fmt.Printf(errNew+": %s\r\n", err.Error())
			os.Exit(1)
		}
	}
	// 模块初始化
	for index, c := range e.Components {
		ci, ok := c.(ComponentInterface)
		if !ok {
			fmt.Printf(errInitiateComponent+": invalid type", e.SortedComponents[index])
			os.Exit(1)
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			fmt.Printf(errInitiateComponent+": %s", e.SortedComponents[index], err.Error())
			os.Exit(1)
		}
	}
	configComponent, err := e.GetComponentByName("Config")
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	config, ok := configComponent.(*libConfig.Config)
	if !ok {
		fmt.Printf(errNew+": %s\r\n", "invalid 'Config' component type")
		os.Exit(1)
	}
	loggerComponent, err := e.GetComponentByName("Logger")
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	logger, ok := loggerComponent.(*libLogger.Logger)
	if !ok {
		fmt.Printf(errNew+": %s\r\n", "invalid 'Logger' component type")
		os.Exit(1)
	}
	for _, c := range components {
		err = e.RegisterComponent(c.Name, c.Component, true)
		if err != nil {
			fmt.Printf(errNew+": %s\r\n", err.Error())
			os.Exit(1)
		}
	}
	e.Config = config
	e.Logger = logger
	e.SetPathPrefix(pathPrefix)
	e.Context, err = e.Config.SetOptions(e.Context, configOptions)
	if err != nil {
		fmt.Printf(errNew+": %s\r\n", err.Error())
		os.Exit(1)
	}
	if !path.IsAbs(configPath) {
		configPath = pathPrefix + "/" + configPath
	}
	_, err = os.Stat(pathPrefix + "/.env")
	if err == nil {
		e.LoadSystemConfig(configPath, envPrefix, pathPrefix+"/.env")
	} else {
		e.LoadSystemConfig(configPath, envPrefix)
	}
	return e
}
func (e *Engine) LoadSystemConfig(configFilePath string, envPrefix string, dotEnvFiles ...string) *Engine {
	var dotEnvFile string
	if len(dotEnvFiles) > 0 {
		dotEnvFile = dotEnvFiles[0]
	}
	configCachedFile, err := libUtility.GetCachedFile(configFilePath)
	if err != nil {
		fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
	}
	defer configCachedFile.Close()
	cachedConfigFilePath := configCachedFile.Name()
	if dotEnvFile != "" {
		e.LoadEnvFile(envPrefix, dotEnvFile)
	} else {
		e.LoadEnv(envPrefix)
	}
	e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
	go func() {
		// 刷新缓存
		for {
			cachedConfigFileInfo, err := os.Stat(cachedConfigFilePath)
			if err != nil {
				fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
			}
			if dotEnvFile != "" {
				dotEnvFileInfo, err := os.Stat(dotEnvFile)
				if err != nil {
					fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
				}
				if cachedConfigFileInfo.ModTime().Before(dotEnvFileInfo.ModTime()) {
					e.LoadEnvFile(envPrefix, dotEnvFile)
					e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
				}
			}
			configFileInfo, err := os.Stat(configFilePath)
			if err != nil {
				fmt.Printf(errLoadSystemConfig+": %s\r\n", err.Error())
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
		fmt.Printf(errLoadCachedConfig+": %s\r\n", err.Error())
		os.Exit(1)
	}
	defer configFile.Close()
	cachedFileByte, err := ioutil.ReadAll(configFile)
	if err != nil {
		fmt.Printf(errLoadCachedConfig+": %s\r\n", err.Error())
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
		fmt.Printf(errLoadCachedConfig+": %s\r\n", err.Error())
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
func (e *Engine) LoadEnvFile(envPrefix string, dotEnvFile string) *Engine {
	e.option.EnvPrefix = envPrefix
	e.Config.SetPathPrefix(e.option.PathPrefix)
	e.Config.SetEnvPrefix(envPrefix)
	err := e.Config.LoadFromEnvFile(dotEnvFile)
	if err != nil {
		fmt.Printf(errLoadEnvFile+": %s\r\n", err.Error())
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
	logger, err := e.GetLogger(name)
	if err != nil {
		return nil, errors.New(errGetLogger + ": " + err.Error())
	}
	return logger, nil
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
					fmt.Printf("ltick: fail to create %s log file '%s' error:%s\r\n", hanlder.Name, logFilename, err.Error())
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
	e.Logger.LoadComponentJsonConfig([]byte(loggerConfig), logProviders)
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
	e.SystemLog("ltick: Execute file \"" + e.executeFile + "\"")
	e.SystemLog("ltick: Startup")
	if e.callback != nil {
		err = e.InjectComponentTo([]interface{}{e.callback})
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
			server.addRoute("ANY", "/")
		}
		if server.RouteGroups != nil {
			for _, routeGroup := range server.RouteGroups {
				if routeGroup.callback != nil {
					err = e.InjectComponentTo([]interface{}{routeGroup.callback})
					if err != nil {
						return fmt.Errorf(errStartupRouteGroupCallback+": %s", err.Error())
					}
				}
			}
		}
		if server.Router.callback != nil {
			err = e.InjectComponentTo([]interface{}{server.Router.callback})
			if err != nil {
				return fmt.Errorf(errStartupRouterCallback+": %s", err.Error())
			}
		}
	}
	sortedComponents := e.GetSortedComponents()
	// 模块初始化
	for index, c := range sortedComponents {
		ci, ok := c.(ComponentInterface)
		if !ok {
			return fmt.Errorf(errStartupInitiateComponent+": invalid type", e.SortedComponents[index])
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			return fmt.Errorf(errStartupInitiateComponent+": %s", e.SortedComponents[index], err.Error())
		}
	}
	// 模块启动
	for index, c := range sortedComponents {
		ci, ok := c.(ComponentInterface)
		if !ok {
			return fmt.Errorf(errStartupInjectComponent+": invalid '%s' component type", e.SortedComponents[index])
		}
		e.Context, err = ci.OnStartup(e.Context)
		if err != nil {
			return fmt.Errorf(errStartupInjectComponent+": %s", err.Error())
		}
	}
	// 注入模块
	err = e.InjectComponent()
	if err != nil {
		return fmt.Errorf(errStartupInjectComponent+": %s", err.Error())
	}
	e.state = STATE_STARTUP
	return nil
}

func (e *Engine) Shutdown() (err error) {
	if e.state != STATE_STARTUP {
		return nil
	}
	e.SystemLog("ltick: Shutdown")
	for _, sortedComponent := range e.GetSortedComponents(true) {
		component, ok := sortedComponent.(ComponentInterface)
		if !ok {
			e.SystemLog("ltick: Shutdown component error: invalid component type")
		}
		e.Context, err = component.OnShutdown(e.Context)
		if err != nil {
			e.SystemLog("ltick: Shutdown component error: " + err.Error())
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

// Register As Component
func (e *Engine) RegisterComponent(componentName string, component ComponentInterface, forceOverwrites ...bool) (err error) {
	e.Context, err = e.registerComponent(e.Context, componentName, component, forceOverwrites...)
	if err != nil {
		return fmt.Errorf(errRegisterComponent+": %s", err.Error())
	}
	return nil
}

// Unregister As Component
func (e *Engine) UnregisterComponent(componentNames ...string) (err error) {
	e.Context, err = e.unregisterComponent(e.Context, componentNames...)
	if err != nil {
		return fmt.Errorf(errUnregisterComponent+": %s [components:'%v']", err.Error(), componentNames)
	}
	return nil
}
