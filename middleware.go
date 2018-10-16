package ltick

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/ltick/tick-routing"
)

var (
	errMiddlewareExists                 = "ltick: middleware '%s' exists"
	errMiddlewareNotExists              = "ltick: middleware '%s' not exists"
	errRegisterMiddleware               = "ltick: register middleware '%s' error"
	errUnRegisterMiddleware             = "ltick: unregister middleware '%s' error"
	errInjectMiddlewareTo               = "ltick: inject middleware '%s' field '%s' error"
)

type MiddlewareInterface interface {
	Initiate(ctx context.Context) (context.Context, error)
	OnRequestStartup(c *routing.Context) error
	OnRequestShutdown(c *routing.Context) error
}

type Middleware struct {
	Name       string
	Middleware MiddlewareInterface
}

/**************** Middleware ****************/
// Register As Middleware
func (r *Registry) RegisterMiddleware(middlewareName string, middleware MiddlewareInterface, ignoreIfExistses ...bool) error {
	var err error
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := r.MiddlewareMap[canonicalMiddlewareName]; ok {
		if !ignoreIfExists {
			return fmt.Errorf(errMiddlewareExists, canonicalMiddlewareName)
		}
		err = r.UnregisterMiddleware(canonicalMiddlewareName)
		if err != nil {
			return fmt.Errorf(errRegisterMiddleware+": %s", canonicalMiddlewareName, err.Error())
		}
	}
	r.Middlewares = append(r.Middlewares, middleware)
	r.MiddlewareMap[canonicalMiddlewareName] = middleware
	r.SortedMiddlewareName = append(r.SortedMiddlewareName, canonicalMiddlewareName)
	return nil
}

// Unregister As Middleware
func (r *Registry) UnregisterMiddleware(middlewareNames ...string) error {
	if len(middlewareNames) > 0 {
		for _, middlewareName := range middlewareNames {
			canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
			_, ok := r.MiddlewareMap[canonicalMiddlewareName]
			if !ok {
				return fmt.Errorf(errMiddlewareNotExists, canonicalMiddlewareName)
			}
			for index, sortedMiddlewareName := range r.SortedMiddlewareName {
				if canonicalMiddlewareName == sortedMiddlewareName {
					r.SortedMiddlewareName = append(r.SortedMiddlewareName[:index], r.SortedMiddlewareName[index+1:]...)
				}
			}
			for index, m := range r.Middlewares {
				if middleware, ok := m.(*Middleware); ok {
					if canonicalMiddlewareName == middleware.Name {
						r.Middlewares = append(r.Middlewares[:index], r.Middlewares[index+1:]...)
					}
				}
			}
			delete(r.MiddlewareMap, canonicalMiddlewareName)
		}
	}
	return nil
}

func (r *Registry) GetMiddleware(middlewareName string) (interface{}, error) {
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	if _, ok := r.MiddlewareMap[canonicalMiddlewareName]; !ok {
		return nil, fmt.Errorf(errMiddlewareNotExists, canonicalMiddlewareName)
	}
	return r.MiddlewareMap[canonicalMiddlewareName], nil
}

func (r *Registry) GetMiddlewareMap() map[string]interface{} {
	return r.MiddlewareMap
}

func (r *Registry) InjectMiddleware() error {
	return r.InjectMiddlewareTo(r.GetSortedMiddlewares())
}

func (r *Registry) InjectMiddlewareByName(componentNames []string) error {
	componentMap := r.GetMiddlewareMap()
	injectTargets := make([]interface{}, 0)
	for _, componentName := range componentNames {
		canonicalMiddlewareName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		if injectTarget, ok := componentMap[canonicalMiddlewareName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return r.InjectMiddlewareTo(injectTargets)
}

func (r *Registry) InjectMiddlewareTo(injectTargets []interface{}) error {
	for _, injectTarget := range injectTargets {
		injectTargetValue := reflect.ValueOf(injectTarget)
		for injectTargetValue.Kind() == reflect.Ptr {
			injectTargetValue = injectTargetValue.Elem()
		}
		if injectTargetValue.Kind() != reflect.Struct {
			continue
		}
		s := structs.New(injectTarget)
		componentType := reflect.TypeOf((*ComponentInterface)(nil)).Elem()
		for _, f := range s.Fields() {
			if f.IsExported() && f.Tag(INJECT_TAG) == "true" {
				if reflect.TypeOf(f.Value()).Implements(componentType) {
					if _, ok := f.Value().(ComponentInterface); ok {
						fieldInjected := false
						if _, ok := r.ComponentMap[f.Name()]; ok {
							err := f.Set(r.ComponentMap[f.Name()])
							if err != nil {
								return fmt.Errorf(errInjectMiddlewareTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
							}
							fieldInjected = true
						}
						if !fieldInjected {
							return fmt.Errorf(errInjectMiddlewareTo+": component or key not exists", injectTargetValue.String(), f.Name())
						}
					}
				}
				if _, ok := r.Values[f.Name()]; ok {
					err := f.Set(r.Values[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectMiddlewareTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
				}
			}
		}
	}
	return nil
}

func (r *Registry) GetSortedMiddlewares(reverses ...bool) []interface{} {
	middlewares := make([]interface{}, len(r.Middlewares))
	if len(r.SortedMiddlewareName) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(r.SortedMiddlewareName) - 1; i >= 0; i-- {
				if middleware, ok := r.MiddlewareMap[r.SortedMiddlewareName[i]]; ok {
					middlewares[index] = middleware
					index++
				}
			}
		} else {
			for i := 0; i < len(r.SortedMiddlewareName); i++ {
				if middleware, ok := r.MiddlewareMap[r.SortedMiddlewareName[i]]; ok {
					middlewares[index] = middleware
					index++
				}
			}
		}
	}
	return middlewares
}

func (r *Registry) GetSortedMiddlewareName() []string {
	return r.SortedMiddlewareName
}
