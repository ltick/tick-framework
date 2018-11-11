package ltick

import (
	"context"
	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
)

const INJECT_TAG = "inject"

type (
	Registry struct {
		Components           []interface{}
		ComponentMap         map[string]interface{}
		SortedComponentName  []string
		Middlewares          []interface{}
		MiddlewareMap        map[string]interface{}
		SortedMiddlewareName []string
		Values               map[string]interface{}
	}
)

func NewRegistry() (r *Registry, err error) {
	r = &Registry{
		Components:           make([]interface{}, 0),
		ComponentMap:         make(map[string]interface{}),
		SortedComponentName:  make([]string, 0),
		Middlewares:          make([]interface{}, 0),
		MiddlewareMap:        make(map[string]interface{}),
		SortedMiddlewareName: make([]string, 0),
		Values:               make(map[string]interface{}),
	}
	// 注册内置模块
	configer := &config.Config{}
	err = r.RegisterComponent("Config", configer, true)
	if err != nil {
		e := errors.Annotate(err, errNew)
		return nil, e
	}
	_, err = configer.Initiate(context.Background())
	if err != nil {
		e := errors.Annotate(err, errNew)
		return nil, e
	}
	return r, nil
}
