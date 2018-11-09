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
	errNewDefault                = "ltick: new classic error"
	errNewServer                 = "ltick: new server error"
	errGetLogger                 = "ltick: get logger error"
	errWithValues                = "ltick: with values error [key:'%s']"
	errWithLoggers               = "ltick: with loggers error [log_name:'%s', log_file:'%s']"
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
)

type State int8

const (
	STATE_INITIATE State = iota
	STATE_STARTUP
	STATE_SHUTDOWN
)

type (
	Engine struct {
		state       State
		executeFile string
		logWriter   io.Writer
		callback    Callback

		Registry  *Registry
		Context   context.Context
		ServerMap map[string]*Server
	}
	Callback interface {
		OnStartup(*Engine) error  // Execute On After All Engine Component OnStartup
		OnShutdown(*Engine) error // Execute On After All Engine Component OnShutdown
	}
)

var configPlaceholdRegExp = regexp.MustCompile(`%\w+%`)

func New(configPath string, dotenvFile string, envPrefix string, registry *Registry, options map[string]config.Option) (e *Engine) {
	executeFile, err := exec.LookPath(os.Args[0])
	if err != nil {
		e := errors.Annotate(err, errNew)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	e = &Engine{
		state:       STATE_INITIATE,
		executeFile: executeFile,
		logWriter:   os.Stdout,
		Registry:    registry,
		Context:     context.Background(),
		ServerMap:   make(map[string]*Server, 0),
	}
	// 注入模块
	err = e.Registry.InjectComponent()
	if err != nil {
		e := errors.Annotate(err, errNew)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	// 模块初始化
	componentMap := e.Registry.GetComponentMap()
	for _, name := range e.Registry.GetSortedComponentName() {
		ci, ok := componentMap[name].(ComponentInterface)
		if !ok {
			e := errors.Annotate(errors.Errorf("invalid type"), errNew)
			fmt.Println(errors.ErrorStack(e))
			os.Exit(1)
		}
		isBuiltinComponent := false
		canonicalName := strings.ToUpper(name[0:1]) + name[1:]
		for _, builtinComponent := range BuiltinComponents {
			if canonicalName == builtinComponent.Name {
				isBuiltinComponent = true
				break
			}
		}
		if !isBuiltinComponent {
			e.Context, err = ci.Initiate(e.Context)
			if err != nil {
				e := errors.Annotate(err, errNew)
				fmt.Println(errors.ErrorStack(e))
				os.Exit(1)
			}
		}
	}
	// 中间件初始化
	for _, m := range e.Registry.GetMiddlewareMap() {
		mi, ok := m.(MiddlewareInterface)
		if !ok {
			e := errors.Annotate(errors.Errorf("invalid type"), errNew)
			fmt.Println(errors.ErrorStack(e))
			os.Exit(1)
		}
		e.Context, err = mi.Initiate(e.Context)
		if err != nil {
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
			os.Exit(1)
		}
	}
	// configer
	configComponent, err := registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	err = configer.SetOptions(options)
	if err != nil {
		e := errors.Annotate(err, errNewDefault)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
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
		os.Exit(1)
	}
	if !path.IsAbs(configPath) {
		e := errors.Annotate(fmt.Errorf("'%s' is not a valid config path", configPath), errNew)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	var dotenvFile string
	if len(dotenvFiles) > 0 {
		dotenvFile = dotenvFiles[0]
	}
	if !path.IsAbs(dotenvFile) {
		e := errors.Annotate(fmt.Errorf("'%s' is not a valid dotenv path", dotenvFile), errNew)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	configCachedFile, err := utility.GetCachedFile(configPath)
	if err != nil {
		e := errors.Annotate(err, errLoadSystemConfig)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
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
		os.Exit(1)
	}
	if !strings.HasPrefix(configPath, "/") {
		configPath = strings.TrimRight(configPath, "/") + "/" + configPath
	}
	_, err = os.Stat(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			e := errors.Annotatef(err, errLoadConfig, configPath, configPath)
			fmt.Println(errors.ErrorStack(e))
			os.Exit(1)
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
		os.Exit(1)
	}
	for _, name := range e.Registry.GetSortedComponentName() {
		err = e.Registry.ConfigureFileConfig(name, configer.ConfigFileUsed(), make(map[string]interface{}))
		if err != nil {
			e := errors.Annotate(err, errNew)
			fmt.Println(errors.ErrorStack(e))
		}
	}
	return e
}
func (e *Engine) LoadEnv(envPrefix string) *Engine {
	// configer
	configComponent, err := e.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errLoadEnv)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errLoadEnv)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	configer.SetEnvPrefix(envPrefix)
	err = configer.LoadFromEnv()
	if err != nil {
		if !os.IsNotExist(err) {
			e := errors.Annotatef(err, errLoadEnv, envPrefix, configer.BindedEnvironmentKeys())
			fmt.Println(errors.ErrorStack(e))
			os.Exit(1)
		}
	}
	return e
}
func (e *Engine) LoadEnvFile(envPrefix string, dotenvFile string) *Engine {
	// configer
	configComponent, err := e.Registry.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	configer.SetEnvPrefix(envPrefix)
	err = configer.LoadFromEnvFile(dotenvFile)
	if err != nil {
		e := errors.Annotatef(err, errLoadEnvFile)
		fmt.Println(errors.ErrorStack(e))
		os.Exit(1)
	}
	return e
}
func (e *Engine) WithValues(values map[string]interface{}) *Engine {
	for key, value := range values {
		err := e.Registry.RegisterValue(key, value)
		if err != nil {
			e := errors.Annotatef(err, errWithValues, key)
			fmt.Println(errors.ErrorStack(e))
			os.Exit(1)
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
func (e *Engine) SetLogWriter(logWriter io.Writer) {
	e.logWriter = logWriter
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
				if _, ok := server.RouteGroups[route.Group]; !ok {
					server.RouteGroups[route.Group] = server.AddRouteGroup(route.Group)
				}
				server.RouteGroups[route.Group].AddRoute(route.Method, route.Path, func(c *routing.Context) error {
					if c.Request.Host == route.Host {
						for _, handler := range route.Handlers {
							err := handler(c)
							if err != nil {
								return err
							}
						}
					}
					return nil
				})
			}
		}
		// proxy
		if server.Router.proxys != nil && len(server.Router.proxys) > 0 {
			for _, proxy := range server.Router.proxys {
				if _, ok := server.RouteGroups[proxy.Group]; !ok {
					server.RouteGroups[proxy.Group] = server.AddRouteGroup(proxy.Group)
				}
				server.RouteGroups[proxy.Group].AddRoute("ANY", proxy.Path, func(c *routing.Context) error {
					upstreamURL, err := proxy.Proxy(c)
					fmt.Println(upstreamURL)
					fmt.Println(err)
					if err != nil {
						return routing.NewHTTPError(http.StatusInternalServerError, err.Error())
					}
					if upstreamURL != nil {
						fmt.Println("===")
						fmt.Println(upstreamURL)
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
		e.Log("ltick: Server not set")
	}
}

func (e *Engine) ServerListenAndServe(server *Server) {
	e.Log("ltick: Server start listen ", server.Port, "...")
	g := graceful.New().Server(
		&http.Server{
			Addr:    fmt.Sprintf(":%d", server.Port),
			Handler: server.Router,
		}).Timeout(server.gracefulStopTimeout).Build()
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
