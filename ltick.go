package ltick

import (
	"bytes"
	"context"
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

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/logger"
	"github.com/ltick/tick-framework/utility"
	libLogger "github.com/ltick/tick-log"
	"github.com/ltick/tick-graceful"
)

var (
	errNew                       = "ltick: new error"
	errNewClassic                = "ltick: new classic error"
	errNewServer                 = "ltick: new server error"
	errGetLogger                 = "ltick: get logger error"
	errWithValues                = "ltick: with values error [key:'%s']"
	errWithLoggers               = "ltick: with loggers error [log_name:'%s', log_file:'%s']"
	errStartupCallback           = "ltick: startup callback error"
	errStartupRouterCallback     = "ltick: startup router callback error"
	errStartupRouteGroupCallback = "ltick: startup route group callback error"
	errStartupInjectComponent    = "ltick: startup inject component error"
	errStartupComponentInitiate  = "ltick: startup component '%s' initiate error"
	errStartupComponentStartup   = "ltick: startup component '%s' startup error"
	errShutdownCallback          = "ltick: shutdown callback error"
	errShutdownComponentShutdown = "ltick: shutdown component '%s' shutdown error"
	errSetConfigOptions          = "ltick: set config options error"
	errLoadCachedConfig          = "ltick: load cached config error"
	errLoadConfig                = "ltick: load config error [path:'%s', name:'%s']"
	errLoadEnv                   = "ltick: load env error [env_prefix:'%s', binded_environment_keys:'%v']"
	errLoadSystemConfig          = "ltick: load system config error"
	errLoadEnvFile               = "ltick: load env file error"
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
		Config           *config.Config `inject:true`
		Logger           *logger.Logger `inject:true`

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
		Formatter logger.Formatter
		Type      logger.Type
		Filename  string
		Writer    logger.Writer // the writer name of writer (stdout, stderr, discard)
		MaxLevel  logger.Level
	}
)

var defaultConfigPath = "etc/ltick.json"
var defaultConfigReloadTime = 120 * time.Second
var configPlaceholdRegExp = regexp.MustCompile(`%\w+%`)

func NewClassic(components []*Component, configOptions map[string]config.Option, option *Option) (engine *Engine) {
	executeFile, err := exec.LookPath(os.Args[0])
	if err != nil {
		e := errors.Annotate(err, errNewClassic)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	executePath, err := filepath.Abs(executeFile)
	if err != nil {
		e := errors.Annotate(err, errNewClassic)
		fmt.Println(errors.ErrorStack(e))
		return nil
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
				loggerTargetMaxLevel := logger.LevelDebug
				for level, levelName := range logger.LevelNames {
					if levelName == loggerTarget["maxlevel"] {
						loggerTargetMaxLevel = level
						break
					}
				}
				switch logger.StringToType(loggerTargetType) {
				case logger.TypeFile:
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
						Type:      logger.TypeFile,
						Formatter: logger.StringToFormatter(loggerTargetFormatter),
						Filename:  loggerTargetFilename,
						MaxLevel:  loggerTargetMaxLevel,
					})
				case logger.TypeConsole:
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
						Type:      logger.TypeConsole,
						Formatter: logger.StringToFormatter(loggerTargetFormatter),
						Writer:    logger.StringToWriter(loggerTargetWriter),
						MaxLevel:  loggerTargetMaxLevel,
					})
				}
			}
		}
	}
	engine.WithLoggers(logHandlers)
	return engine
}

