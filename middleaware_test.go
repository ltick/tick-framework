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

type testMiddleware1 struct {
	Config *libConfig.Config
	Foo    string
	Foo1   string
}

func (f *testMiddleware1) Initiate(ctx context.Context) (newCtx context.Context, err error) {
	var options map[string]libConfig.Option = map[string]libConfig.Option{}
	newCtx, err = f.Config.SetOptions(ctx, options)
	if err != nil {
		return newCtx, err
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
	Config *libConfig.Config
	Test   *testMiddleware1 `inject:"true"`
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

func (suite *TestSuite) TestMiddleware(t *testing.T) {
	var values map[string]interface{} = map[string]interface{}{}
	var components []*Component = []*Component{}
	var options map[string]libConfig.Option = make(map[string]libConfig.Option, 0)
	a := New(os.Args[0], filepath.Dir(os.Args[0]), "/tmp/ltick.json", "LTICK", components, options).
		WithCallback(&TestCallback{}).
		WithValues(values)
	a.SetSystemLogWriter(ioutil.Discard)
	a.SetContextValue("output", "")
	a.Startup()
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
	a.Shutdown()
	assert.Equal(t, "testComponent1-Register|testComponent1-Startup||testComponent1-Unregister", a.GetContextValue("output"))
}

