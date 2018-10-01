package module

import (
	"errors"
	"reflect"
	"strings"
	"context"
	"fmt"

	"github.com/fatih/structs"
	libConfig "github.com/go-ozzo/ozzo-config"
	"github.com/ltick/tick-framework/module/cache"
	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/database"
	"github.com/ltick/tick-framework/module/logger"
	"github.com/ltick/tick-framework/module/queue"
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
	&Module{Name: "Logger", Module: &logger.Logger{}},
	&Module{Name: "Config", Module: &config.Config{}},
}

type Module struct {
	Name   string
	Module ModuleInterface
}

var Modules = []*Module{
	&Module{Name: "Database", Module: &database.Database{}},
	&Module{Name: "Cache", Module: &cache.Cache{}},
	&Module{Name: "Queue", Module: &queue.Queue{}},
}

type ModuleInterface interface {
	Initiate(ctx context.Context) (context.Context, error)
	OnStartup(ctx context.Context) (context.Context, error)
	OnShutdown(ctx context.Context) (context.Context, error)
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

/**************** Builtin Module ****************/
// Register As BuiltinModule
func (this *Instance) registerBuiltinModule(ctx context.Context, moduleName string, module ModuleInterface, ignoreIfExistses ...bool) (newCtx context.Context, err error) {
	newCtx, err = this.registerModule(ctx, moduleName, module, ignoreIfExistses...)
	if err != nil {
		return newCtx, errors.New(errRegisterBuiltinModule + ": " + err.Error())
	}
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	this.BuiltinModules[canonicalModuleName] = module
	this.SortedBuiltinModules = append(this.SortedBuiltinModules, canonicalModuleName)
	return newCtx, nil
}

// Unregister As BuiltinModule
func (this *Instance) unregisterBuiltinModule(ctx context.Context, moduleNames ...string) (newCtx context.Context, err error) {
	newCtx, err = this.unregisterModule(ctx, moduleNames...)
	if err != nil {
		return newCtx, errors.New(errUnregisterBuiltinModule + ": " + err.Error())
	}
	for _, moduleName := range moduleNames {
		canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
		delete(this.BuiltinModules, canonicalModuleName)
	}
	return ctx, nil
}

func (this *Instance) GetBuiltinModule(moduleName string) (interface{}, error) {
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	if _, ok := this.BuiltinModules[canonicalModuleName]; !ok {
		return nil, fmt.Errorf(errBuiltinModuleNotExists, canonicalModuleName)
	}
	return this.BuiltinModules[canonicalModuleName], nil
}

func (this *Instance) GetBuiltinModules() map[string]interface{} {
	modules := make(map[string]interface{}, len(this.BuiltinModules))
	for moduleName, module := range this.BuiltinModules {
		modules[moduleName] = module
	}
	return modules
}

func (this *Instance) GetSortedBuiltinModules(reverses ...bool) []interface{} {
	modules := make([]interface{}, len(this.BuiltinModules))
	if len(this.SortedBuiltinModules) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(this.SortedBuiltinModules) - 1; i >= 0; i-- {
				if module, ok := this.BuiltinModules[this.SortedBuiltinModules[i]]; ok {
					modules[index] = module
					index++
				}
			}
		} else {
			for i := 0; i < len(this.SortedBuiltinModules); i++ {
				if module, ok := this.BuiltinModules[this.SortedBuiltinModules[i]]; ok {
					modules[index] = module
					index++
				}
			}
		}
	}
	return modules
}

/**************** Module ****************/
// Register As Module
func (this *Instance) registerModule(ctx context.Context, moduleName string, module ModuleInterface, ignoreIfExistses ...bool) (context.Context, error) {
	var err error
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := this.Modules[canonicalModuleName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errModuleExists, canonicalModuleName)
		}
		ctx, err = this.unregisterModule(ctx, canonicalModuleName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterModule+": %s", canonicalModuleName, err.Error())
		}
	}
	err = this.InjectModuleTo([]interface{}{module})
	if err != nil {
		return ctx, fmt.Errorf(errRegisterModule+": %s", canonicalModuleName, err.Error())
	}
	newCtx, err := module.Initiate(ctx)
	if err != nil {
		return ctx, fmt.Errorf(errRegisterModule+": %s", canonicalModuleName, err.Error())
	}
	this.Modules[canonicalModuleName] = module
	this.SortedModules = append(this.SortedModules, canonicalModuleName)
	return newCtx, nil
}

