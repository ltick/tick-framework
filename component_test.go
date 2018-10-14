package ltick

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	libConfig "github.com/ltick/tick-framework/config"
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
func (suite *TestSuite) TestComponentInjection() {
	e := &Engine{
		Context:          context.Background(),
		Components:       make([]interface{}, 0),
		ComponentMap:     make(map[string]interface{}),
		SortedComponents: make([]string, 0),
		Values:           make(map[string]interface{}),
	}
	err := e.RegisterComponent("TestComponent1", &testComponent1{})
	assert.Nil(suite.T(), err)
	testComponent, err := e.GetComponentByName("TestComponent1")
	assert.Nil(suite.T(), err)
	err = e.RegisterValue("Foo", "Bar")
	err = e.RegisterValue("Foo1", "Bar1")
	assert.Nil(suite.T(), err)
	if testComponent != nil {
		testComponent1, ok := testComponent.(*testComponent1)
		assert.Equal(suite.T(), true, ok)
		err = e.RegisterComponent("TestComponent2", &testComponent2{})
		assert.Nil(suite.T(), err)
		testComponent, err = e.GetComponentByName("TestComponent2")
		assert.Nil(suite.T(), err)
		if testComponent != nil {
			testComponent2, ok := testComponent.(*testComponent2)
			assert.Equal(suite.T(), true, ok)
			err = e.RegisterComponent("TestComponent3", &testComponent3{})
			assert.Nil(suite.T(), err)
			testComponent, err = e.GetComponentByName("TestComponent3")
			assert.Nil(suite.T(), err)
			if testComponent != nil {
				testComponent3, ok := testComponent.(*testComponent3)
				assert.Equal(suite.T(), true, ok)

				err = e.InjectComponentByName([]string{"TestComponent1", "TestComponent2", "TestComponent3"})
				assert.Nil(suite.T(), err)
				assert.NotNil(suite.T(), testComponent1, "testComponent1 is nil")
				assert.NotNil(suite.T(), testComponent2, "testComponent2 is nil")
				assert.NotNil(suite.T(), testComponent3, "testComponent3 is nil")
				assert.NotNil(suite.T(), testComponent2.TestComponent1, "testComponent2.TestComponent1 is nil")
				assert.NotNil(suite.T(), testComponent3.TestComponent1, "testComponent3.TestComponent1 is nil")
				assert.NotNil(suite.T(), testComponent2.TestComponent3, "testComponent2.TestComponent3 is nil")
				assert.NotNil(suite.T(), testComponent2.TestComponent3.TestComponent1, "testComponent2.TestComponent3.TestComponent1 is nil")
				assert.Equal(suite.T(), "Bar", testComponent1.Foo)
				assert.Equal(suite.T(), "Bar", testComponent2.TestComponent1.Foo)
				assert.Equal(suite.T(), "Bar", testComponent2.TestComponent3.TestComponent1.Foo)
				assert.Equal(suite.T(), "Bar1", testComponent1.Foo1)
				assert.Equal(suite.T(), "Bar1", testComponent2.TestComponent1.Foo1)
				assert.Equal(suite.T(), "Bar1", testComponent2.TestComponent3.TestComponent1.Foo1)
			}
		}
	}
}

func (suite *TestSuite) TestUseComponent() {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var components []*Component = []*Component{
		&Component{Name: "testComponent2", Component: &testComponent2{}},
	}
	a := New(os.Args[0], filepath.Dir(os.Args[0]), suite.systemConfigFile, "LTICK", components, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.UseComponent("Database", "Cache", "queue")
	assert.Nil(suite.T(), err)
	logger, err := a.GetComponentByName("logger")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), logger)
	config, err := a.GetComponentByName("Config")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), config)
	database, err := a.GetComponentByName("database")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), database)
	cache, err := a.GetComponentByName("cache")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), cache)
	queue, err := a.GetComponentByName("queue")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), queue)
}

func (suite *TestSuite) TestComponentConfig() {
	var values map[string]interface{} = map[string]interface{}{}
	var components []*Component = []*Component{
		&Component{Name: "TestComponent1", Component: &testComponent1{}},
	}
	var options map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(os.Args[0]), suite.systemConfigFile, "LTICK", components, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err := a.RegisterComponent("TestComponent1", &testComponent1{})
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "ltick: component 'TestComponent1' exists", err.Error())
	testComponent, err := a.GetComponentByName("testComponent1")
	assert.Nil(suite.T(), err)
	component, ok := testComponent.(*testComponent1)
	assert.Equal(suite.T(), true, ok)
	assert.Equal(suite.T(), "Bar", component.Foo)
	err = a.LoadComponentFileConfig("testComponent1", suite.systemConfigFile, nil, "component.TestComponent1")
	assert.Nil(suite.T(), err)
	a.Startup()
	assert.Equal(suite.T(), "Startup|testComponent1-Startup|", a.GetContextValue("output"))

	err = a.UnregisterComponent("TestComponent1")
	assert.Nil(suite.T(), err)
	a.Shutdown()
	assert.Equal(suite.T(), "Startup|testComponent1-Startup|", a.GetContextValue("output"))
}

func (suite *TestSuite) TestSortComponent() {
	components := []*Component{
		&Component{Name: "TestComponent2", Component: &testComponent2{}},
		&Component{Name: "TestComponent4", Component: &testComponent4{}},
		&Component{Name: "TestComponent1", Component: &testComponent1{}},
		&Component{Name: "TestComponent3", Component: &testComponent3{}},
	}
	sortComponent := SortComponent(components)
	assert.Equal(suite.T(), sortComponent, []string{"TestComponent1", "TestComponent3", "TestComponent2", "TestComponent4"})
}
