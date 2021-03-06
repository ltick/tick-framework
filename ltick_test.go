package ltick

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/utility"
	"github.com/stretchr/testify/assert"
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
	_, err = utility.CopyFile("testdata/ltick.json", "etc/ltick.json")
	assert.Nil(suite.T(), err)
	_, err = utility.CopyFile("testdata/.env", ".env")
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
	r, err := NewRegistry()
	assert.Nil(suite.T(), err)
	a := New(r,
		EngineLogWriter(ioutil.Discard),
		EngineCallback(&TestCallback{}),
		EngineConfigFile(suite.configFile),
		EngineConfigDotenvFile(suite.dotenvFile),
		EngineConfigEnvPrefix("LTICK"))
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
		err = r.RegisterComponent(c, true)
		assert.Nil(suite.T(), err)
	}
	a := New(r,
		EngineLogWriter(ioutil.Discard),
		EngineCallback(&TestCallback{}),
		EngineConfigFile(suite.configFile),
		EngineConfigDotenvFile(suite.dotenvFile),
		EngineConfigEnvPrefix("LTICK"))
	configComponent, err := r.GetComponentByName("Config")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), configComponent)
	configer, ok := configComponent.Component.(*config.Config)
	assert.True(suite.T(), ok)
	err = configer.SetOptions(options)
	assert.Nil(suite.T(), err)
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
