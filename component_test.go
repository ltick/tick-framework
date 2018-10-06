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

type testComponent1 struct {
	Foo  string `inject:"true"`
	Foo1 string `inject:"true"`
}

func (f *testComponent1) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	return ctx, nil
}
func (f *testComponent1) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testComponent1-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	f.Foo = "Foo"
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

type testComponent2 struct {
	TestComponent1 *testComponent1 `inject:"true"`
	TestComponent3 *testComponent3 `inject:"true"`
}

func (this *testComponent2) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	return ctx, nil
}
func (f *testComponent2) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testComponent2-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
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

type testComponent3 struct {
	TestComponent1 *testComponent1 `inject:"true"`
}

func (this *testComponent3) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	return ctx, nil
}
func (f *testComponent3) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testComponent3-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testComponent3) OnShutdown(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "|testComponent3-Shutdown"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}

type testComponent4 struct {
	TestComponent2 *testComponent2 `inject:"true"`
}

func (this *testComponent4) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	return ctx, nil
}
func (f *testComponent4) OnStartup(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "testComponent4-Startup|"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (f *testComponent4) OnShutdown(ctx context.Context) (context.Context, error) {
	if ctx.Value("output") != nil {
		output := ctx.Value("output").(string)
		output = output + "|testComponent4-Shutdown"
		ctx = context.WithValue(ctx, "output", output)
	}
	return ctx, nil
}
func (suite *TestSuite) TestComponentInjection(t *testing.T) {
	e := &Engine{
		Context:          context.Background(),
		Components:       make([]interface{}, 0),
		ComponentMap:     make(map[string]interface{}),
		SortedComponents: make([]string, 0),
		Values:           make(map[string]interface{}),
	}
	err := e.RegisterComponent("TestComponent1", &testComponent1{})
	assert.Nil(t, err)
	testComponent, err := e.GetComponentByName("TestComponent1")
	assert.Nil(t, err)
	err = e.RegisterValue("Foo", "Bar")
	err = e.RegisterValue("Foo1", "Bar1")
	assert.Nil(t, err)
	if testComponent != nil {
		testComponent1, ok := testComponent.(*testComponent1)
		assert.Equal(t, true, ok)
		err = e.RegisterComponent("TestComponent2", &testComponent2{})
		assert.Nil(t, err)
		testComponent, err = e.GetComponentByName("TestComponent2")
		assert.Nil(t, err)
		if testComponent != nil {
			testComponent2, ok := testComponent.(*testComponent2)
			assert.Equal(t, true, ok)
			err = e.RegisterComponent("TestComponent3", &testComponent3{})
			assert.Nil(t, err)
			testComponent, err = e.GetComponentByName("TestComponent3")
			assert.Nil(t, err)
			if testComponent != nil {
				testComponent3, ok := testComponent.(*testComponent3)
				assert.Equal(t, true, ok)

				err = e.InjectComponentByName([]string{"TestComponent1", "TestComponent2", "TestComponent3"})
				assert.Nil(t, err)
				assert.NotNil(t, testComponent1, "testComponent1 is nil")
				assert.NotNil(t, testComponent2, "testComponent2 is nil")
				assert.NotNil(t, testComponent3, "testComponent3 is nil")
				assert.NotNil(t, testComponent2.TestComponent1, "testComponent2.TestComponent1 is nil")
				assert.NotNil(t, testComponent3.TestComponent1, "testComponent3.TestComponent1 is nil")
				assert.NotNil(t, testComponent2.TestComponent3, "testComponent2.TestComponent3 is nil")
				assert.NotNil(t, testComponent2.TestComponent3.TestComponent1, "testComponent2.TestComponent3.TestComponent1 is nil")
				assert.Equal(t, "Bar", testComponent1.Foo)
				assert.Equal(t, "Bar", testComponent2.TestComponent1.Foo)
				assert.Equal(t, "Bar", testComponent2.TestComponent3.TestComponent1.Foo)
				assert.Equal(t, "Bar1", testComponent1.Foo1)
				assert.Equal(t, "Bar1", testComponent2.TestComponent1.Foo1)
				assert.Equal(t, "Bar1", testComponent2.TestComponent3.TestComponent1.Foo1)
			}
		}
	}
}

func (suite *TestSuite) TestUseComponent(t *testing.T) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var components []*Component = []*Component{
		&Component{Name: "testComponent2", Component: &testComponent2{}},
	}
	a := New(os.Args[0], filepath.Dir(os.Args[0]), "/tmp/ltick.json", "LTICK", components, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.UseComponent("Database", "Cache", "queue")
	assert.Nil(t, err)
	logger, err := a.GetComponentByName("logger")
	assert.Nil(t, err)
	assert.NotNil(t, logger)
	config, err := a.GetComponentByName("Config")
	assert.Nil(t, err)
	assert.NotNil(t, config)
	utility, err := a.GetComponentByName("utility")
	assert.Nil(t, err)
	assert.NotNil(t, utility)
	database, err := a.GetComponentByName("database")
	assert.Nil(t, err)
	assert.NotNil(t, database)
	cache, err := a.GetComponentByName("cache")
	assert.Nil(t, err)
	assert.NotNil(t, cache)
	queue, err := a.GetComponentByName("queue")
	assert.Nil(t, err)
	assert.NotNil(t, queue)
}

func (suite *TestSuite) TestComponentConfig(t *testing.T) {
	var values map[string]interface{} = map[string]interface{}{}
	var components []*Component = []*Component{}
	var options map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(os.Args[0]), "/tmp/ltick.json", "LTICK", components, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err := a.RegisterComponent("testComponent1", &testComponent1{})
	assert.Nil(t, err)
	testComponent, err := a.GetComponentByName("testComponent1")
	assert.Nil(t, err)
	module, ok := testComponent.(*testComponent1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Foo", module.Foo)
	err = a.LoadComponentFileConfig("testComponent1", "testdata/services.json", nil, "components.testComponent1")
	assert.Nil(t, err)
	a.Startup()
	assert.Equal(t, "testComponent1-Register|testComponent1-Startup|", a.GetContextValue("output"))
	testComponent, err = a.GetComponentByName("testComponent1")
	assert.Nil(t, err)
	module, ok = testComponent.(*testComponent1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "Bar", module.Foo)
	a.LoadComponentJsonConfig("testComponent1", []byte(`
{
  "components": {
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
		return nil
	})
	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	a.ServeHTTP(res, req)
	assert.Equal(t, "Bar1", res.Body.String())

	err = a.UnregisterComponent("testComponent1")
	assert.Nil(t, err)
	a.Shutdown()
	assert.Equal(t, "testComponent1-Register|testComponent1-Startup||testComponent1-Unregister", a.GetContextValue("output"))
}

func (suite *TestSuite) TestSortComponent(t *testing.T) {
	components := []*Component{
		&Component{Name: "TestComponent2", Component: &testComponent2{}},
		&Component{Name: "TestComponent4", Component: &testComponent4{}},
		&Component{Name: "TestComponent1", Component: &testComponent1{}},
		&Component{Name: "TestComponent3", Component: &testComponent3{}},
	}
	sortComponent := SortComponent(components)
	assert.Equal(t, sortComponent, []string{"TestComponent1", "TestComponent3", "TestComponent2", "TestComponent4"})
}