// Unregister As Module
func (this *Instance) unregisterModule(ctx context.Context, moduleNames ...string) (context.Context, error) {
	if len(moduleNames) > 0 {
		for _, moduleName := range moduleNames {
			canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
			_, ok := this.Modules[canonicalModuleName]
			if !ok {
				return ctx, fmt.Errorf(errModuleNotExists, canonicalModuleName)
			}
			for index, sortedModuleName := range this.SortedModules {
				if canonicalModuleName == sortedModuleName {
					this.SortedModules = append(this.SortedModules[:index], this.SortedModules[index+1:]...)
				}
			}
			delete(this.Modules, canonicalModuleName)
		}
	}
	return ctx, nil
}

func (this *Instance) GetModule(moduleName string) (interface{}, error) {
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	if _, ok := this.Modules[canonicalModuleName]; !ok {
		return nil, fmt.Errorf(errModuleNotExists, canonicalModuleName)
	}
	return this.Modules[canonicalModuleName], nil
}

func (this *Instance) GetModules() map[string]interface{} {
	modules := make(map[string]interface{}, len(this.Modules))
	for moduleName, module := range this.Modules {
		modules[moduleName] = module
	}
	return modules
}

func (this *Instance) GetSortedModules(reverses ...bool) []interface{} {
	modules := make([]interface{}, len(this.Modules))
	if len(this.SortedModules) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(this.SortedModules) - 1; i >= 0; i-- {
				if module, ok := this.Modules[this.SortedModules[i]]; ok {
					modules[index] = module
					index++
				}
			}
		} else {
			for i := 0; i < len(this.SortedModules); i++ {
				if module, ok := this.Modules[this.SortedModules[i]]; ok {
					modules[index] = module
					index++
				}
			}
		}
	}
	return modules
}

func (this *Instance) UseModule(ctx context.Context, moduleNames ...string) (context.Context, error) {
	var err error
	// 内置模块注册
	for _, moduleName := range moduleNames {
		canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
		moduleTargets := make([]interface{}, 0)
		moduleInterfaces := make([]ModuleInterface, 0)
		moduleExists := false
		for _, module := range Modules {
			canonicalExistsModuleName := strings.ToUpper(module.Name[0:1]) + module.Name[1:]
			if canonicalModuleName == canonicalExistsModuleName {
				moduleExists = true
				moduleTargets = append(moduleTargets, module.Module)
				moduleInterfaces = append(moduleInterfaces, module.Module)
			}
		}
		if !moduleExists {
			return ctx, fmt.Errorf(errModuleNotExists, canonicalModuleName)
		}
		err = this.InjectModuleTo(moduleTargets)
		if err != nil {
			return ctx, fmt.Errorf(errUseModule+": %s", err.Error())
		}
		for _, moduleInterface := range moduleInterfaces {
			ctx, err = this.registerModule(ctx, canonicalModuleName, moduleInterface, true)
			if err != nil {
				return ctx, fmt.Errorf(errUseModule+": %s", err.Error())
			}
		}
	}
	return ctx, nil
}

/**************** User Module ****************/
// Register As User Module
func (this *Instance) RegisterUserModule(ctx context.Context, moduleName string, module ModuleInterface, ignoreIfExistses ...bool) (context.Context, error) {
	var err error
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := this.UserModules[canonicalModuleName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errModuleExists, canonicalModuleName)
		}
		ctx, err = this.unregisterModule(ctx, moduleName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterUserModule+": %s", canonicalModuleName, err.Error())
		}
	}
	newCtx, err := this.registerModule(ctx, canonicalModuleName, module, true)
	if err != nil {
		return ctx, fmt.Errorf(errRegisterUserModule+": %s", err.Error())
	}
	this.UserModules[canonicalModuleName] = module
	this.SortedUserModules = append(this.SortedUserModules, canonicalModuleName)
	return newCtx, nil
}

