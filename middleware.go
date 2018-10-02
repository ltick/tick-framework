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
	errUserMiddlewareNotExists          = "ltick: user middleware '%s' not exists"
	errBuiltinMiddlewareNotExists       = "ltick: builtin middleware '%s' not exists"
	errMiddlewareInvaildType            = "ltick: middleware '%s' invalid type"
	errMiddlewareLoadConfig             = "ltick: middleware '%s' load config error"
	errMiddlewareRegisterConfigProvider = "ltick: middleware '%s' register config provider error"
	errMiddlewareConfigure              = "ltick: middleware '%s' configure error"
	errRegisterMiddleware               = "ltick: register middleware '%s' error"
	errUnregisterMiddleware             = "ltick: unregister middleware '%s' error"
	errRegisterUserMiddleware           = "ltick: register user middleware '%s' error"
	errUnregisterUserMiddleware         = "ltick: unregister user middleware '%s' error"
	errRegisterBuiltinMiddleware        = "ltick: register builtin middleware '%s' error"
	errUnregisterBuiltinMiddleware      = "ltick: unregister builtin middleware '%s' error"
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
func registerMiddleware(ctx context.Context, middlewareName string, middleware MiddlewareInterface, ignoreIfExistses ...bool) (context.Context, error) {
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
		ctx, err = unregisterMiddleware(ctx, canonicalMiddlewareName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterMiddleware+": %s", canonicalMiddlewareName, err.Error())
		}
	}
	err = InjectMiddlewareTo([]interface{}{middleware})
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
func unregisterMiddleware(ctx context.Context, middlewareNames ...string) (context.Context, error) {
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

func GetMiddleware(middlewareName string) (interface{}, error) {
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	if _, ok := MiddlewareMap[canonicalMiddlewareName]; !ok {
		return nil, fmt.Errorf(errMiddlewareNotExists, canonicalMiddlewareName)
	}
	return MiddlewareMap[canonicalMiddlewareName], nil
}

func GetMiddlewares() map[string]interface{} {
	middlewares := make(map[string]interface{}, len(Middlewares))
	for middlewareName, middleware := range MiddlewareMap {
		middlewares[middlewareName] = middleware
	}
	return middlewares
}

func GetSortedMiddlewares(reverses ...bool) []interface{} {
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

func UseMiddleware(ctx context.Context, middlewareNames ...string) (context.Context, error) {
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
		err = InjectMiddlewareTo(middlewareTargets)
		if err != nil {
			return ctx, fmt.Errorf(errUseMiddleware+": %s", err.Error())
		}
		for _, middlewareInterface := range middlewareInterfaces {
			ctx, err = registerMiddleware(ctx, canonicalMiddlewareName, middlewareInterface, true)
			if err != nil {
				return ctx, fmt.Errorf(errUseMiddleware+": %s", err.Error())
			}
		}
	}
	return ctx, nil
}

/**************** User Middleware ****************/
// Register As User Middleware
func RegisterUserMiddleware(ctx context.Context, middlewareName string, middleware MiddlewareInterface, ignoreIfExistses ...bool) (context.Context, error) {
	var err error
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := UserMiddlewareMap[canonicalMiddlewareName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errMiddlewareExists, canonicalMiddlewareName)
		}
		ctx, err = unregisterMiddleware(ctx, middlewareName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterUserMiddleware+": %s", canonicalMiddlewareName, err.Error())
		}
	}
	newCtx, err := registerMiddleware(ctx, canonicalMiddlewareName, middleware, true)
	if err != nil {
		return ctx, fmt.Errorf(errRegisterUserMiddleware+": %s", err.Error())
	}
	UserMiddlewareMap[canonicalMiddlewareName] = middleware
	UserMiddlewareOrder = append(UserMiddlewareOrder, canonicalMiddlewareName)
	return newCtx, nil
}

// Unregister As User Middleware
func UnregisterUserMiddleware(ctx context.Context, middlewareNames ...string) (context.Context, error) {
	if len(middlewareNames) > 0 {
		for _, middlewareName := range middlewareNames {
			canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
			_, ok := UserMiddlewareMap[canonicalMiddlewareName]
			if !ok {
				return ctx, fmt.Errorf(errUserMiddlewareNotExists, canonicalMiddlewareName)
			}
			_, ok = UserMiddlewareMap[canonicalMiddlewareName].(MiddlewareInterface)
			if !ok {
				return ctx, fmt.Errorf(errMiddlewareInvaildType, canonicalMiddlewareName)
			}
			for index, sortedMiddlewareName := range UserMiddlewareOrder {
				if canonicalMiddlewareName == sortedMiddlewareName {
					UserMiddlewareOrder = append(UserMiddlewareOrder[:index], UserMiddlewareOrder[index+1:]...)
				}
			}
			delete(UserMiddlewareMap, canonicalMiddlewareName)
			ctx, err := unregisterMiddleware(ctx, canonicalMiddlewareName)
			if err != nil {
				return ctx, fmt.Errorf(errUnregisterUserMiddleware+": %s [middleware:'%s']", err.Error(), canonicalMiddlewareName)
			}
		}
	}
	return ctx, nil
}

func GetUserMiddleware(middlewareName string) (interface{}, error) {
	canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
	if _, ok := UserMiddlewareMap[canonicalMiddlewareName]; !ok {
		return nil, fmt.Errorf(errUserMiddlewareNotExists, canonicalMiddlewareName)
	}
	return UserMiddlewareMap[canonicalMiddlewareName], nil
}

func GetUserMiddlewareMap() map[string]interface{} {
	middlewares := make(map[string]interface{}, len(UserMiddlewareMap))
	for middlewareName, middleware := range UserMiddlewareMap {
		middlewares[middlewareName] = middleware
	}
	return middlewares
}

func GetSortedUseMiddleware(reverses ...bool) []interface{} {
	middlewares := make([]interface{}, 0)
	if len(UserMiddlewareOrder) > 0 {
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(UserMiddlewareOrder) - 1; i >= 0; i-- {
				if middleware, ok := UserMiddlewareMap[UserMiddlewareOrder[i]]; ok {
					middlewares = append(middlewares, middleware)
				}
			}
		} else {
			for i := 0; i < len(UserMiddlewareOrder); i++ {
				if middleware, ok := UserMiddlewareMap[UserMiddlewareOrder[i]]; ok {
					middlewares = append(middlewares, middleware)
				}
			}
		}
	}
	return middlewares
}

func LoadMiddlewareFileConfig(middlewareName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
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
	registeredMiddleware, err := GetMiddleware(canonicalMiddlewareName)
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
func LoadMiddlewareJsonConfig(middlewareName string, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
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
	registeredMiddleware, err := GetMiddleware(canonicalMiddlewareName)
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

func InjectMiddleware() error {
	// Middlewares
	middlewares := GetSortedMiddlewares()
	injectTargets := make([]interface{}, len(middlewares))
	for index, injectTarget := range middlewares {
		injectTargets[index] = injectTarget
	}
	return InjectMiddlewareTo(injectTargets)
}

func InjectMiddlewareByName(middlewareNames []string) error {
	// Middlewares
	middlewares := GetMiddlewares()
	injectTargets := make([]interface{}, 0)
	for _, middlewareName := range middlewareNames {
		canonicalMiddlewareName := strings.ToUpper(middlewareName[0:1]) + middlewareName[1:]
		if injectTarget, ok := middlewares[canonicalMiddlewareName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return InjectMiddlewareTo(injectTargets)
}

func InjectMiddlewareTo(injectTargets []interface{}) error {
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
				if _, ok := Values[f.Name()]; ok {
					err := f.Set(Values[f.Name()])
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