func New(executeFile string, pathPrefix string, configPath string, envPrefix string, components []*Component, configOptions map[string]config.Option) (e *Engine) {
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
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
	}
	// 模块初始化
	for _, c := range e.ComponentMap {
		ci, ok := c.(ComponentInterface)
		if !ok {
			e := errors.Annotate(errors.Errorf("invalid type"), errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
	}
	configComponent, err := e.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	config, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	e.Config = config
	loggerComponent, err := e.GetComponentByName("Logger")
	if err != nil {
		e := errors.Annotate(err, errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	logger, ok := loggerComponent.(*logger.Logger)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Logger' component type"), errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	e.Logger = logger
	e.SetPathPrefix(pathPrefix)
	e.Context, err = e.Config.SetOptions(e.Context, configOptions)
	if err != nil {
		e := errors.Annotate(err, errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
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
	for _, c := range components {
		err = e.RegisterComponent(c.Name, c.Component, true)
		if err != nil {
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
	}
	// 模块初始化
	for name, c := range e.ComponentMap {
		ci, ok := c.(ComponentInterface)
		if !ok {
			e := errors.Annotate(errors.Errorf("invalid type"), errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
		e.LoadComponentFileConfig(name, configPath, make(map[string]interface{}), "component."+name)
	}
	return e
}
func (e *Engine) LoadSystemConfig(configFilePath string, envPrefix string, dotEnvFiles ...string) *Engine {
	var dotEnvFile string
	if len(dotEnvFiles) > 0 {
		dotEnvFile = dotEnvFiles[0]
	}
	configCachedFile, err := utility.GetCachedFile(configFilePath)
	if err != nil {
		e := errors.Annotate(err, errLoadSystemConfig)
		fmt.Println(errors.ErrorStack(e))
		return nil
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
				e := errors.Annotate(err, errLoadSystemConfig)
				fmt.Println(errors.ErrorStack(e))
				return
			}
			if dotEnvFile != "" {
				dotEnvFileInfo, err := os.Stat(dotEnvFile)
				if err != nil {
					e := errors.Annotate(err, errLoadSystemConfig)
					fmt.Println(errors.ErrorStack(e))
					return
				}
				if cachedConfigFileInfo.ModTime().Before(dotEnvFileInfo.ModTime()) {
					e.LoadEnvFile(envPrefix, dotEnvFile)
					e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
				}
			}
			configFileInfo, err := os.Stat(configFilePath)
			if err != nil {
				e := errors.Annotate(err, errLoadSystemConfig)
				fmt.Println(errors.ErrorStack(e))
				return
			}
			if cachedConfigFileInfo.ModTime().Before(configFileInfo.ModTime()) {
				e.LoadCachedConfig(configFilePath, cachedConfigFilePath)
			}
			time.Sleep(defaultConfigReloadTime)
		}
	}()
	return e
}
func (e *Engine) SetConfigOptions(configOptions map[string]config.Option) (err error) {
	e.Context, err = e.Config.SetOptions(e.Context, configOptions)
	if err != nil {
		e := errors.Annotate(err, errSetConfigOptions)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	return nil
}
func (e *Engine) LoadCachedConfig(configFilePath string, cachedConfigFilePath string) {
	configFile, err := os.OpenFile(configFilePath, os.O_RDONLY, 0644)
	if err != nil {
		e := errors.Annotate(err, errLoadCachedConfig)
		fmt.Println(errors.ErrorStack(e))
		return
	}
	defer configFile.Close()
	cachedFileByte, err := ioutil.ReadAll(configFile)
	if err != nil {
		e := errors.Annotate(err, errLoadCachedConfig)
		fmt.Println(errors.ErrorStack(e))
		return
	}
	matches := configPlaceholdRegExp.FindAll(cachedFileByte, -1)
	for _, match := range matches {
		replaceKey := string(match)
		replaceConfigKey := strings.Trim(replaceKey, "%")
		cachedFileByte = bytes.Replace(cachedFileByte, []byte(replaceKey), []byte(e.Config.GetString(replaceConfigKey)), -1)
	}
	err = ioutil.WriteFile(cachedConfigFilePath, cachedFileByte, 0644)
	if err != nil {
		e := errors.Annotate(err, errLoadCachedConfig)
		fmt.Println(errors.ErrorStack(e))
		return
	}
	e.LoadConfig(filepath.Dir(cachedConfigFilePath), strings.Replace(filepath.Base(cachedConfigFilePath), filepath.Ext(cachedConfigFilePath), "", 1))
}
func (e *Engine) LoadConfig(configPath string, configName string) *Engine {
	var err error
	if configPath == "" || configName == "" {
		e := errors.Annotatef(errors.Errorf("config_path or config_name is empty"), errLoadConfig, configPath, configPath)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	if !strings.HasPrefix(configPath, "/") {
		configPath = strings.TrimRight(configPath, "/") + "/" + configPath
	}
	_, err = os.Stat(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			e := errors.Annotatef(err, errLoadConfig, configPath, configPath)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
	}
	e.Config.AddConfigPath(configPath)
	err = e.Config.LoadFromConfigPath(configName)
	if err != nil {
		e := errors.Annotatef(err, errLoadConfig, configPath, configPath)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	return e
}
func (e *Engine) LoadEnv(envPrefix string) *Engine {
	e.Config.SetEnvPrefix(envPrefix)
	err := e.Config.LoadFromEnv()
	if err != nil {
		if !os.IsNotExist(err) {
			e := errors.Annotatef(err, errLoadEnv, envPrefix, e.Config.BindedEnvironmentKeys())
			fmt.Println(errors.ErrorStack(e))
			return nil
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
		e := errors.Annotatef(err, errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	return e
}
func (e *Engine) WithValues(values map[string]interface{}) *Engine {
	for key, value := range values {
		err := e.RegisterValue(key, value)
		if err != nil {
			e := errors.Annotatef(err, errWithValues, key)
			fmt.Println(errors.ErrorStack(e))
			return nil
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
		return nil, errors.Annotatef(err, errGetLogger)
	}
	return logger, nil
}
func (e *Engine) WithLoggers(handlers []*LogHanlder) *Engine {
	var logProviders map[string]interface{} = make(map[string]interface{})
	logTargetProviderConfigs := make([]string, len(handlers))
	for index, hanlder := range handlers {
		switch hanlder.Type {
		case logger.TypeFile:
			logFilename := hanlder.Filename
			if !strings.HasPrefix(logFilename, "/") {
				logFilename = e.option.PathPrefix + "/" + logFilename
			}
			_, err := os.Stat(logFilename)
			if err != nil {
				if os.IsNotExist(err) {
					_, err = os.Create(logFilename)
					if err != nil {
						e := errors.Annotatef(err, errWithLoggers, hanlder.Name, logFilename)
						fmt.Println(errors.ErrorStack(e))
						return nil
					}
				} else {
					e := errors.Annotatef(err, errWithLoggers, hanlder.Name, logFilename)
					fmt.Println(errors.ErrorStack(e))
					return nil
				}
			}
			logTargetProviderName := hanlder.Name + "FileTarget"
			logProviders[logTargetProviderName] = logger.NewFileTarget
			logTargetProviderConfigs[index] = `"` + hanlder.Name + `":{"type": "` + logTargetProviderName + `","Filename":"` + logFilename + `","Rotate":true,"MaxBytes":` + strconv.Itoa(1<<22) + `}`
			e.SystemLog("ltick: register log [name: '" + hanlder.Name + "', target: 'file', file: '" + logFilename + "']")
		case logger.TypeConsole:
			logWriter := hanlder.Writer
			logTargetProviderName := hanlder.Name + "ConsoleTarget"
			logProviders[logTargetProviderName] = logger.NewConsoleTarget
			logTargetProviderConfigs[index] = `"` + hanlder.Name + `":{"type": "` + logTargetProviderName + `","Writer":"` + logWriter.String() + `"}`
			index++
			e.SystemLog("ltick: register log [name: '" + hanlder.Name + "', target:'console', writer:'" + logWriter.String() + "']")
		}
	}
	loggerConfig := `{`
	if len(logTargetProviderConfigs) > 0 {
		loggerConfig = loggerConfig + `"Targets": {` + strings.Join(logTargetProviderConfigs, ",") + `}`
	}
	loggerConfig = loggerConfig + `}`
	e.Config.LoadComponentJsonConfig(e.Logger, "Logger", []byte(loggerConfig), logProviders)
	for _, hanlder := range handlers {
		e.Logger.NewLogger(hanlder.Name)
		switch hanlder.Formatter {
		case logger.FormatterRaw:
			e.Logger.SetLoggerFormatter(hanlder.Name, logger.RawLogFormatter())
		case logger.FormatterSys:
			e.Logger.SetLoggerFormatter(hanlder.Name, logger.SysLogFormatter())
		case logger.FormatterDefault:
			e.Logger.SetLoggerFormatter(hanlder.Name, logger.DefaultLogFormatter())
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
			return errors.Annotatef(err, errStartupCallback)
		}
		err = e.callback.OnStartup(e)
		if err != nil {
			return errors.Annotatef(err, errStartupCallback)
		}
	}
	for _, server := range e.Servers {
		if server.Router.routes != nil && len(server.Router.routes) > 0 {
			for _, route := range server.Router.routes {
				server.AddRoute(route.Method, route.Host, route.Handlers...)
			}
		}
		// proxy
		if server.Router.proxys != nil && len(server.Router.proxys) > 0 {
			server.AddRoute("ANY", "/")
		}
		if server.RouteGroups != nil {
			for _, routeGroup := range server.RouteGroups {
				if routeGroup.callback != nil {
					err = e.InjectComponentTo([]interface{}{routeGroup.callback})
					if err != nil {
						return errors.Annotatef(err, errStartupRouteGroupCallback)
					}
				}
			}
		}
		if server.Router.callback != nil {
			err = e.InjectComponentTo([]interface{}{server.Router.callback})
			if err != nil {
				return errors.Annotatef(err, errStartupRouterCallback)
			}
		}
	}
	sortedComponents := e.GetSortedComponents()
	// 模块初始化
	for index, c := range sortedComponents {
		ci, ok := c.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errStartupComponentInitiate, e.SortedComponents[index])
		}
		e.Context, err = ci.Initiate(e.Context)
		if err != nil {
			return errors.Annotatef(err, errStartupComponentInitiate, e.SortedComponents[index])
		}
	}
	// 模块启动
	for index, c := range sortedComponents {
		ci, ok := c.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errStartupComponentStartup, e.SortedComponents[index])
		}
		e.Context, err = ci.OnStartup(e.Context)
		if err != nil {
			return errors.Annotatef(err, errStartupComponentStartup, e.SortedComponents[index])
		}
	}
	// 注入模块
	err = e.InjectComponent()
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
	e.SystemLog("ltick: Shutdown")
	sortedComponents := e.GetSortedComponents()
	for index, c := range sortedComponents {
		component, ok := c.(ComponentInterface)
		if !ok {
			return errors.Annotatef(errors.Errorf("invalid type"), errShutdownComponentShutdown, e.SortedComponents[index])
		}
		e.Context, err = component.OnShutdown(e.Context)
		if err != nil {
			return errors.Annotatef(err, errShutdownComponentShutdown, e.SortedComponents[index])
		}
	}
	if e.callback != nil {
		err = e.callback.OnShutdown(e)
		if err != nil {
			return errors.Annotatef(err, errShutdownCallback)
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
			return
		}
	}
	e.SystemLog("ltick: Server stop listen ", server.Port, "...")
}

func (e *Engine) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// server
	if e.Servers != nil {
		serverCount := len(e.Servers)
		for _, server := range e.Servers {
			serverCount--
			if serverCount == 0 {
				e.ServerServeHTTP(server, res, req)
			} else {
				go e.ServerServeHTTP(server, res, req)
			}
		}
	} else {
		e.SystemLog("ltick: Server not set")
	}
}
func (e *Engine) ServerServeHTTP(server *Server, res http.ResponseWriter, req *http.Request) {
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
		return err
	}
	return nil
}

// Unregister As Component
func (e *Engine) UnregisterComponent(componentNames ...string) (err error) {
	e.Context, err = e.unregisterComponent(e.Context, componentNames...)
	if err != nil {
		return err
	}
	return nil
}