// Unregister As User Module
func (this *Instance) UnregisterUserModule(ctx context.Context, moduleNames ...string) (context.Context, error) {
	if len(moduleNames) > 0 {
		for _, moduleName := range moduleNames {
			canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
			_, ok := this.UserModules[canonicalModuleName]
			if !ok {
				return ctx, fmt.Errorf(errUserModuleNotExists, canonicalModuleName)
			}
			_, ok = this.UserModules[canonicalModuleName].(ModuleInterface)
			if !ok {
				return ctx, fmt.Errorf(errModuleInvaildType, canonicalModuleName)
			}
			for index, sortedModuleName := range this.SortedUserModules {
				if canonicalModuleName == sortedModuleName {
					this.SortedUserModules = append(this.SortedUserModules[:index], this.SortedUserModules[index+1:]...)
				}
			}
			delete(this.UserModules, canonicalModuleName)
			ctx, err := this.unregisterModule(ctx, canonicalModuleName)
			if err != nil {
				return ctx, fmt.Errorf(errUnregisterUserModule+": %s [module:'%s']", err.Error(), canonicalModuleName)
			}
		}
	}
	return ctx, nil
}

func (this *Instance) GetUserModule(moduleName string) (interface{}, error) {
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	if _, ok := this.UserModules[canonicalModuleName]; !ok {
		return nil, fmt.Errorf(errUserModuleNotExists, canonicalModuleName)
	}
	return this.UserModules[canonicalModuleName], nil
}

func (this *Instance) GetUserModules() map[string]interface{} {
	modules := make(map[string]interface{}, len(this.UserModules))
	for moduleName, module := range this.UserModules {
		modules[moduleName] = module
	}
	return modules
}

func (this *Instance) GetSortedUserModules(reverses ...bool) []interface{} {
	modules := make([]interface{}, 0)
	if len(this.SortedUserModules) > 0 {
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(this.SortedUserModules) - 1; i >= 0; i-- {
				if module, ok := this.UserModules[this.SortedUserModules[i]]; ok {
					modules = append(modules, module)
				}
			}
		} else {
			for i := 0; i < len(this.SortedUserModules); i++ {
				if module, ok := this.UserModules[this.SortedUserModules[i]]; ok {
					modules = append(modules, module)
				}
			}
		}
	}
	return modules
}

func (this *Instance) LoadModuleFileConfig(moduleName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	// create a Config object
	c := libConfig.New()
	err = c.Load(configFile)
	if err != nil {
		return fmt.Errorf(errModuleLoadConfig+": %s", canonicalModuleName, err.Error())
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return fmt.Errorf(errModuleRegisterConfigProvider+": %s", configProviderName, err.Error())
			}
		}
	}
	registeredModule, err := this.GetModule(canonicalModuleName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
		registeredModule, err = this.GetBuiltinModule(canonicalModuleName)
		if err != nil {
			return err
		}
		err = c.Configure(registeredModule, configTag...)
		if err != nil {
			return fmt.Errorf(errModuleConfigure+": %s", canonicalModuleName, err.Error())
		}
		this.Modules[canonicalModuleName] = registeredModule
	} else {
		err = c.Configure(registeredModule, configTag...)
		if err != nil {
			return fmt.Errorf(errModuleConfigure+": %s", canonicalModuleName, err.Error())
		}
		this.Modules[canonicalModuleName] = registeredModule
	}
	return nil
}

