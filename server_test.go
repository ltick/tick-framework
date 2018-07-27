package ltick

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ltick/tick-framework/module/logger"
	"github.com/ltick/tick-framework/module/utility"
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
	logger, ok := loggerModule.(*logger.Instance)
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
	e.WithContextValue("testlogger", logger.NewLogger("test"))

	return nil
}
func (f *ServerAppInitFunc) OnShutdown(e *Engine) error {
	err := e.unregisterModule("logger")
	if err != nil {
		return err
	}
	return nil
}

type ServerRequestInitFunc struct{}

func (f *ServerRequestInitFunc) OnRequestStartup(ctx context.Context, c *routing.Context) (context.Context, error) {
	testlogger := ctx.Value("testlogger").(*log.Logger)
	testlogger.Info("OnRequestStartup")
	return ctx, nil
}

func (f *ServerRequestInitFunc) OnRequestShutdown(ctx context.Context, c *routing.Context) (context.Context, error) {
	testlogger := ctx.Value("testlogger").(*log.Logger)
	testlogger.Info("OnRequestStartup")
	return ctx, nil
}

type ServerGroupRequestInitFunc struct{}

func (f *ServerGroupRequestInitFunc) OnRequestStartup(ctx context.Context, c *routing.Context) (context.Context, error) {
	testlogger := ctx.Value("testlogger").(*log.Logger)
	testlogger.Info("GroupOnRequestStartup")
	return ctx, nil
}

func (f *ServerGroupRequestInitFunc) OnRequestShutdown(ctx context.Context, c *routing.Context) (context.Context, error) {
	testlogger := ctx.Value("testlogger").(*log.Logger)
	testlogger.Info("GroupOnRequestStartup")
	return ctx, nil
}

func TestServerCallback(t *testing.T) {
	a := New(&ServerAppInitFunc{})
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.Startup()
	assert.Nil(t, err)
	assert.NotNil(t, a.Context.Value("testlogger"))
	utilityModule, err := a.GetBuiltinModule("utility")
	assert.Nil(t, err)
	a.WithContextValue("utility",utilityModule)
	accessLogFunc := func(ctx context.Context, c *routing.Context, rw *access.LogResponseWriter, elapsed float64) {
		testlogger := ctx.Value("testlogger").(*log.Logger)
		utility := ctx.Value("utility").(*utility.Instance)
		clientIP := utility.GetClientIP(c.Request)
		requestLine := fmt.Sprintf("%s %s %s", c.Request.Method, c.Request.URL.String(), c.Request.Proto)
		testlogger.Info(`%s - %s [%s] "%s" %d %d %d %.3f "%s" "%s" %s "-" "-"`, clientIP, c.Request.Host, time.Now().Format("2/Jan/2006:15:04:05 -0700"), requestLine, c.Request.ContentLength, rw.Status, rw.BytesWritten, elapsed/1e3, c.Request.Header.Get("Referer"), c.Request.Header.Get("User-Agent"), c.Request.RemoteAddr)
	}
	errorLogHandler := func(ctx context.Context, c *routing.Context, err error) error {
		testlogger := ctx.Value("testlogger").(*log.Logger)
		testlogger.Info(`%s|%s|%s|%s|%s|%s`, time.Now().Format("2016-04-20T17:40:12+08:00"), log.LevelError, "", c.Get("c.RequestuestId"), err.Error(), c.Get("errorStack"))
		return nil
	}
	// server
	testlogger := a.Context.Value("testlogger").(*log.Logger)
	a.WithContextValue("Foo", "Bar")
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
	rg := r.AddRouteGroup("").WithCallback(&ServerGroupRequestInitFunc{})
	rg.AddRoute("GET", "/test/<id>", func(ctx context.Context, c *routing.Context) (context.Context, error) {
		c.Response.Write([]byte(c.Param("id")))
		return ctx, nil
	})
	rg.AddRoute("GET", "/Foo", func(ctx context.Context, c *routing.Context) (context.Context, error) {
		c.Response.Write([]byte(ctx.Value("Foo").(string)))
		return ctx, nil
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
