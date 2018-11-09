package ltick

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/api"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-log"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var configs map[string]config.Option = map[string]config.Option{
	"APP_ENV":     config.Option{Type: config.String, Default: "local", EnvironmentKey: "APP_ENV"},
	"PREFIX_PATH": config.Option{Type: config.String, EnvironmentKey: "PREFIX_PATH"},
	"TMP_PATH":    config.Option{Type: config.String, Default: "/tmp"},
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

var GetLogContext = func(ctx context.Context) (forwardRequestId string, requestId string, clientIP string, serverAddress string) {
	if ctx.Value("forwardRequestId") != nil {
		forwardRequestId = ctx.Value("forwardRequestId").(string)
	}
	if ctx.Value("requestId") != nil {
		requestId = ctx.Value("requestId").(string)
	}
	if ctx.Value("clientIP") != nil {
		clientIP = ctx.Value("clientIP").(string)
	}
	if ctx.Value("serverAddress") != nil {
		serverAddress = ctx.Value("serverAddress").(string)
	}
	return forwardRequestId, requestId, clientIP, serverAddress
}

var infoLogFunc utility.LogFunc = func(ctx context.Context, format string, data ...interface{}) {
	ctxAppLogger := ctx.Value("appLogger")
	if ctxAppLogger == nil {
		return
	}
	appLogger, ok := ctxAppLogger.(*log.Logger)
	if !ok {
		return
	}
	forwardRequestId, requestId, _, serverAddress := GetLogContext(ctx)
	logData := make([]interface{}, len(data)+3)
	logData[0] = forwardRequestId
	logData[1] = requestId
	logData[2] = serverAddress
	copy(logData[3:], data)
	appLogger.Info("TEST|%s|%s|%s|"+format, logData...)
}
var systemLogFunc utility.LogFunc = func(ctx context.Context, format string, data ...interface{}) {
	ctxSystemLogger := ctx.Value("systemLogger")
	if ctxSystemLogger == nil {
		return
	}
	systemLogger, ok := ctxSystemLogger.(*log.Logger)
	if !ok {
		return
	}
	systemLogger.Info(format, data...)
}
var debugLogFunc utility.LogFunc = func(ctx context.Context, format string, data ...interface{}) {
	ctxAppLogger := ctx.Value("appLogger")
	if ctxAppLogger == nil {
		return
	}
	appLogger, ok := ctxAppLogger.(*log.Logger)
	if !ok {
		return
	}
	forwardRequestId, requestId, _, serverAddress := GetLogContext(ctx)
	logData := make([]interface{}, len(data)+3)
	logData[0] = forwardRequestId
	logData[1] = requestId
	logData[2] = serverAddress
	copy(logData[3:], data)
	appLogger.Debug("TEST|%s|%s|%s|"+format, logData...)
}
var accessLogFunc access.LogWriterFunc = func(c *routing.Context, rw *access.LogResponseWriter, elapsed float64) {
	ctxAppLogger := c.Context.Value("appLogger")
	if ctxAppLogger == nil {
		return
	}
	appLogger, ok := ctxAppLogger.(*log.Logger)
	if !ok {
		return
	}
	ctxAccessLogger := c.Context.Value("accessLogger")
	if ctxAccessLogger == nil {
		return
	}
	accessLogger, ok := ctxAccessLogger.(*log.Logger)
	if !ok {
		return
	}
	//来源请求ID
	forwardRequestId := c.Get("uniqid")
	//请求ID
	requestId := c.Get("requestId")
	//客户端IP
	clientIP := c.Get("clientIP")
	//服务端IP
	serverAddress := c.Get("serverAddress")
	requestLine := fmt.Sprintf("%s %s %s", c.Request.Method, c.Request.RequestURI, c.Request.Proto)
	debug := new(bool)
	if c.Get("DEBUG") != nil {
		*debug = c.Get("DEBUG").(bool)
	}
	if *debug {
		appLogger.Info(`TEST_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, forwardRequestId, requestId, serverAddress, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
	} else {
		appLogger.Info(`TEST_ACCESS|%s|%s|%s|%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, forwardRequestId, requestId, serverAddress, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
	}
	if *debug {
		accessLogger.Info(`%s - %v [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
	} else {
		accessLogger.Info(`%s - %v [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
	}
}

type ServerAppCallback struct{}

func (f *ServerAppCallback) OnStartup(e *Engine) error {
	return nil
}
func (f *ServerAppCallback) OnShutdown(e *Engine) error {
	return nil
}

type ServerRequestCallback struct{}

func (f *ServerRequestCallback) OnRequestStartup(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*log.Logger)
	systemLogger.Info("OnRequestStartup")
	return nil
}

func (f *ServerRequestCallback) OnRequestShutdown(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*log.Logger)
	systemLogger.Info("OnRequestShutdown")
	return nil
}

type ServerGroupRequestCallback struct{}

func (f *ServerGroupRequestCallback) OnRequestStartup(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*log.Logger)
	systemLogger.Info("OnGroupRequestStartup")
	return nil
}

func (f *ServerGroupRequestCallback) OnRequestShutdown(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*log.Logger)
	systemLogger.Info("OnGroupRequestShutdown")
	return nil
}