// Register As Module
func (this *Instance) LoadModuleJsonConfig(moduleName string, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
	// create a Config object
	c := libConfig.New()
	err = c.LoadJSON(configData)
	if err != nil {
		return fmt.Errorf(errModuleLoadConfig+": %s", canonicalModuleName, err.Error())
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return fmt.Errorf(errModuleRegisterConfigProvider+": %s", configProviderName, err.Error())
			}
		}
	}
	registeredModule, err := this.GetModule(canonicalModuleName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
		registeredModule, err = this.GetBuiltinModule(canonicalModuleName)
		if err != nil {
			return err
		}
		err = c.Configure(registeredModule, configTag...)
		if err != nil {
			return fmt.Errorf(errModuleConfigure+": %s", canonicalModuleName, err.Error())
		}
		this.Modules[canonicalModuleName] = registeredModule
	} else {
		err = c.Configure(registeredModule, configTag...)
		if err != nil {
			return fmt.Errorf(errModuleConfigure+": %s", canonicalModuleName, err.Error())
		}
		this.Modules[canonicalModuleName] = registeredModule
	}
	return nil
}

// Register As Value
func (this *Instance) RegisterValue(ctx context.Context, key string, value interface{}, forceOverwrites ...bool) (context.Context, error) {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	forceOverwrite := false
	if len(forceOverwrites) > 0 {
		forceOverwrite = forceOverwrites[0]
	}
	if _, ok := this.Values[Key]; ok && !forceOverwrite {
		return ctx, fmt.Errorf(errValueExists, Key)
	}
	this.Values[Key] = value
	return ctx, nil
}

// Unregister As Value
func (this *Instance) UnregisterValue(ctx context.Context, keys ...string) (context.Context, error) {
	if len(keys) > 0 {
		for _, key := range keys {
			Key := strings.ToUpper(key[0:1]) + key[1:]
			if _, ok := this.Values[Key]; !ok {
				return ctx, fmt.Errorf(errValueNotExists, Key)
			}
			delete(this.Values, Key)
		}
	}
	return ctx, nil
}

func (this *Instance) GetValue(key string) (interface{}, error) {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	if _, ok := this.Values[Key]; !ok {
		return nil, fmt.Errorf(errValueNotExists, Key)
	}
	return this.Values[Key], nil
}

func (this *Instance) GetValues() map[string]interface{} {
	return this.Values
}

func (this *Instance) InjectModule() error {
	// Modules
	modules := this.GetSortedModules()
	injectTargets := make([]interface{}, len(modules))
	for index, injectTarget := range modules {
		injectTargets[index] = injectTarget
	}
	return this.InjectModuleTo(injectTargets)
}

func (this *Instance) InjectModuleByName(moduleNames []string) error {
	// Modules
	modules := this.GetModules()
	injectTargets := make([]interface{}, 0)
	for _, moduleName := range moduleNames {
		canonicalModuleName := strings.ToUpper(moduleName[0:1]) + moduleName[1:]
		if injectTarget, ok := modules[canonicalModuleName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return this.InjectModuleTo(injectTargets)
}

func (this *Instance) InjectModuleTo(injectTargets []interface{}) error {
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
			if _, ok := this.BuiltinModules[f.Name()]; ok {
				err := f.Set(this.BuiltinModules[f.Name()])
				if err != nil {
					return fmt.Errorf(errInjectModuleTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
				}
			} else if _, ok := this.Modules[f.Name()]; ok {
				err := f.Set(this.Modules[f.Name()])
				if err != nil {
					return fmt.Errorf(errInjectModuleTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
				}
			} else if f.IsExported() && f.Tag(INJECT_TAG) == "true" {
				fieldInjected := false
				if _, ok := this.UserModules[f.Name()]; ok {
					err := f.Set(this.UserModules[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectModuleTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
					fieldInjected = true
				}
				if _, ok := this.Values[f.Name()]; ok {
					err := f.Set(this.Values[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectModuleTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
					fieldInjected = true
				}
				if !fieldInjected {
					return fmt.Errorf(errInjectModuleTo+": module or key not exists", injectTargetValue.String(), f.Name())
				}
			}
		}
	}
	return nil
}
