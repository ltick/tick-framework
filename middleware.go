package ltick

import (
	"context"
	"fmt"

	"github.com/juju/errors"
	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-framework/middleware"
)

var (
	errGetMiddlewareByName  = "ltick: get middleware by name error"
	errMiddlewareExists     = "ltick: middleware '%s' exists"
	errMiddlewareNotExists  = "ltick: middleware '%s' not exists"
	errRegisterMiddleware   = "ltick: register middleware '%s' error"
	errUnRegisterMiddleware = "ltick: unregister middleware '%s' error"
	errInjectMiddlewareTo   = "ltick: inject middleware '%s' field '%s' error"
	errUseMiddleware   = "ltick: use middleware '%s' error"
)

type MiddlewareInterface interface {
	Prepare(ctx context.Context) (context.Context, error)
	Initiate(ctx context.Context) (context.Context, error)
	OnRequestStartup(c *routing.Context) error
	OnRequestShutdown(c *routing.Context) error
}

type Middlewares []*Middleware

func (cs Middlewares) Get(name string) *Middleware {
	for _, c := range cs {
		canonicalName := canonicalName(name)
		if canonicalName == c.Name {
			return c
		}
	}
	return nil
}

type Middleware struct {
	Name       string
	Middleware MiddlewareInterface
}

var (
	OptionalMiddlewares = Middlewares{
		&Middleware{Name: "IPFilter", Middleware: &middleware.IPFilter{}},
		&Middleware{Name: "Prometheus", Middleware: &middleware.Prometheus{}},
	}
)

/**************** Middleware ****************/
func (r *Registry) UseMiddleware(middlewareNames ...string) error {
	var err error
	middlewares := make([]*Middleware, 0)
	for _, middlewareName := range middlewareNames {
		canonicalMiddlewareName := canonicalName(middlewareName)
		middleware := OptionalMiddlewares.Get(canonicalMiddlewareName)
		if middleware != nil {
			middlewares = append(middlewares, middleware)
			err = r.RegisterMiddleware(middleware, true)
			if err != nil {
				return errors.Annotatef(err, errUseMiddleware, middleware.Name)
			}
		} else {
			return errors.Annotatef(err, errMiddlewareNotExists, canonicalMiddlewareName)
		}
	}
	for _, middleware := range middlewares {
		err = r.InjectComponentTo([]interface{}{middleware})
		if err != nil {
			return errors.Annotatef(err, errUseMiddleware, middleware.Name)
		}
	}
	return nil
}

/**************** Middleware ****************/
// Register As Middleware
func (r *Registry) RegisterMiddleware(middleware *Middleware, ignoreIfExistses ...bool) error {
	var err error
	canonicalMiddlewareName := canonicalName(middleware.Name)
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
			canonicalMiddlewareName := canonicalName(middlewareName)
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
				if canonicalMiddlewareName == m.Name {
					r.Middlewares = append(r.Middlewares[:index], r.Middlewares[index+1:]...)
				}
			}
			delete(r.MiddlewareMap, canonicalMiddlewareName)
		}
	}
	return nil
}

func (r *Registry) GetMiddlewareByName(middlewareName string) (*Middleware, error) {
	canonicalMiddlewareName := canonicalName(middlewareName)
	if _, ok := r.MiddlewareMap[canonicalMiddlewareName]; !ok {
		return nil, errors.Annotate(errors.Errorf(errMiddlewareNotExists, canonicalMiddlewareName), errGetMiddlewareByName)
	}
	return r.MiddlewareMap[canonicalMiddlewareName], nil
}

func (r *Registry) GetMiddlewareMap() map[string]*Middleware {
	return r.MiddlewareMap
}

func (r *Registry) InjectMiddleware() error {
	middlewares := r.GetSortedMiddlewares()
	injectTargets := make([]interface{}, len(middlewares))
	for i, c := range middlewares {
		injectTargets[i] = c.Middleware
	}
	return r.InjectComponentTo(injectTargets)
}

func (r *Registry) InjectMiddlewareByName(middlewareNames []string) error {
	middlewareMap := r.GetMiddlewareMap()
	injectTargets := make([]interface{}, 0)
	for _, middlewareName := range middlewareNames {
		canonicalMiddlewareName := canonicalName(middlewareName)
		if injectTarget, ok := middlewareMap[canonicalMiddlewareName]; ok {
			injectTargets = append(injectTargets, injectTarget.Middleware)
		}
	}
	return r.InjectComponentTo(injectTargets)
}

func (r *Registry) GetSortedMiddlewares(reverses ...bool) []*Middleware {
	middlewares := make([]*Middleware, len(r.Middlewares))
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
