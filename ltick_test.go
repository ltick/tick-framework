package ltick

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ltick/tick-framework/module"
	libConfig "github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-routing"
	"github.com/stretchr/testify/assert"
)

type TestCallback struct{}

func (f *TestCallback) OnStartup(e *Engine) error {
	output := e.GetContextValueString("output")
	output = output + "Startup|"
	e.SetContextValue("output", output)
	return nil
}

func (f *TestCallback) OnShutdown(e *Engine) error {
	output := e.GetContextValueString("output")
	output = output + "Shutdown"
	e.SetContextValue("output", output)
	return nil
}

func TestAppCallback(t *testing.T) {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*module.Module = []*module.Module{}
	var configs map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, configs).
		WithCallback(&TestCallback{}).WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.Startup()
	assert.Nil(t, err)
	output := a.GetContextValueString("output")
	assert.Equal(t, "Startup|", output)
	err = a.Shutdown()
	assert.Nil(t, err)
	output = a.GetContextValueString("output")
	assert.Equal(t, "Startup|Shutdown", output)
}

type testModule1 struct {
	Config *libConfig.Instance
	Foo    string
	Foo1   string
}

func (this *testModule1) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	newCtx, err = this.Config.SetOptions(ctx, options)
	if err != nil {
		return newCtx, err
	}
	return ctx, nil
}
func (f *testModule1) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testModule1-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testModule1) OnShutdown(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "|testModule1-Shutdown"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testModule1) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "testModule1-RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar1")
	}
	return nil
}
func (f *testModule1) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|testModule1-RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar2")
	}
	return nil
}

type testModule2 struct {
	Config *libConfig.Instance
	Test   *testModule1 `inject:"true"`
}

func (this *testModule2) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	newCtx, err = this.Config.SetOptions(ctx, options)
	if err != nil {
		return newCtx, err
	}
	return ctx, nil
}
func (f *testModule2) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testModule2-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	f.Test.Foo = "Bar2"
	return ctx, nil
}
func (f *testModule2) OnShutdown(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "|testModule2-Shutdown"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testModule2) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "testModule2-RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar1")
	}
	return  nil
}
func (f *testModule2) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|testModule2-RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	return nil
}

func TestModule(t *testing.T) {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*module.Module = []*module.Module{
		&module.Module{Name: "testModule2", Module: &testModule2{}},
	}
	var options map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err := a.RegisterUserModule("testModule1", &testModule1{})
	assert.Nil(t, err)
	testModule, err := a.GetModule("testModule1")
	assert.Nil(t, err)
	module, ok := testModule.(*testModule1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Foo", module.Foo)
	err = a.LoadModuleFileConfig("testModule1", "testdata/services.json", nil, "modules.testModule1")
	assert.Nil(t, err)
	a.Startup()
	assert.Equal(t, "testModule1-Register|testModule1-Startup|", a.GetContextValue("output"))
	testModule, err = a.GetModule("testModule1")
	assert.Nil(t, err)
	module, ok = testModule.(*testModule1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Bar", module.Foo)
	a.LoadModuleJsonConfig("testModule1", []byte(`
{
  "modules": {
    "testModule1": {
        "Foo": "Bar1"
    }
  }
}`), nil, "testModule1")
	assert.Equal(t, "Bar1", module.Foo)
	srv := a.NewServer("test", 8080, 30*time.Second, 3*time.Second)
	rg := srv.GetRouteGroup("/")
	assert.NotNil(t, rg)
	rg.AddRoute("GET", "/test", func(c *routing.Context) error {
		c.ResponseWriter.Write([]byte(c.Get("Foo").(string)))
		return  nil
	})
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	a.ServeHTTP(res, req)
	assert.Equal(t, "Bar1", res.Body.String())

	err = a.UnregisterUserModule("testModule1")
	assert.Nil(t, err)
	a.Shutdown()
	assert.Equal(t, "testModule1-Register|testModule1-Startup||testModule1-Unregister", a.GetContextValue("output"))
}

func TestInjectModule(t *testing.T) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*module.Module = []*module.Module{}
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err := a.RegisterUserModule("Test", &testModule1{})
	assert.Nil(t, err)
	testModule, err := a.GetModule("Test")
	assert.Nil(t, err)
	module, ok := testModule.(*testModule1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Foo", module.Foo)
	err = a.RegisterUserModule("testModule2", &testModule2{})
	assert.Nil(t, err)
	err = a.RegisterValue("Foo", "Bar2")
	assert.Nil(t, err)
	err = a.InjectModuleByName("testModule2")
	assert.Nil(t, err)
	a.Startup()
	testModule, err = a.GetModule("testModule2")
	assert.Nil(t, err)
	_module, ok := testModule.(*testModule2)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Bar2", _module.Test.Foo)
	assert.Equal(t, "Bar2", module.Foo)
	assert.Equal(t, "testModule1-Register|testModule2-Register|testModule1-Startup|testModule2-Startup|", a.GetContextValue("output"))
	a.Shutdown()
	assert.Equal(t, "testModule1-Register|testModule2-Register|testModule1-Startup|testModule2-Startup||testModule2-Shutdown|testModule1-Shutdown", a.GetContextValue("output"))
}

func TestBuildinModule(t *testing.T) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*module.Module = []*module.Module{}
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.UseModule("Database", "Cache", "queue")
	assert.Nil(t, err)
	logger, err := a.GetBuiltinModule("logger")
	assert.Nil(t, err)
	assert.NotNil(t, logger)
	config, err := a.GetBuiltinModule("Config")
	assert.Nil(t, err)
	assert.NotNil(t, config)
	utility, err := a.GetBuiltinModule("utility")
	assert.Nil(t, err)
	assert.NotNil(t, utility)
	database, err := a.GetModule("database")
	assert.Nil(t, err)
	assert.NotNil(t, database)
	cache, err := a.GetModule("cache")
	assert.Nil(t, err)
	assert.NotNil(t, cache)
	queue, err := a.GetModule("queue")
	assert.Nil(t, err)
	assert.NotNil(t, queue)
}
