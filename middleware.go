package ltick

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	libConfig "github.com/go-ozzo/ozzo-config"
	"github.com/ltick/tick-routing"
)

var (
	errMiddlewareExists                 = "ltick: middleware '%s' exists"
	errMiddlewareNotExists              = "ltick: middleware '%s' not exists"
	errMiddlewareInvaildType            = "ltick: middleware '%s' invalid type"
	errMiddlewareLoadConfig             = "ltick: middleware '%s' load config error"
	errMiddlewareRegisterConfigProvider = "ltick: middleware '%s' register config provider error"
	errMiddlewareConfigure              = "ltick: middleware '%s' configure error"
	errRegisterMiddleware               = "ltick: register middleware '%s' error"
	errUnregisterMiddleware             = "ltick: unregister middleware '%s' error"
	errInjectMiddleware                 = "ltick: inject builtin middleware '%s' field '%s' error"
	errInjectMiddlewareTo               = "ltick: inject middleware '%s' field '%s' error"
	errUseMiddleware                    = "ltick: use middleware error"
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

var (
	Middlewares                                = []*Middleware{}
	UserMiddlewareMap   map[string]interface{} = make(map[string]interface{})
	UserMiddlewareOrder []string               = make([]string, 0)
	MiddlewareMap       map[string]interface{} = make(map[string]interface{})
	MiddlewareOrder     []string               = make([]string, 0)
)

/**************** Middleware ****************/
// Register As Middleware
func (e *Engine) registerMiddleware(ctx context.Context, middlewareName string, middleware MiddlewareInterface, ignoreIfExistses ...bool) (context.Context, error) {
	var err error
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := MiddlewareMap[canonicalMiddlewareName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errMiddlewareExists, canonicalMiddlewareName)
		}
		ctx, err = e.unregisterMiddleware(ctx, canonicalMiddlewareName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterMiddleware+": %s", canonicalMiddlewareName, err.Error())
		}
	}
	err = e.InjectMiddlewareTo([]interface{}{middleware})
	if err != nil {
		return ctx, fmt.Errorf(errRegisterMiddleware+": %s", canonicalMiddlewareName, err.Error())
	}
	newCtx, err := middleware.Initiate(ctx)
	if err != nil {
		return ctx, fmt.Errorf(errRegisterMiddleware+": %s", canonicalMiddlewareName, err.Error())
	}
	MiddlewareMap[canonicalMiddlewareName] = middleware
	MiddlewareOrder = append(MiddlewareOrder, canonicalMiddlewareName)
	return newCtx, nil
}

// Unregister As Middleware
func (e *Engine) unregisterMiddleware(ctx context.Context, middlewareNames ...string) (context.Context, error) {
	if len(middlewareNames) > 0 {
		for _, middlewareName := range middlewareNames {
			canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
			_, ok := MiddlewareMap[canonicalMiddlewareName]
			if !ok {
				return ctx, fmt.Errorf(errMiddlewareNotExists, canonicalMiddlewareName)
			}
			for index, sortedMiddlewareName := range MiddlewareOrder {
				if canonicalMiddlewareName == sortedMiddlewareName {
					MiddlewareOrder = append(MiddlewareOrder[:index], MiddlewareOrder[index+1:]...)
				}
			}
			delete(MiddlewareMap, canonicalMiddlewareName)
		}
	}
	return ctx, nil
}

func (e *Engine) GetMiddleware(middlewareName string) (interface{}, error) {
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	if _, ok := MiddlewareMap[canonicalMiddlewareName]; !ok {
		return nil, fmt.Errorf(errMiddlewareNotExists, canonicalMiddlewareName)
	}
	return MiddlewareMap[canonicalMiddlewareName], nil
}

func (e *Engine) GetMiddlewares() map[string]interface{} {
	middlewares := make(map[string]interface{}, len(Middlewares))
	for middlewareName, middleware := range MiddlewareMap {
		middlewares[middlewareName] = middleware
	}
	return middlewares
}

func (e *Engine) GetSortedMiddlewares(reverses ...bool) []interface{} {
	middlewares := make([]interface{}, len(Middlewares))
	if len(MiddlewareOrder) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(MiddlewareOrder) - 1; i >= 0; i-- {
				if middleware, ok := MiddlewareMap[MiddlewareOrder[i]]; ok {
					middlewares[index] = middleware
					index++
				}
			}
		} else {
			for i := 0; i < len(MiddlewareOrder); i++ {
				if middleware, ok := MiddlewareMap[MiddlewareOrder[i]]; ok {
					middlewares[index] = middleware
					index++
				}
			}
		}
	}
	return middlewares
}

