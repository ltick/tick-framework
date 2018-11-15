package ltick

import (
	"context"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
)

const INJECT_TAG = "inject"

type (
	Registry struct {
		Components           []*Component
		ComponentMap         map[string]*Component
		SortedComponentName  []string
		Middlewares          []*Middleware
		MiddlewareMap        map[string]*Middleware
		SortedMiddlewareName []string
		Values               map[string]interface{}
	}
)

func NewRegistry() (r *Registry, err error) {
	r = &Registry{
		Components:           make([]*Component, 0),
		ComponentMap:         make(map[string]*Component),
		SortedComponentName:  make([]string, 0),
		Middlewares:          make([]*Middleware, 0),
		MiddlewareMap:        make(map[string]*Middleware),
		SortedMiddlewareName: make([]string, 0),
		Values:               make(map[string]interface{}),
	}
	// 注册内置模块
	configer := &config.Config{}
	err = r.RegisterComponent(&Component{
		Name:      "Config",
		Component: configer,
	}, true)
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
