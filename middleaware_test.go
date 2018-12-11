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

func (f *testMiddleware1) Prepare(ctx context.Context) (newCtx context.Context, err error) {
	return ctx, nil
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

func (f *testMiddleware2) Prepare(ctx context.Context) (newCtx context.Context, err error) {
	return ctx, nil
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
	r, err := NewRegistry()
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
	for key, value := range values {
		err := r.RegisterValue(key, value)
		assert.Nil(suite.T(), err)
	}
	a := New(r,
		EngineLogWriter(ioutil.Discard),
		EngineCallback(&TestCallback{}),
		EngineConfigFile(suite.configFile),
		EngineConfigDotenvFile(suite.dotenvFile),
		EngineConfigEnvPrefix("LTICK"))
	a.SetContextValue("output", "")
	router := a.NewServerRouter(ServerRouterTimeoutHandlers([]routing.Handler{func(c *routing.Context) error {
		a.Context = context.WithValue(a.Context, "output", "Timeout")
		return routing.NewHTTPError(http.StatusRequestTimeout)
	}}), ServerRouterTimeout("3s"), )
	assert.NotNil(suite.T(), router)
	srv := a.NewServer(router, ServerLogWriter(ioutil.Discard), ServerPort(8080), ServerGracefulStopTimeoutDuration(30*time.Second))
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
