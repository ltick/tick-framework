package ltick

import (
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
		ComponentStates map[string]ComponentState
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
		ComponentStates: make(map[string]ComponentState),
	}
	return r, nil
}
