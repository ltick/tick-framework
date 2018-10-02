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

	libConfig "github.com/ltick/tick-framework/config"
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
	var modules []*Component = []*Component{}
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

type testComponent1 struct {
	Config *libConfig.Config
	Foo    string
	Foo1   string
}

func (this *testComponent1) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	newCtx, err = this.Config.SetOptions(ctx, options)
	if err != nil {
		return newCtx, err
	}
	return ctx, nil
}
func (f *testComponent1) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testComponent1-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testComponent1) OnShutdown(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "|testComponent1-Shutdown"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testComponent1) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "testComponent1-RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar1")
	}
	return nil
}
func (f *testComponent1) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|testComponent1-RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar2")
	}
	return nil
}

type testComponent2 struct {
	Config *libConfig.Config
	Test   *testComponent1 `inject:"true"`
}

func (this *testComponent2) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	newCtx, err = this.Config.SetOptions(ctx, options)
	if err != nil {
		return newCtx, err
	}
	return ctx, nil
}
func (f *testComponent2) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testComponent2-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	f.Test.Foo = "Bar2"
	return ctx, nil
}
func (f *testComponent2) OnShutdown(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "|testComponent2-Shutdown"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testComponent2) OnRequestStartup(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "testComponent2-RequestStartup|"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	if c.Get("Foo") == nil {
		c.Set("Foo", "Bar1")
	}
	return  nil
}
func (f *testComponent2) OnRequestShutdown(c *routing.Context) error {
	if c.Context.Value("output") != nil {
		output := c.Context.Value("output").(string)
		output = output + "|testComponent2-RequestShutdown"
		c.Context = context.WithValue(c.Context, "output", output)
	}
	return nil
}

func TestComponent(t *testing.T) {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*Component = []*Component{
		&Component{Name: "testComponent2", Component: &testComponent2{}},
	}
	var options map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err := a.RegisterUserComponent("testComponent1", &testComponent1{})
	assert.Nil(t, err)
	testComponent, err := a.GetComponent("testComponent1")
	assert.Nil(t, err)
	module, ok := testComponent.(*testComponent1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Foo", module.Foo)
	err = a.LoadComponentFileConfig("testComponent1", "testdata/services.json", nil, "modules.testComponent1")
	assert.Nil(t, err)
	a.Startup()
	assert.Equal(t, "testComponent1-Register|testComponent1-Startup|", a.GetContextValue("output"))
	testComponent, err = a.GetComponent("testComponent1")
	assert.Nil(t, err)
	module, ok = testComponent.(*testComponent1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Bar", module.Foo)
	a.LoadComponentJsonConfig("testComponent1", []byte(`
{
  "modules": {
    "testComponent1": {
        "Foo": "Bar1"
    }
  }
}`), nil, "testComponent1")
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

	err = a.UnregisterUserComponent("testComponent1")
	assert.Nil(t, err)
	a.Shutdown()
	assert.Equal(t, "testComponent1-Register|testComponent1-Startup||testComponent1-Unregister", a.GetContextValue("output"))
}

func TestInjectComponent(t *testing.T) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*Component = []*Component{}
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err := a.RegisterUserComponent("Test", &testComponent1{})
	assert.Nil(t, err)
	testComponent, err := a.GetComponent("Test")
	assert.Nil(t, err)
	module, ok := testComponent.(*testComponent1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Foo", module.Foo)
	err = a.RegisterUserComponent("testComponent2", &testComponent2{})
	assert.Nil(t, err)
	err = a.RegisterValue("Foo", "Bar2")
	assert.Nil(t, err)
	err = a.InjectComponentByName("testComponent2")
	assert.Nil(t, err)
	a.Startup()
	testComponent, err = a.GetComponent("testComponent2")
	assert.Nil(t, err)
	_module, ok := testComponent.(*testComponent2)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Bar2", _module.Test.Foo)
	assert.Equal(t, "Bar2", module.Foo)
	assert.Equal(t, "testComponent1-Register|testComponent2-Register|testComponent1-Startup|testComponent2-Startup|", a.GetContextValue("output"))
	a.Shutdown()
	assert.Equal(t, "testComponent1-Register|testComponent2-Register|testComponent1-Startup|testComponent2-Startup||testComponent2-Shutdown|testComponent1-Shutdown", a.GetContextValue("output"))
}

func TestBuildinComponent(t *testing.T) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var modules []*Component = []*Component{}
	a := New(os.Args[0], filepath.Dir(filepath.Dir(os.Args[0])), "ltick.json", "LTICK", modules, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.UseComponent("Database", "Cache", "queue")
	assert.Nil(t, err)
	logger, err := a.GetBuiltinComponent("logger")
	assert.Nil(t, err)
	assert.NotNil(t, logger)
	config, err := a.GetBuiltinComponent("Config")
	assert.Nil(t, err)
	assert.NotNil(t, config)
	utility, err := a.GetBuiltinComponent("utility")
	assert.Nil(t, err)
	assert.NotNil(t, utility)
	database, err := a.GetComponent("database")
	assert.Nil(t, err)
	assert.NotNil(t, database)
	cache, err := a.GetComponent("cache")
	assert.Nil(t, err)
	assert.NotNil(t, cache)
	queue, err := a.GetComponent("queue")
	assert.Nil(t, err)
	assert.NotNil(t, queue)
}
