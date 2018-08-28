package module

import (
	"context"

	"fmt"
	"github.com/ltick/tick-framework/module/cache"
	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/database"
	"github.com/ltick/tick-framework/module/logger"
	"github.com/ltick/tick-framework/module/queue"
	"github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-routing"
	"strings"
)

var (
	errNewInstance                  = "ltick: new instance "
	errModuleExists                 = "ltick: module '%s' exists"
	errModuleNotExists              = "ltick: module '%s' not exists"
	errUserModuleNotExists          = "ltick: user module '%s' not exists"
	errBuiltinModuleNotExists       = "ltick: builtin module '%s' not exists"
	errModuleInvaildType            = "ltick: module '%s' invalid type"
	errModuleLoadConfig             = "ltick: module '%s' load config error"
	errModuleRegisterConfigProvider = "ltick: module '%s' register config provider error"
	errModuleConfigure              = "ltick: module '%s' configure error"
	errRegisterModule               = "ltick: register module '%s' error"
	errUnregisterModule             = "ltick: unregister module '%s' error"
	errRegisterUserModule           = "ltick: register user module '%s' error"
	errUnregisterUserModule         = "ltick: unregister user module '%s' error"
	errRegisterBuiltinModule        = "ltick: register builtin module '%s' error"
	errUnregisterBuiltinModule      = "ltick: unregister builtin module '%s' error"
	errInjectModule                 = "ltick: inject builtin module '%s' field '%s' error"
	errInjectModuleTo               = "ltick: inject module '%s' field '%s' error"
	errUseModule                    = "ltick: use module error"
	errValueExists                  = "ltick: value '%s' exists"
	errValueNotExists               = "ltick: value '%s' not exists"
)

const INJECT_TAG = "inject"

var BuiltinModules = []*Module{
	&Module{Name: "Utility", Module: &utility.Instance{}},
	&Module{Name: "Logger", Module: &logger.Instance{}},
	&Module{Name: "Config", Module: &config.Instance{}},
}

type Module struct {
	Name   string
	Module ModuleInterface
}

var Modules = []*Module{
	&Module{Name: "Database", Module: &database.Instance{}},
	&Module{Name: "Cache", Module: &cache.Instance{}},
	&Module{Name: "Queue", Module: &queue.Instance{}},
}

type ModuleInterface interface {
	Initiate(ctx context.Context) (context.Context, error)
	OnStartup(ctx context.Context) (context.Context, error)
	OnShutdown(ctx context.Context) (context.Context, error)
	OnRequestStartup(c *routing.Context) error
	OnRequestShutdown(c *routing.Context) error
}

type Instance struct {
	BuiltinModules       map[string]interface{}
	SortedBuiltinModules []string
	UserModules          map[string]interface{}
	SortedUserModules    []string
	Modules              map[string]interface{}
	SortedModules        []string
	Values               map[string]interface{}
}

func NewInstance(ctx context.Context) (context.Context, *Instance, error) {
	instance := &Instance{
		BuiltinModules:       make(map[string]interface{}),
		SortedBuiltinModules: make([]string, 0),
		UserModules:          make(map[string]interface{}),
		SortedUserModules:    make([]string, 0),
		Modules:              make(map[string]interface{}),
		SortedModules:        make([]string, 0),
		Values:               make(map[string]interface{}),
	}
	// 内置模块注册
	for _, builtinModule := range BuiltinModules {
		canonicalBuiltinModuleName := strings.ToUpper(builtinModule.Name[0:1]) + builtinModule.Name[1:]
		ctx, err := instance.registerBuiltinModule(ctx, canonicalBuiltinModuleName, builtinModule.Module, true)
		if err != nil {
			return ctx, nil, fmt.Errorf(errNewInstance+": %s", err.Error())
		}
	}
	return ctx, instance, nil
}
