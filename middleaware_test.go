package ltick

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-routing"
	"github.com/stretchr/testify/assert"
)

type TestRequestCallback struct {
	Config *config.Config `inject:"true"`
}

func (f *TestRequestCallback) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	return nil
}

func (f *TestRequestCallback) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	return nil
}

type testMiddleware1 struct {
	Config *config.Config `inject:"true"`
	Foo    string
	Foo1   string
}

func (f *testMiddleware1) Initiate(ctx context.Context) (context.Context, error) {
	var options map[string]config.Option = map[string]config.Option{}
	err := f.Config.SetOptions(options)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}

func (f *testMiddleware1) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "testMiddleware1-RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar1")
	}
	return nil
}
func (f *testMiddleware1) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|testMiddleware1-RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar2")
	}
	return nil
}

type testMiddleware2 struct {
	Config *config.Config
	Test   *testMiddleware1 `inject:"true"`
}

func (f *testMiddleware2) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var options map[string]config.Option = map[string]config.Option{}
	err = f.Config.SetOptions(options)
	if err != nil {
		return newCtx, err
	}
	return ctx, nil
}

func (f *testMiddleware2) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "testMiddleware2-RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar1")
	}
	return nil
}
func (f *testMiddleware2) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|testMiddleware2-RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	return nil
}

func (suite *TestSuite) TestMiddleware() {
	var values map[string]interface{} = map[string]interface{}{}
	var options map[string]config.Option = make(map[string]config.Option, 0)
	r, err := NewRegistry()
	assert.Nil(suite.T(), err)
	configComponent, err := r.GetComponentByName("Config")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), configComponent)
	configer, ok := configComponent.Component.(*config.Config)
	assert.True(suite.T(), ok)
	err = configer.SetOptions(options)
	assert.Nil(suite.T(), err)
	err = r.RegisterMiddleware(&Middleware{
		Name:       "testMiddleware1",
		Middleware: &testMiddleware1{},
	})
	assert.Nil(suite.T(), err)
	err = r.RegisterMiddleware(&Middleware{
		Name:       "testMiddleware2",
		Middleware: &testMiddleware2{},
	})
	assert.Nil(suite.T(), err)
	a := New(r, EngineLogWriter(ioutil.Discard), EngineCallback(&TestCallback{})).
		LoadConfig(EngineConfigFile(suite.configFile), EngineConfigDotenvFile(suite.dotenvFile), EngineConfigEnvPrefix("LTICK")).
		WithValues(values)
	a.SetContextValue("output", "")

	router := NewServerRouter(a.Context, ServerRouterTimeoutHandler(func(c *routing.Context) error {
		a.Context = context.WithValue(a.Context, "output", "Timeout")
		return routing.NewHTTPError(http.StatusRequestTimeout)
	}), ServerRouterHandlerTimeout(3*time.Second), ServerRouterGracefulStopTimeout(30*time.Second))
	assert.NotNil(suite.T(), router)
	srv := NewServer(router, ServerLogWriter(ioutil.Discard), ServerPort(8080))
	a.SetServer("test", srv)
	rg := srv.AddRouteGroup("/").AddCallback(&TestRequestCallback{})
	assert.NotNil(suite.T(), rg)
	rg.AddRoute("GET", "test", func(c *routing.Context) error {
		c.ResponseWriter.Write([]byte("Bar1"))
		return nil
	})
	a.Startup()
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	srv.ServeHTTP(res, req)
	assert.Equal(suite.T(), "Bar1", res.Body.String())
	a.Shutdown()
	assert.Equal(suite.T(), "Startup||Shutdown", a.GetContextValue("output"))
}