func (e *Engine) UseMiddleware(ctx context.Context, middlewareNames ...string) (context.Context, error) {
	var err error
	// 内置模块注册
	for _, middlewareName := range middlewareNames {
		canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
		middlewareTargets := make([]interface{}, 0)
		middlewareInterfaces := make([]MiddlewareInterface, 0)
		middlewareExists := false
		for _, middleware := range Middlewares {
			canonicalExistsMiddlewareName := strings.ToUpper(middleware.Name[0:1]) + middleware.Name[1:]
			if canonicalMiddlewareName == canonicalExistsMiddlewareName {
				middlewareExists = true
				middlewareTargets = append(middlewareTargets, middleware.Middleware)
				middlewareInterfaces = append(middlewareInterfaces, middleware.Middleware)
			}
		}
		if !middlewareExists {
			return ctx, fmt.Errorf(errMiddlewareNotExists, canonicalMiddlewareName)
		}
		err = e.InjectMiddlewareTo(middlewareTargets)
		if err != nil {
			return ctx, fmt.Errorf(errUseMiddleware+": %s", err.Error())
		}
		for _, middlewareInterface := range middlewareInterfaces {
			ctx, err = e.registerMiddleware(ctx, canonicalMiddlewareName, middlewareInterface, true)
			if err != nil {
				return ctx, fmt.Errorf(errUseMiddleware+": %s", err.Error())
			}
		}
	}
	return ctx, nil
}

func (e *Engine) LoadMiddlewareFileConfig(middlewareName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	// create a Config object
	c := libConfig.New()
	err = c.Load(configFile)
	if err != nil {
		return fmt.Errorf(errMiddlewareLoadConfig+": %s", canonicalMiddlewareName, err.Error())
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return fmt.Errorf(errMiddlewareRegisterConfigProvider+": %s", configProviderName, err.Error())
			}
		}
	}
	registeredMiddleware, err := e.GetMiddleware(canonicalMiddlewareName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
	}
	err = c.Configure(registeredMiddleware, configTag...)
	if err != nil {
		return fmt.Errorf(errMiddlewareConfigure+": %s", canonicalMiddlewareName, err.Error())
	}
	MiddlewareMap[canonicalMiddlewareName] = registeredMiddleware
	return nil
}

// Register As Middleware
func (e *Engine) LoadMiddlewareJsonConfig(middlewareName string, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	// create a Config object
	c := libConfig.New()
	err = c.LoadJSON(configData)
	if err != nil {
		return fmt.Errorf(errMiddlewareLoadConfig+": %s", canonicalMiddlewareName, err.Error())
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return fmt.Errorf(errMiddlewareRegisterConfigProvider+": %s", configProviderName, err.Error())
			}
		}
	}
	registeredMiddleware, err := e.GetMiddleware(canonicalMiddlewareName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
	}
	err = c.Configure(registeredMiddleware, configTag...)
	if err != nil {
		return fmt.Errorf(errMiddlewareConfigure+": %s", canonicalMiddlewareName, err.Error())
	}
	MiddlewareMap[canonicalMiddlewareName] = registeredMiddleware
	return nil
}

func (e *Engine) InjectMiddleware() error {
	// Middlewares
	middlewares := e.GetSortedMiddlewares()
	injectTargets := make([]interface{}, len(middlewares))
	for index, injectTarget := range middlewares {
		injectTargets[index] = injectTarget
	}
	return e.InjectMiddlewareTo(injectTargets)
}

func (e *Engine) InjectMiddlewareByName(middlewareNames []string) error {
	// Middlewares
	middlewares := e.GetMiddlewares()
	injectTargets := make([]interface{}, 0)
	for _, middlewareName := range middlewareNames {
		canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
		if injectTarget, ok := middlewares[canonicalMiddlewareName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return e.InjectMiddlewareTo(injectTargets)
}

func (e *Engine) InjectMiddlewareTo(injectTargets []interface{}) error {
	for _, injectTarget := range injectTargets {
		injectTargetValue := reflect.ValueOf(injectTarget)
		for injectTargetValue.Kind() == reflect.Ptr {
			injectTargetValue = injectTargetValue.Elem()
		}
		if injectTargetValue.Kind() != reflect.Struct {
			continue
		}
		s := structs.New(injectTarget)
		for _, f := range s.Fields() {
			if _, ok := MiddlewareMap[f.Name()]; ok {
				err := f.Set(MiddlewareMap[f.Name()])
				if err != nil {
					return fmt.Errorf(errInjectMiddlewareTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
				}
			} else if f.IsExported() && f.Tag(INJECT_TAG) == "true" {
				fieldInjected := false
				if _, ok := UserMiddlewareMap[f.Name()]; ok {
					err := f.Set(UserMiddlewareMap[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectMiddlewareTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
					fieldInjected = true
				}
				if !fieldInjected {
					return fmt.Errorf(errInjectMiddlewareTo+": middleware or key not exists", injectTargetValue.String(), f.Name())
				}
			}
		}
	}
	return nil
}
