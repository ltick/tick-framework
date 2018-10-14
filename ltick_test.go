package ltick

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"fmt"
	libConfig "github.com/ltick/tick-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	systemConfigFile string
	envConfigFile    string
}

func (suite *TestSuite) SetupTest() {
	var err error
	suite.systemConfigFile, err = filepath.Abs("testdata/ltick.json")
	if err != nil {
		fmt.Println("xxxx")
	}
	suite.envConfigFile, err = filepath.Abs("testdata/.env")
	if err != nil {
		fmt.Println("xxxx")
	}
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
	output = output + "Shutdown"
	e.SetContextValue("output", output)
	return nil
}

func (suite *TestSuite) TestAppCallback() {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var components []*Component = []*Component{}
	var configs map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(os.Args[0]), suite.systemConfigFile, "LTICK", components, configs).
		WithCallback(&TestCallback{}).WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	err := a.Startup()
	assert.Nil(suite.T(), err)
	output := a.GetContextValueString("output")
	assert.Equal(suite.T(), "Startup|", output)
	err = a.Shutdown()
	assert.Nil(suite.T(), err)
	output = a.GetContextValueString("output")
	assert.Equal(suite.T(), "Startup|Shutdown", output)
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
