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
	"github.com/ltick/tick-graceful"
	libLog "github.com/ltick/tick-log"
)

var (
	errNew                       = "ltick: new error"
	errNewDefault                = "ltick: new classic error"
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
	errLoadComponentFileConfig   = "ltick: load component file config error"
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

		Registry  *Registry
		Context   context.Context
		ServerMap map[string]*Server
	}
	Callback interface {
		OnStartup(*Engine) error  // Execute On After All Engine Component OnStartup
		OnShutdown(*Engine) error // Execute On After All Engine Component OnShutdown
	}
	LogHanlder struct {
		Name      string
		Formatter log.Formatter
		Type      log.Type
		Filename  string
		Writer    log.Writer // the writer name of writer (stdout, stderr, discard)
		MaxLevel  log.Level
	}
)

var defaultConfigPath = "etc/ltick.json"
var defaultDotenvPath = ".env"
var defaultConfigReloadTime = 120 * time.Second
var configPlaceholdRegExp = regexp.MustCompile(`%\w+%`)

func NewDefault(envPrefix string, registry *Registry, options map[string]config.Option) (engine *Engine) {
	defaultConfigFile, err := filepath.Abs(defaultConfigPath)
	if err != nil {
		e := errors.Annotatef(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	defaultDotenvFile, err := filepath.Abs(defaultDotenvPath)
	if err != nil {
		e := errors.Annotatef(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	engine = New(defaultConfigFile, defaultDotenvFile, envPrefix, registry)
	// configer
	configComponent, err := engine.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	err = configer.SetOptions(options)
	if err != nil {
		e := errors.Annotate(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	logHandlers := make([]*LogHanlder, 0)
	logTargetsConfig := configer.GetStringMap("components.log.targets")
	for logName, logTargetInterface := range logTargetsConfig {
		logTarget := logTargetInterface.(map[string]interface{})
		logTargetTypeInterface, ok := logTarget["type"]
		if ok {
			logTargetType, ok := logTargetTypeInterface.(string)
			if ok {
				logTargetMaxLevel := log.LevelDebug
				for level, levelName := range log.LevelNames {
					if levelName == logTarget["maxlevel"] {
						logTargetMaxLevel = level
						break
					}
				}
				switch log.StringToType(logTargetType) {
				case log.TypeFile:
					logTargetFormatterInterface, ok := logTarget["formatter"]
					if !ok {
						continue
					}
					logTargetFormatter, ok := logTargetFormatterInterface.(string)
					if !ok {
						continue
					}
					logTargetFilenameInterface, ok := logTarget["filename"]
					if !ok {
						continue
					}
					logTargetFilename, ok := logTargetFilenameInterface.(string)
					if !ok {
						continue
					}
					logHandlers = append(logHandlers, &LogHanlder{
						Name:      logName,
						Type:      log.TypeFile,
						Formatter: log.StringToFormatter(logTargetFormatter),
						Filename:  logTargetFilename,
						MaxLevel:  logTargetMaxLevel,
					})
				case log.TypeConsole:
					logTargetFormatterInterface, ok := logTarget["formatter"]
					if !ok {
						continue
					}
					logTargetFormatter, ok := logTargetFormatterInterface.(string)
					if !ok {
						continue
					}
					logTargetWriterInterface, ok := logTarget["writer"]
					if !ok {
						continue
					}
					logTargetWriter, ok := logTargetWriterInterface.(string)
					if !ok {
						continue
					}
					logHandlers = append(logHandlers, &LogHanlder{
						Name:      logName,
						Type:      log.TypeConsole,
						Formatter: log.StringToFormatter(logTargetFormatter),
						Writer:    log.StringToWriter(logTargetWriter),
						MaxLevel:  logTargetMaxLevel,
					})
				}
			}
		}
	}
	engine.WithLoggers(logHandlers)
	return engine
}

func New(configPath string, dotenvFile string, envPrefix string, registry *Registry) (e *Engine) {
	executeFile, err := exec.LookPath(os.Args[0])
	if err != nil {
		e := errors.Annotate(err, errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	e = &Engine{
		state:           STATE_INITIATE,
		executeFile:     executeFile,
		systemLogWriter: os.Stdout,
		Registry:        registry,
		Context:         context.Background(),
		ServerMap:       make(map[string]*Server, 0),
	}
	// 模块初始化
	componentMap := e.Registry.GetComponentMap()
	for _, name := range e.Registry.GetSortedComponentName() {
		ci, ok := componentMap[name].(ComponentInterface)
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
		e.Registry.LoadComponentFileConfig(name, configPath, make(map[string]interface{}), "component."+name)
	}
	// 中间件初始化
	for _, m := range e.Registry.GetMiddlewareMap() {
		mi, ok := m.(MiddlewareInterface)
		if !ok {
			e := errors.Annotate(errors.Errorf("invalid type"), errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
		e.Context, err = mi.Initiate(e.Context)
		if err != nil {
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
	}
	// 加载系统配置
	if dotenvFile != "" {
		e.LoadSystemConfig(configPath, envPrefix, dotenvFile)
	} else {
		e.LoadSystemConfig(configPath, envPrefix)
	}
	return e
}
func (e *Engine) LoadSystemConfig(configPath string, envPrefix string, dotenvFiles ...string) *Engine {
	if configPath == "" {
		e := errors.Annotate(fmt.Errorf("'%s' is a empty config path", configPath), errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	if !path.IsAbs(configPath) {
		e := errors.Annotate(fmt.Errorf("'%s' is not a valid config path", configPath), errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	var dotenvFile string
	if len(dotenvFiles) > 0 {
		dotenvFile = dotenvFiles[0]
	}
	if !path.IsAbs(dotenvFile) {
		e := errors.Annotate(fmt.Errorf("'%s' is not a valid dotenv path", dotenvFile), errNew)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	configCachedFile, err := utility.GetCachedFile(configPath)
	if err != nil {
		e := errors.Annotate(err, errLoadSystemConfig)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	defer configCachedFile.Close()
	cachedConfigFilePath := configCachedFile.Name()
	if dotenvFile != "" {
		e.LoadEnvFile(envPrefix, dotenvFile)
	} else {
		e.LoadEnv(envPrefix)
	}
	e.LoadCachedConfig(configPath, cachedConfigFilePath)
	go func() {
		// 刷新缓存
		for {
			cachedConfigFileInfo, err := os.Stat(cachedConfigFilePath)
			if err != nil {
				e := errors.Annotate(err, errLoadSystemConfig)
				fmt.Println(errors.ErrorStack(e))
				return
			}
			if dotenvFile != "" {
				dotenvFileInfo, err := os.Stat(dotenvFile)
				if err != nil {
					e := errors.Annotate(err, errLoadSystemConfig)
					fmt.Println(errors.ErrorStack(e))
					return
				}
				if cachedConfigFileInfo.ModTime().Before(dotenvFileInfo.ModTime()) {
					e.LoadEnvFile(envPrefix, dotenvFile)
					e.LoadCachedConfig(configPath, cachedConfigFilePath)
				}
			}
			configFileInfo, err := os.Stat(configPath)
			if err != nil {
				e := errors.Annotate(err, errLoadSystemConfig)
				fmt.Println(errors.ErrorStack(e))
				return
			}
			if cachedConfigFileInfo.ModTime().Before(configFileInfo.ModTime()) {
				e.LoadCachedConfig(configPath, cachedConfigFilePath)
			}
			time.Sleep(defaultConfigReloadTime)
		}
	}()
	return e
}

func (e *Engine) LoadCachedConfig(configPath string, cachedConfigFilePath string) {
	configFile, err := os.OpenFile(configPath, os.O_RDONLY, 0644)
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
	matches := configPlaceholdRegExp.FindAll(cachedFileByte, -1)
	for _, match := range matches {
		replaceKey := string(match)
		replaceConfigKey := strings.Trim(replaceKey, "%")
		cachedFileByte = bytes.Replace(cachedFileByte, []byte(replaceKey), []byte(configer.GetString(replaceConfigKey)), -1)
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
	configer.AddConfigPath(configPath)
	err = configer.LoadFromConfigPath(configName)
	if err != nil {
		e := errors.Annotatef(err, errLoadConfig, configPath, configPath)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	return e
}
func (e *Engine) LoadEnv(envPrefix string) *Engine {
	// configer
	configComponent, err := e.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errLoadEnv)
		fmt.Println(errors.ErrorStack(e))
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errLoadEnv)
		fmt.Println(errors.ErrorStack(e))
	}
	configer.SetEnvPrefix(envPrefix)
	err = configer.LoadFromEnv()
	if err != nil {
		if !os.IsNotExist(err) {
			e := errors.Annotatef(err, errLoadEnv, envPrefix, configer.BindedEnvironmentKeys())
			fmt.Println(errors.ErrorStack(e))
			return nil
		}
	}
	return nil
}
func (e *Engine) LoadEnvFile(envPrefix string, dotenvFile string) *Engine {
	// configer
	configComponent, err := e.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
	}
	configer.SetEnvPrefix(envPrefix)
	err = configer.LoadFromEnvFile(dotenvFile)
	if err != nil {
		e := errors.Annotatef(err, errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	return e
}
func (e *Engine) WithValues(values map[string]interface{}) *Engine {
	for key, value := range values {
		err := e.Registry.RegisterValue(key, value)
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
func (e *Engine) GetLogger(name string) (*libLog.Logger, error) {
	// log
	loggerComponent, err := e.Registry.GetComponentByName("Log")
	if err != nil {
		return nil, errors.Annotate(err, errGetLogger)
	}
	log, ok := loggerComponent.(*log.Logger)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Logger' component type"), errGetLogger)
		fmt.Println(errors.ErrorStack(e))
		return nil, errors.Annotate(err, errGetLogger)
	}
	logger, err := log.GetLogger(name)
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
		case log.TypeFile:
			logFilename, err := filepath.Abs(hanlder.Filename)
			if err != nil {
				e := errors.Annotatef(err, errWithLoggers, hanlder.Name, logFilename)
				fmt.Println(errors.ErrorStack(e))
				return nil
			}
			_, err = os.Stat(logFilename)
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
			logProviders[logTargetProviderName] = log.NewFileTarget
			logTargetProviderConfigs[index] = `"` + hanlder.Name + `":{"type": "` + logTargetProviderName + `","Filename":"` + logFilename + `","Rotate":true,"MaxBytes":` + strconv.Itoa(1<<22) + `}`
			e.SystemLog("ltick: register log [name: '" + hanlder.Name + "', target: 'file', file: '" + logFilename + "']")
		case log.TypeConsole:
			logWriter := hanlder.Writer
			logTargetProviderName := hanlder.Name + "ConsoleTarget"
			logProviders[logTargetProviderName] = log.NewConsoleTarget
			logTargetProviderConfigs[index] = `"` + hanlder.Name + `":{"type": "` + logTargetProviderName + `","Writer":"` + logWriter.String() + `"}`
			index++
			e.SystemLog("ltick: register log [name: '" + hanlder.Name + "', target:'console', writer:'" + logWriter.String() + "']")
		}
	}
	logConfig := `{`
	if len(logTargetProviderConfigs) > 0 {
		logConfig = logConfig + `"Targets": {` + strings.Join(logTargetProviderConfigs, ",") + `}`
	}
	logConfig = logConfig + `}`
	// configer
	configComponent, err := e.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errWithLoggers)
		fmt.Println(errors.ErrorStack(e))
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errWithLoggers)
		fmt.Println(errors.ErrorStack(e))
	}
	// logger
	loggerComponent, err := e.Registry.GetComponentByName("Log")
	if err != nil {
		e := errors.Annotate(err, errWithLoggers)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	logger, ok := loggerComponent.(*log.Logger)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Logger' component type"), errWithLoggers)
		fmt.Println(errors.ErrorStack(e))
		return nil
	}
	configer.LoadComponentJsonConfig(logger, "Log", []byte(logConfig), logProviders)
	for _, hanlder := range handlers {
		logger.NewLogger(hanlder.Name)
		switch hanlder.Formatter {
		case log.FormatterRaw:
			logger.SetLoggerFormatter(hanlder.Name, log.RawLogFormatter())
		case log.FormatterSys:
			logger.SetLoggerFormatter(hanlder.Name, log.SysLogFormatter())
		case log.FormatterDefault:
			logger.SetLoggerFormatter(hanlder.Name, log.DefaultLogFormatter())
		}
		logger.SetLoggerTarget(hanlder.Name, hanlder.Name)
		logger.SetLoggerMaxLevel(hanlder.Name, hanlder.MaxLevel)
		logger.OpenLogger(hanlder.Name)
	}
	return e
}
func (e *Engine) SetSystemLogWriter(systemLogWriter io.Writer) {
	e.systemLogWriter = systemLogWriter
}
func (e *Engine) SystemLog(args ...interface{}) {
	fmt.Fprintln(e.systemLogWriter, args...)
}
func (e *Engine) Startup() (err error) {
	if e.state != STATE_INITIATE {
		return nil
	}
	e.SystemLog("ltick: Execute file \"" + e.executeFile + "\"")
	e.SystemLog("ltick: Startup")
	if e.callback != nil {
		err = e.Registry.InjectComponentTo([]interface{}{e.callback})
		if err != nil {
			return errors.Annotatef(err, errStartupCallback)
		}
		err = e.callback.OnStartup(e)
		if err != nil {
			return errors.Annotatef(err, errStartupCallback)
		}
	}
	for _, server := range e.ServerMap {
		if server.Router.routes != nil && len(server.Router.routes) > 0 {
			for _, route := range server.Router.routes {
				server.AddRoute(route.Method, route.Host, route.Handlers...)
			}
		}
		// proxy
		if server.Router.proxys != nil && len(server.Router.proxys) > 0 {
			server.AddRoute("ANY", "/")
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
	e.SystemLog("ltick: Shutdown")
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
	if e.ServerMap != nil {
		serverCount := len(e.ServerMap)
		for _, server := range e.ServerMap {
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