type TestServerSuite struct {
	suite.Suite
	configFile string
	dotenvFile string
	testAppLog string

	engine        *Engine
	DefaultServer *Server
	Server        *Server
}

func (suite *TestServerSuite) SetupTest() {
	var err error
	suite.configFile, err = filepath.Abs("testdata/ltick.json")
	assert.Nil(suite.T(), err)
	suite.dotenvFile, err = filepath.Abs("testdata/.env")
	assert.Nil(suite.T(), err)
	suite.testAppLog, err = filepath.Abs("testdata/app.log")
	assert.Nil(suite.T(), err)
	var options map[string]config.Option = map[string]config.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	registry, err := NewRegistry()
	assert.Nil(suite.T(), err)
	configComponent, err := registry.GetComponentByName("Config")
	assert.Nil(suite.T(), err)
	configer, ok := configComponent.(*config.Config)
	assert.True(suite.T(), ok)
	err = configer.SetOptions(options)
	assert.Nil(suite.T(), err)
	suite.engine = New(suite.configFile, suite.dotenvFile, "LTICK", registry, configs).
		WithCallback(&ServerAppCallback{}).WithValues(values)
	/*
	.WithLoggers([]*LogHanlder{
		&LogHanlder{Name: "access", Formatter: log.FormatterRaw, Type: log.TypeConsole, Writer: log.WriterStdout, MaxLevel: log.LevelDebug},
		&LogHanlder{Name: "app", Formatter: log.FormatterDefault, Type: log.TypeFile, Filename: suite.testAppLog, MaxLevel: log.LevelInfo},
		&LogHanlder{Name: "system", Formatter: log.FormatterSys, Type: log.TypeConsole, Writer: log.WriterStdout, MaxLevel: log.LevelInfo},
	})
	*/
	suite.engine.SetLogWriter(ioutil.Discard)
	accessLogger, err := suite.engine.GetLogger("access")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), accessLogger)
	appLogger, err := suite.engine.GetLogger("app")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), appLogger)
	systemLogger, err := suite.engine.GetLogger("system")
	fmt.Println(err)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), systemLogger)
	suite.engine.SetContextValue("systemLogger", systemLogger)
	suite.engine.SetContextValue("appLogger", appLogger)
	suite.engine.SetContextValue("accessLogger", accessLogger)
	suite.engine.SetContextValue("Foo", "Bar")
	// Default Server
	suite.DefaultServer = NewDefaultServer(suite.engine, &ServerRequestCallback{})
	suite.engine.SetServer("default", suite.DefaultServer)
	// Server
	errorLogHandler := func(c *routing.Context, err error) error {
		ctxAppLogger := c.Context.Value("appLogger")
		if ctxAppLogger == nil {
			return errors.New("miss app logger")
		}
		appLogger, ok := ctxAppLogger.(*log.Logger)
		if !ok {
			return errors.New("invalid app logger")
		}
		appLogger.Info(`TEST|%s|%s|%s|%s|%s`, c.Get("forwardRequestId"), c.Get("requestId"), c.Get("serverAddress"), err.Error(), c.Get("errorStack"))
		return err
	}

	recoveryHandler := func(c *routing.Context, err error) error {
		if httpError, ok := err.(routing.HTTPError); ok {
			return routing.NewHTTPError(httpError.StatusCode(), httpError.Error())
		} else {
			return routing.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	router := &ServerRouter{
		Router: routing.New(suite.engine.Context).Timeout(3 * time.Second),
		routes: make([]*ServerRouterRoute, 0),
		proxys: make([]*ServerRouterProxy, 0),
	}
	assert.NotNil(suite.T(), router)
	router.WithAccessLogger(accessLogFunc).
		WithErrorHandler(systemLogger.Error, errorLogHandler).
		WithRecoveryHandler(systemLogger.Emergency, recoveryHandler).
		WithPanicLogger(systemLogger.Emergency).
		WithTypeNegotiator(JSON, XML, XML2, HTML).
		WithSlashRemover(http.StatusMovedPermanently).
		WithLanguageNegotiator("zh-CN", "en-US").
		WithCors(CorsAllowAll).
		WithCallback(&ServerRequestCallback{})
	suite.Server = NewServer(8080, 30*time.Second, router)
	suite.engine.SetServer("test", suite.Server)
}

func (suite *TestServerSuite) TestDefaultServer() {
	rg := suite.DefaultServer.GetRouteGroup("/")
	if rg == nil {
		rg = suite.DefaultServer.AddRouteGroup("/")
	}
	assert.NotNil(suite.T(), rg)
	rg.WithCallback(&ServerGroupRequestCallback{})
	rg.AddRoute("GET", "user/<id>", func(c *routing.Context) error {
		_, err := c.ResponseWriter.Write([]byte(c.Param("id")))
		return err
	})
	rg.AddRoute("GET", "test/<id>", func(c *routing.Context) error {
		_, err := c.ResponseWriter.Write([]byte(c.Param("id")))
		return err
	})
	err := suite.engine.Startup()
	assert.Nil(suite.T(), err)
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/user/1", nil)
	suite.DefaultServer.ServeHTTP(res, req)
	assert.Equal(suite.T(), "1", res.Body.String())

	res = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test/1", nil)
	suite.DefaultServer.ServeHTTP(res, req)
	assert.Equal(suite.T(), "1", res.Body.String())
}

func (suite *TestServerSuite) TestServer() {
	rg := suite.Server.GetRouteGroup("/")
	if rg == nil {
		rg = suite.Server.AddRouteGroup("/")
	}
	assert.NotNil(suite.T(), rg)
	rg.WithCallback(&ServerGroupRequestCallback{})
	rg.AddRoute("GET", "user/<id>", func(c *routing.Context) error {
		_, err := c.ResponseWriter.Write([]byte(c.Param("id")))
		return err
	})
	rg.AddRoute("GET", "Foo", func(c *routing.Context) error {
		_, err := c.ResponseWriter.Write([]byte(c.Context.Value("Foo").(string)))
		return err
	})
	err := suite.engine.Startup()
	assert.Nil(suite.T(), err)
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/user/1", nil)
	suite.Server.ServeHTTP(res, req)
	assert.Equal(suite.T(), "1", res.Body.String())

	res = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/Foo", nil)
	suite.Server.ServeHTTP(res, req)
	assert.Equal(suite.T(), "Bar", res.Body.String())
}

type Param struct {
	Id           int                   `param:"<in:path> <required> <name:id> <desc:ID> <range: 0:10>"`
	Title        string                `param:"<in:query> <notempty>"`
	Num          float32               `param:"<in:formData> <required> <name:n> <range: 0.1:10> <err: formData param 'n' must be number in 0.1~10>"`
	Paragraph    []string              `param:"<in:formData> <name:p> <len: 1:20> <regexp: ^[\\w&=]*$>"`
	Picture      *multipart.FileHeader `param:"<in:formData> <name:pic> <maxmb:30>"`
	Cookie       *http.Cookie          `param:"<in:cookie> <name:testCookie>"`
	CookieString string                `param:"<in:cookie> <name:testCookie>"`
}

var once sync.Once

// Implement the handler interface
func (p Param) Serve(ctx *api.Context) error {
	once.Do(func() {
		ctx.SetSession("123", "abc")
		ctx.SetCookie("SessionId", "123")
	})

	/*
		info, err := ctx.StoreFile("pic", true)
		if err == nil {
			return errors.Annotatef(err, "ctx.StoreFile: error [filename:'%s', url:'%s', size:'%d']", p.Picture.Filename, info.Url, info.Size)
		}
	*/
	return ctx.ResponseJSON(200,
		api.Map{
			"Struct Params":    p,
			"Additional Param": ctx.Param("additional"),
		}, true)
	// return ctx.String(200, "name=%v", name)
	return nil
}

// Doc returns the API's note, result or parameters information.
func (p Param) Doc() api.Doc {
	return api.Doc{
		Note: "param desc",
		Return: api.JSONMsg{
			Code: 1,
			Info: "success",
		},
		MoreParams: []api.APIParam{
			{
				Name:  "additional",
				In:    "path",
				Model: "a",
				Desc:  "defined by the `Doc()` method",
			},
		},
	}
}
func (suite *TestServerSuite) TestApi() {
	rg := suite.Server.GetRouteGroup("/")
	if rg == nil {
		rg = suite.Server.AddRouteGroup("/")
	}
	assert.NotNil(suite.T(), rg)
	apiHandler, err := api.ToAPIHandler(&Param{}, true)
	assert.Nil(suite.T(), err)
	rg.AddApiRoute("POST", "user/<id>", apiHandler)
	err = suite.engine.Startup()
	assert.Nil(suite.T(), err)
	// case 1
	res := httptest.NewRecorder()
	form := url.Values{}
	form.Add("n", "1")
	form.Add("p", "!#!")
	req, _ := http.NewRequest("POST", "/user/1?title=title", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	suite.Server.ServeHTTP(res, req)
	assert.Equal(suite.T(), http.StatusBadRequest, res.Code)
	assert.Equal(suite.T(), "{\"status\":400,\"message\":\"*ltick.Param|p|must be in a valid format\"}\n", res.Body.String())
	// case 2
	res = httptest.NewRecorder()
	form = url.Values{}
	form.Add("n", "1")
	form.Add("p", "abc=")
	req, _ = http.NewRequest("POST", "/user/1?title=title", strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	suite.Server.ServeHTTP(res, req)
	assert.Equal(suite.T(), http.StatusOK, res.Code)
	assert.Equal(suite.T(), "1", res.Body.String())

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	err = w.WriteField("n", "1")
	assert.Nil(suite.T(), err)
	err = w.WriteField("p", "abc=")
	assert.Nil(suite.T(), err)
	testPic, err := filepath.Abs("./testdata/lenna.bmp")
	assert.Nil(suite.T(), err)
	testFile, err := os.Open(testPic)
	fw, err := w.CreateFormFile("pic", testFile.Name())
	assert.Nil(suite.T(), err)
	_, err = io.Copy(fw, testFile)
	assert.Nil(suite.T(), err)
	w.Close()
	req, _ = http.NewRequest("POST", "/user/1?title=title", body)
	suite.Server.ServeHTTP(res, req)
	assert.Equal(suite.T(), http.StatusOK, res.Code)
	assert.Equal(suite.T(), "1", res.Body.String())
}

func TestTestServerSuite(t *testing.T) {
	suite.Run(t, new(TestServerSuite))
}
