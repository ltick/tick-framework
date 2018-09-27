package ltick

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ltick/tick-framework/module"
	libConfig "github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/logger"
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-log"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/access"
	"github.com/stretchr/testify/assert"
)

type ServerAppInitFunc struct{}

func (f *ServerAppInitFunc) OnStartup(e *Engine) error {
	loggerModule, err := e.GetBuiltinModule("logger")
	if err != nil {
		return err
	}
	logger, ok := loggerModule.(*logger.Logger)
	if !ok {
		return errors.New("logger type error")
	}
	configProviders := make(map[string]interface{}, 2)
	// register the target types to allow configuring Logger.Targets.
	configProviders["ConsoleTarget"] = log.NewConsoleTarget
	configProviders["FileTarget"] = log.NewFileTarget
	configPath, err := filepath.Abs("testdata/services.json")
	if err != nil {
		return err
	}
	err = logger.LoadModuleFileConfig(configPath, configProviders, "modules.Logger")
	if err != nil {
		return err
	}
	e.SetContextValue("testlogger", logger.NewLogger("test"))

	return nil
}
func (f *ServerAppInitFunc) OnShutdown(e *Engine) error {
	return nil
}

type ServerRequestInitFunc struct{}

func (f *ServerRequestInitFunc) OnRequestStartup(c *routing.Context) error {
	testlogger := c.Context.Value("testlogger").(*log.Logger)
	testlogger.Info("OnRequestStartup")
	return nil
}

func (f *ServerRequestInitFunc) OnRequestShutdown(c *routing.Context) error {
	testlogger := c.Context.Value("testlogger").(*log.Logger)
	testlogger.Info("OnRequestStartup")
	return nil
}

type ServerGroupRequestInitFunc struct{}

func (f *ServerGroupRequestInitFunc) OnRequestStartup(c *routing.Context) error {
	testlogger := c.Context.Value("testlogger").(*log.Logger)
	testlogger.Info("GroupOnRequestStartup")
	return nil
}

func (f *ServerGroupRequestInitFunc) OnRequestShutdown(c *routing.Context) error {
	testlogger := c.Context.Value("testlogger").(*log.Logger)
	testlogger.Info("GroupOnRequestStartup")
	return nil
}

func TestServerCallback(t *testing.T) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*module.Module = []*module.Module{}
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&ServerAppInitFunc{}).WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.Startup()
	assert.Nil(t, err)
	assert.NotNil(t, a.Context.Value("testlogger"))
	utilityModule, err := a.GetBuiltinModule("utility")
	assert.Nil(t, err)
	a.SetContextValue("utility", utilityModule)
	accessLogFunc := func(c *routing.Context, rw *access.LogResponseWriter, elapsed float64) {
		testlogger := c.Context.Value("testlogger").(*log.Logger)
		clientIP := utility.GetClientIP(c.Request)
		requestLine := fmt.Sprintf("%s %s %s", c.Request.Method, c.Request.URL.String(), c.Request.Proto)
		testlogger.Info(`%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s "-" "-"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr)
	}
	errorLogHandler := func(c *routing.Context, err error) error {
		testlogger := c.Context.Value("testlogger").(*log.Logger)
		testlogger.Info(`%s|%s|%s|%s|%s|%s`, time.Now().Format("2016-04-20T17:40:12+08:00"), log.LevelError, "", c.Get("c.RequestuestId"), err.Error(), c.Get("errorStack"))
		return nil
	}
	// server
	testlogger := a.Context.Value("testlogger").(*log.Logger)
	a.SetContextValue("Foo", "Bar")
	a.NewServer("test", 8080, 30*time.Second, 3*time.Second)
	s := a.GetServer("test")
	r := s.Router.WithAccessLogger(accessLogFunc).
		WithErrorHandler(testlogger.Error, errorLogHandler).
		WithPanicLogger(testlogger.Emergency).
		WithTypeNegotiator(JSON, XML, XML2, HTML).
		WithSlashRemover(http.StatusMovedPermanently).
		WithLanguageNegotiator("zh-CN", "en-US").
		WithCors(CorsAllowAll).
		WithCallback(&ServerRequestInitFunc{})
	assert.NotNil(t, r)
	rg := s.GetRouteGroup("/")
	assert.NotNil(t, rg)
	rg.WithCallback(&ServerGroupRequestInitFunc{})
	rg.AddRoute("GET", "/test/<id>", func(c *routing.Context) error {
		c.ResponseWriter.Write([]byte(c.Param("id")))
		return nil
	})
	rg.AddRoute("GET", "/Foo", func(c *routing.Context) error {
		c.ResponseWriter.Write([]byte(c.Context.Value("Foo").(string)))
		return nil
	})

	a.SetSystemLogWriter(ioutil.Discard)
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test/1", nil)
	a.ServeHTTP(res, req)
	assert.Equal(t, "1", res.Body.String())

	res = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/Foo", nil)
	a.ServeHTTP(res, req)
	assert.Equal(t, "Bar", res.Body.String())
}
