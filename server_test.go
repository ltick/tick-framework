package ltick

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/logger"
	"github.com/ltick/tick-framework/utility"
	libLog "github.com/ltick/tick-log"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/stretchr/testify/assert"
)

var infoLogFunc utility.LogFunc
var systemLogFunc utility.LogFunc
var debugLogFunc utility.LogFunc
var traceLogFunc utility.LogFunc
var accessLogFunc access.LogWriterFunc

type ServerAppCallback struct{}

func (f *ServerAppCallback) OnStartup(e *Engine) error {
	return nil
}
func (f *ServerAppCallback) OnShutdown(e *Engine) error {
	return nil
}

type ServerRequestCallback struct{}

func (f *ServerRequestCallback) OnRequestStartup(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*libLog.Logger)
	systemLogger.Info("OnRequestStartup")
	return nil
}

func (f *ServerRequestCallback) OnRequestShutdown(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*libLog.Logger)
	systemLogger.Info("OnRequestStartup")
	return nil
}

type ServerGroupRequestCallback struct{}

func (f *ServerGroupRequestCallback) OnRequestStartup(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*libLog.Logger)
	systemLogger.Info("OnRequestStartup")
	return nil
}

func (f *ServerGroupRequestCallback) OnRequestShutdown(c *routing.Context) error {
	systemLogger := c.Context.Value("systemLogger").(*libLog.Logger)
	systemLogger.Info("OnRequestStartup")
	return nil
}

func TestServerCallback(t *testing.T) {
	var testAppLog string
	testAppLog, _ = filepath.Abs("testdata/app.log")
	var options map[string]config.Option = map[string]config.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var components []*Component = []*Component{}
	configFile, err := filepath.Abs("testdata/ltick.json")
	assert.Nil(t, err)
	dotenvFile, err := filepath.Abs("testdata/.env")
	assert.Nil(t, err)
	registry, err := NewRegistry(components...)
	assert.Nil(t, err)
	r, err := NewRegistry(components...)
	assert.Nil(t, err)
	configComponent, err := r.GetComponentByName("Config")
	assert.Nil(t, err)
	assert.NotNil(t, configComponent)
	configer, ok := configComponent.(*config.Config)
	assert.True(t, ok)
	err = configer.SetOptions(options)
	assert.Nil(t, err)

	a := New(configFile, dotenvFile, "LTICK", registry).
		WithCallback(&ServerAppCallback{}).WithValues(values).WithLoggers([]*LogHanlder{
		&LogHanlder{Name: "access", Formatter: log.FormatterRaw, Type: log.TypeConsole, Writer: log.WriterStdout, MaxLevel: log.LevelDebug},
		&LogHanlder{Name: "app", Formatter: log.FormatterDefault, Type: log.TypeFile, Filename: testAppLog, MaxLevel: log.LevelInfo},
		&LogHanlder{Name: "system", Formatter: log.FormatterSys, Type: log.TypeConsole, Writer: log.WriterStdout, MaxLevel: log.LevelInfo},
	})
	a.SetSystemLogWriter(ioutil.Discard)
	accessLogger, err := a.GetLogger("access")
	assert.Nil(t, err)
	assert.NotNil(t, accessLogger)
	appLogger, err := a.GetLogger("app")
	assert.Nil(t, err)
	assert.NotNil(t, appLogger)
	systemLogger, err := a.GetLogger("system")
	assert.Nil(t, err)
	assert.NotNil(t, systemLogger)
	a.SetContextValue("systemLogger", systemLogger)
	GetLogContext := func(ctx context.Context) (forwardRequestId string, requestId string, clientIP string, serverAddress string) {
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
	systemLogFunc = func(ctx context.Context, format string, data ...interface{}) {
		if systemLogger == nil {
			return
		}
		systemLogger.Info(format, data...)
	}
	debugLogFunc = func(ctx context.Context, format string, data ...interface{}) {
		if appLogger == nil {
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
	infoLogFunc = func(ctx context.Context, format string, data ...interface{}) {
		if appLogger == nil {
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
	accessLogFunc = func(c *routing.Context, rw *access.LogResponseWriter, elapsed float64) {
		if appLogger == nil || accessLogger == nil {
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
			accessLogger.Info(`%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "%v" "%v"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress, c.Request.Header, rw.Header())
		} else {
			accessLogger.Info(`%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s %s "-" "-"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr, serverAddress)
		}
	}
	errorLogHandler := func(c *routing.Context, err error) error {
		appLogger.Info(`TEST|%s|%s|%s|%s|%s`, c.Get("forwardRequestId"), c.Get("requestId"), c.Get("serverAddress"), err.Error(), c.Get("errorStack"))
		return nil
	}
	// server
	a.SetContextValue("Foo", "Bar")
	router := &ServerRouter{
		Router: routing.New(a.Context).Timeout(3*time.Second),
		routes: make([]*ServerRouterRoute, 0),
		proxys: make([]*ServerRouterProxy, 0),
	}
	router.WithAccessLogger(accessLogFunc).
		WithErrorHandler(systemLogger.Error, errorLogHandler).
		WithPanicLogger(systemLogger.Emergency).
		WithTypeNegotiator(JSON, XML, XML2, HTML).
		WithSlashRemover(http.StatusMovedPermanently).
		WithLanguageNegotiator("zh-CN", "en-US").
		WithCors(CorsAllowAll).
		WithCallback(&ServerRequestCallback{})
	a.NewServer("test", 8080, 30*time.Second, router)
	s := a.GetServer("test")

	assert.NotNil(t, router)
	rg := s.GetRouteGroup("/").WithCallback(&ServerGroupRequestCallback{})
	assert.NotNil(t, rg)
	//rg.WithCallback(&ServerGroupRequestCallback{})
	rg.AddRoute("GET", "test/<id>", func(c *routing.Context) error {
		c.ResponseWriter.Write([]byte(c.Param("id")))
		return nil
	})
	rg.AddRoute("GET", "Foo", func(c *routing.Context) error {
		c.ResponseWriter.Write([]byte(c.Context.Value("Foo").(string)))
		return nil
	})
	err = a.Startup()
	assert.Nil(t, err)
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test/1", nil)
	a.ServeHTTP(res, req)
	assert.Equal(t, "1", res.Body.String())

	res = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/Foo", nil)
	a.ServeHTTP(res, req)
	assert.Equal(t, "Bar", res.Body.String())
}
