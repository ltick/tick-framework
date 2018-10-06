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
}

// Make sure that VariableThatShouldStartAtFive is set to five
// before each test
func (suite *TestSuite) SetupTest() {
	var testSystemConfig string = `{
  "temporary_path": "/tmp",
  "server": {
    "port": 8081,
    "max_idle_conns_per_host": 10,
    "request_timeout": "0s",
    "graceful_stop_timeout": "120s"
  },
  "router": {
  	"proxy": {
      "www.example.com": {
        "rule": [
          "(?P<prefix>test)(\/|[?])(?P<path>.*)$"
        ],
        "upstream": "http://127.0.0.1:8088/$prefix$path"
      }
    }
  },
  "component": {
    "Logger": {
      "Targets": {
        "access": {
          "Type": "%ACCESS_LOG_TYPE%",
          "Writer": "%ACCESS_LOG_WRITER%",
          "Filename": "%ACCESS_LOG_FILENAME%",
          "MaxLevel": "%ACCESS_LOG_MAX_LEVEL%",
          "Formatter": "%ACCESS_LOG_FORMATTER%"
        },
        "debug": {
          "Type": "%DEBUG_LOG_TYPE%",
          "Writer": "%DEBUG_LOG_WRITER%",
          "Filename": "%DEBUG_LOG_FILENAME%",
          "MaxLevel": "%DEBUG_LOG_MAX_LEVEL%",
          "Formatter": "%DEBUG_LOG_FORMATTER%"
        },
        "system": {
          "Type": "%SYSTEM_LOG_TYPE%",
          "Writer": "%SYSTEM_LOG_WRITER%",
          "Filename": "%SYSTEM_LOG_FILENAME%",
          "MaxLevel": "%SYSTEM_LOG_MAX_LEVEL%",
          "Formatter": "%SYSTEM_LOG_FORMATTER%"
        }
      }
    },
    "testModule1": {
      "Foo": "Bar"
    }
  }
}`
	var testEnvConfig string = `LTICK_APP_ENV=docker
LTICK_DEBUG=1
LTICK_TMP_PATH=/tmp

LTICK_ZOOKEEPER_HOST=10.30.129.204:2181
LTICK_ZOOKEEPER_USER=
LTICK_ZOOKEEPER_PASSWORD=

LTICK_STORAGE_HBASE_HOST=10
LTICK_STORAGE_HBASE_PORT=10
LTICK_STORAGE_HBASE_POOL_SIZE=10

LTICK_DATABASE_HOST=10.30.129.204
LTICK_DATABASE_PORT=3306
LTICK_DATABASE_USER=root
LTICK_DATABASE_PASSWORD=19850402
LTICK_DATABASE_TIMEOUT=5000ms
LTICK_DATABASE_MAX_OPEN_CONNS=100
LTICK_DATABASE_MAX_IDLE_CONNS=300

LTICK_REDIS_HOST=10.30.129.151
LTICK_REDIS_PORT=6379
LTICK_REDIS_PASSWORD=
LTICK_REDIS_MAX_IDLE=100
LTICK_REDIS_MAX_ACTIVE=300

LTICK_QUEUE_PROVIDER=kafka
LTICK_QUEUE_KAFKA_BROKERS=127.0.0.1
LTICK_QUEUE_KAFKA_EVENT_GROUP=
LTICK_QUEUE_KAFKA_EVENT_TOPIC=test

LTICK_ACCESS_LOG_TYPE=file
LTICK_ACCESS_LOG_FILENAME=/tmp/access.log
LTICK_DEBUG_LOG_TYPE=console
LTICK_DEBUG_LOG_WRITER=stdout
LTICK_SYSTEM_LOG_TYPE=console
LTICK_SYSTEM_LOG_WRITER=stdout
`
	fd, err := os.OpenFile("/tmp/ltick.json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err == nil {
		_, err = fd.Write([]byte(testSystemConfig))
		if err != nil {
			fmt.Println("xxxx")
		}
	}
	fd, err = os.OpenFile("/tmp/.env", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err == nil {
		_, err = fd.Write([]byte(testEnvConfig))
		if err != nil {
			fmt.Println("xxxx")
		}
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

func (suite *TestSuite) TestAppCallback(t *testing.T) {
	var values map[string]interface{} = make(map[string]interface{}, 0)
	var components []*Component = []*Component{}
	var configs map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(os.Args[0]), "/tmp/ltick.json", "LTICK", components, configs).
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

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
