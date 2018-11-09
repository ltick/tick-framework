package ltick

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/ltick/tick-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/juju/errors"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	configFile string
	dotenvFile string
	testAppLog string
}

func (suite *TestSuite) SetupTest() {
	var err error
	suite.configFile, err = filepath.Abs("testdata/ltick.json")
	assert.Nil(suite.T(), err)
	suite.dotenvFile, err = filepath.Abs("testdata/.env")
	assert.Nil(suite.T(), err)
	suite.testAppLog, err = filepath.Abs("testdata/app.log")
	assert.Nil(suite.T(), err)
}

type TestCallback struct{}

func (f *TestCallback) OnStartup(e *Engine) error {
	output := e.GetContextValueString("output")
	output = output + "Startup|"
	e.SetContextValue("output", output)
	return nil
}

func (f *TestCallback) OnShutdown(e *Engine) error {
	output := e.GetContextValueString("output")
	output = output + "|Shutdown"
	e.SetContextValue("output", output)
	return nil
}

func (suite *TestSuite) TestAppCallback() {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	r, err := NewRegistry()
	assert.Nil(suite.T(), err)
	a := New(suite.configFile, suite.dotenvFile, "LTICK", r, configs).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err = a.Startup()
	assert.Nil(suite.T(), err)
	output := a.GetContextValueString("output")
	assert.Equal(suite.T(), "Startup|", output)
	err = a.Shutdown()
	assert.Nil(suite.T(), err)
	output = a.GetContextValueString("output")
	assert.Equal(suite.T(), "Startup||Shutdown", output)
}

func (suite *TestSuite) TestComponentCallback() {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var components []*Component = []*Component{
		&Component{Name: "TestComponent1", Component: &testComponent1{}},
	}
	var options map[string]config.Option = make(map[string]config.Option, 0)
	r, err := NewRegistry()
	assert.Nil(suite.T(), err)
	err = r.RegisterValue("Foo", "Bar")
	assert.Nil(suite.T(), err)
	err = r.RegisterValue("Foo1", "Bar1")
	assert.Nil(suite.T(), err)
	for _, c := range components {
		err = r.RegisterComponent(c.Name, c.Component, true)
		assert.Nil(suite.T(), err)
	}
	configComponent, err := r.GetComponentByName("Config")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), configComponent)
	configer, ok := configComponent.(*config.Config)
	assert.True(suite.T(), ok)
	err = configer.SetOptions(options)
	assert.Nil(suite.T(), err)

	a := New(suite.configFile, suite.dotenvFile, "LTICK", r, configs).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	err = a.Startup()
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.Equal(suite.T(), "Startup|testComponent1-Startup|", a.GetContextValue("output"))
	err = a.Shutdown()
	assert.Nil(suite.T(), err, errors.ErrorStack(err))
	assert.Equal(suite.T(), "Startup|testComponent1-Startup||testComponent1-Shutdown|Shutdown", a.GetContextValue("output"))
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
