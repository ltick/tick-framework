package ltick

import (
	"context"

	"github.com/ltick/tick-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/juju/errors"
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
	r, err := NewRegistry([]*Component{})
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	err = r.RegisterComponent("TestComponent1", &testComponent1{})
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	testComponent, err := r.GetComponentByName("TestComponent1")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	err = r.RegisterValue("Foo", "Bar")
	err = r.RegisterValue("Foo1", "Bar1")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	if testComponent != nil {
		testComponent1, ok := testComponent.(*testComponent1)
		assert.Equal(suite.T(), true, ok)
		err = r.RegisterComponent("TestComponent2", &testComponent2{})
		assert.Nil(suite.T(), err, errors.ErrorStack(err))
		testComponent, err = r.GetComponentByName("TestComponent2")
		assert.Nil(suite.T(), err, errors.ErrorStack(err))
		if testComponent != nil {
			testComponent2, ok := testComponent.(*testComponent2)
			assert.Equal(suite.T(), true, ok)
			err = r.RegisterComponent("TestComponent3", &testComponent3{})
			assert.Nil(suite.T(), err, errors.ErrorStack(err))
			testComponent, err = r.GetComponentByName("TestComponent3")
			assert.Nil(suite.T(), err, errors.ErrorStack(err))
			if testComponent != nil {
				testComponent3, ok := testComponent.(*testComponent3)
				assert.Equal(suite.T(), true, ok)

				err = r.InjectComponentByName([]string{"TestComponent1", "TestComponent2", "TestComponent3"})
				assert.Nil(suite.T(), err, errors.ErrorStack(err))
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
	var components []*Component = []*Component{
		&Component{Name: "testComponent2", Component: &testComponent2{}},
	}
	r, err := NewRegistry(components)
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	logComponent, err := r.GetComponentByName("log")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.NotNil(suite.T(), logComponent)
	configComponent, err := r.GetComponentByName("Config")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.NotNil(suite.T(), configComponent)
	err = r.UseComponent("Database", "Cache", "queue")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	databaseComponent, err := r.GetComponentByName("database")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.NotNil(suite.T(), databaseComponent)
	cacheComponent, err := r.GetComponentByName("cache")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.NotNil(suite.T(), cacheComponent)
	queueComponent, err := r.GetComponentByName("queue")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.NotNil(suite.T(), queueComponent)
}

func (suite *TestSuite) TestComponentConfig() {
	var components []*Component = []*Component{
		&Component{Name: "TestComponent1", Component: &testComponent1{}},
		&Component{Name: "TestComponent2", Component: &testComponent2{}},
		&Component{Name: "TestComponent3", Component: &testComponent3{}},
	}
	var options map[string]config.Option = make(map[string]config.Option, 0)
	r, err := NewRegistry(components)
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	err = r.RegisterValue("Foo", "Bar")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	err = r.RegisterValue("Foo1", "Bar1")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	configComponent, err := r.GetComponentByName("Config")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.NotNil(suite.T(), configComponent)
	configer, ok := configComponent.(*config.Config)
	assert.True(suite.T(), ok)
	err = configer.SetOptions(options)
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	err = r.RegisterComponent("TestComponent1", &testComponent1{})
	assert.NotNil(suite.T(), err, errors.ErrorStack(err))
	assert.Equal(suite.T(), "ltick: component 'TestComponent1' exists", err.Error())
	err = r.InjectComponent()
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	testComponent, err := r.GetComponentByName("testComponent1")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	component, ok := testComponent.(*testComponent1)
	assert.Equal(suite.T(), true, ok)
	assert.Equal(suite.T(), "Bar", component.Foo)
	err = r.LoadComponentFileConfig("testComponent1", suite.configFile, nil, "component.TestComponent1")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	err = r.UnregisterComponent("TestComponent2")
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
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
