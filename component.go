package ltick

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	libConfig "github.com/go-ozzo/ozzo-config"
	"github.com/ltick/tick-framework/cache"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/database"
	"github.com/ltick/tick-framework/logger"
	"github.com/ltick/tick-framework/queue"
)

var (
	errComponentExists                 = "ltick: component '%s' exists"
	errComponentNotExists              = "ltick: component '%s' not exists"
	errUserComponentNotExists          = "ltick: user component '%s' not exists"
	errBuiltinComponentNotExists       = "ltick: builtin component '%s' not exists"
	errComponentInvaildType            = "ltick: component '%s' invalid type"
	errComponentLoadConfig             = "ltick: component '%s' load config error"
	errComponentRegisterConfigProvider = "ltick: component '%s' register config provider error"
	errComponentConfigure              = "ltick: component '%s' configure error"
	errRegisterComponent               = "ltick: register component '%s' error"
	errUnregisterComponent             = "ltick: unregister component '%s' error"
	errRegisterUserComponent           = "ltick: register user component '%s' error"
	errUnregisterUserComponent         = "ltick: unregister user component '%s' error"
	errRegisterBuiltinComponent        = "ltick: register builtin component '%s' error"
	errUnregisterBuiltinComponent      = "ltick: unregister builtin component '%s' error"
	errInjectComponent                 = "ltick: inject builtin component '%s' field '%s' error"
	errInjectComponentTo               = "ltick: inject component '%s' field '%s' error"
	errUseComponent                    = "ltick: use component error"
	errValueExists                     = "ltick: value '%s' exists"
	errValueNotExists                  = "ltick: value '%s' not exists"
)

const INJECT_TAG = "inject"

type ComponentInterface interface {
	Initiate(ctx context.Context) (context.Context, error)
	OnStartup(ctx context.Context) (context.Context, error)
	OnShutdown(ctx context.Context) (context.Context, error)
}

type Component struct {
	Name      string
	Component ComponentInterface
}

var (
	BuiltinComponents = []*Component{
		&Component{Name: "Logger", Component: &logger.Logger{}},
		&Component{Name: "Config", Component: &config.Config{}},
	}
	Components                                   = []*Component{
		&Component{Name: "Database", Component: &database.Database{}},
		&Component{Name: "Cache", Component: &cache.Cache{}},
		&Component{Name: "Queue", Component: &queue.Queue{}},
	}
	BuiltinComponentMap   map[string]interface{} = make(map[string]interface{})
	BuiltinComponentOrder []string               = make([]string, 0)
	UserComponentMap   map[string]interface{} = make(map[string]interface{})
	UserComponentOrder []string               = make([]string, 0)
	ComponentMap       map[string]interface{} = make(map[string]interface{})
	ComponentOrder     []string               = make([]string, 0)
	Values             map[string]interface{} = make(map[string]interface{})
)

/**************** Builtin Component ****************/
// Register As BuiltinComponent
func registerBuiltinComponent(ctx context.Context, componentName string, component ComponentInterface, ignoreIfExistses ...bool) (newCtx context.Context, err error) {
	newCtx, err = registerComponent(ctx, componentName, component, ignoreIfExistses...)
	if err != nil {
		return newCtx, errors.New(errRegisterBuiltinComponent + ": " + err.Error())
	}
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	BuiltinComponentMap[canonicalComponentName] = component
	BuiltinComponentOrder = append(BuiltinComponentOrder, canonicalComponentName)
	return newCtx, nil
}

// Unregister As BuiltinComponent
func unregisterBuiltinComponent(ctx context.Context, componentNames ...string) (newCtx context.Context, err error) {
	newCtx, err = unregisterComponent(ctx, componentNames...)
	if err != nil {
		return newCtx, errors.New(errUnregisterBuiltinComponent + ": " + err.Error())
	}
	for _, componentName := range componentNames {
		canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		delete(BuiltinComponentMap, canonicalComponentName)
	}
	return ctx, nil
}

func GetBuiltinComponent(componentName string) (interface{}, error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	if _, ok := BuiltinComponentMap[canonicalComponentName]; !ok {
		return nil, fmt.Errorf(errBuiltinComponentNotExists, canonicalComponentName)
	}
	return BuiltinComponentMap[canonicalComponentName], nil
}

func GetBuiltinComponents() map[string]interface{} {
	components := make(map[string]interface{}, len(BuiltinComponents))
	for componentName, component := range BuiltinComponentMap {
		components[componentName] = component
	}
	return components
}

func GetSortedBuiltinComponents(reverses ...bool) []interface{} {
	components := make([]interface{}, len(BuiltinComponents))
	if len(BuiltinComponentOrder) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(BuiltinComponentOrder) - 1; i >= 0; i-- {
				if component, ok := BuiltinComponentMap[BuiltinComponentOrder[i]]; ok {
					components[index] = component
					index++
				}
			}
		} else {
			for i := 0; i < len(BuiltinComponentOrder); i++ {
				if component, ok := BuiltinComponentMap[BuiltinComponentOrder[i]]; ok {
					components[index] = component
					index++
				}
			}
		}
	}
	return components
}

/**************** Component ****************/
// Register As Component
func registerComponent(ctx context.Context, componentName string, component ComponentInterface, ignoreIfExistses ...bool) (context.Context, error) {
	var err error
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := ComponentMap[canonicalComponentName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errComponentExists, canonicalComponentName)
		}
		ctx, err = unregisterComponent(ctx, canonicalComponentName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterComponent+": %s", canonicalComponentName, err.Error())
		}
	}
	err = InjectComponentTo([]interface{}{component})
	if err != nil {
		return ctx, fmt.Errorf(errRegisterComponent+": %s", canonicalComponentName, err.Error())
	}
	newCtx, err := component.Initiate(ctx)
	if err != nil {
		return ctx, fmt.Errorf(errRegisterComponent+": %s", canonicalComponentName, err.Error())
	}
	ComponentMap[canonicalComponentName] = component
	ComponentOrder = append(ComponentOrder, canonicalComponentName)
	return newCtx, nil
}

// Unregister As Component
func unregisterComponent(ctx context.Context, componentNames ...string) (context.Context, error) {
	if len(componentNames) > 0 {
		for _, componentName := range componentNames {
			canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
			_, ok := ComponentMap[canonicalComponentName]
			if !ok {
				return ctx, fmt.Errorf(errComponentNotExists, canonicalComponentName)
			}
			for index, sortedComponentName := range ComponentOrder {
				if canonicalComponentName == sortedComponentName {
					ComponentOrder = append(ComponentOrder[:index], ComponentOrder[index+1:]...)
				}
			}
			delete(ComponentMap, canonicalComponentName)
		}
	}
	return ctx, nil
}

func GetComponent(componentName string) (interface{}, error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	if _, ok := ComponentMap[canonicalComponentName]; !ok {
		return nil, fmt.Errorf(errComponentNotExists, canonicalComponentName)
	}
	return ComponentMap[canonicalComponentName], nil
}

func GetComponents() map[string]interface{} {
	components := make(map[string]interface{}, len(Components))
	for componentName, component := range ComponentMap {
		components[componentName] = component
	}
	return components
}

func GetSortedComponents(reverses ...bool) []interface{} {
	components := make([]interface{}, len(Components))
	if len(ComponentOrder) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(ComponentOrder) - 1; i >= 0; i-- {
				if component, ok := ComponentMap[ComponentOrder[i]]; ok {
					components[index] = component
					index++
				}
			}
		} else {
			for i := 0; i < len(ComponentOrder); i++ {
				if component, ok := ComponentMap[ComponentOrder[i]]; ok {
					components[index] = component
					index++
				}
			}
		}
	}
	return components
}

func UseComponent(ctx context.Context, componentNames ...string) (context.Context, error) {
	var err error
	// 内置模块注册
	for _, componentName := range componentNames {
		canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		componentTargets := make([]interface{}, 0)
		componentInterfaces := make([]ComponentInterface, 0)
		componentExists := false
		for _, component := range Components {
			canonicalExistsComponentName := strings.ToUpper(component.Name[0:1]) + component.Name[1:]
			if canonicalComponentName == canonicalExistsComponentName {
				componentExists = true
				componentTargets = append(componentTargets, component.Component)
				componentInterfaces = append(componentInterfaces, component.Component)
			}
		}
		if !componentExists {
			return ctx, fmt.Errorf(errComponentNotExists, canonicalComponentName)
		}
		err = InjectComponentTo(componentTargets)
		if err != nil {
			return ctx, fmt.Errorf(errUseComponent+": %s", err.Error())
		}
		for _, componentInterface := range componentInterfaces {
			ctx, err = registerComponent(ctx, canonicalComponentName, componentInterface, true)
			if err != nil {
				return ctx, fmt.Errorf(errUseComponent+": %s", err.Error())
			}
		}
	}
	return ctx, nil
}

/**************** User Component ****************/
// Register As User Component
func RegisterUserComponent(ctx context.Context, componentName string, component ComponentInterface, ignoreIfExistses ...bool) (context.Context, error) {
	var err error
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := UserComponentMap[canonicalComponentName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errComponentExists, canonicalComponentName)
		}
		ctx, err = unregisterComponent(ctx, componentName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterUserComponent+": %s", canonicalComponentName, err.Error())
		}
	}
	newCtx, err := registerComponent(ctx, canonicalComponentName, component, true)
	if err != nil {
		return ctx, fmt.Errorf(errRegisterUserComponent+": %s", err.Error())
	}
	UserComponentMap[canonicalComponentName] = component
	UserComponentOrder = append(UserComponentOrder, canonicalComponentName)
	return newCtx, nil
}

// Unregister As User Component
func UnregisterUserComponent(ctx context.Context, componentNames ...string) (context.Context, error) {
	if len(componentNames) > 0 {
		for _, componentName := range componentNames {
			canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
			_, ok := UserComponentMap[canonicalComponentName]
			if !ok {
				return ctx, fmt.Errorf(errUserComponentNotExists, canonicalComponentName)
			}
			_, ok = UserComponentMap[canonicalComponentName].(ComponentInterface)
			if !ok {
				return ctx, fmt.Errorf(errComponentInvaildType, canonicalComponentName)
			}
			for index, sortedComponentName := range UserComponentOrder {
				if canonicalComponentName == sortedComponentName {
					UserComponentOrder = append(UserComponentOrder[:index], UserComponentOrder[index+1:]...)
				}
			}
			delete(UserComponentMap, canonicalComponentName)
			ctx, err := unregisterComponent(ctx, canonicalComponentName)
			if err != nil {
				return ctx, fmt.Errorf(errUnregisterUserComponent+": %s [component:'%s']", err.Error(), canonicalComponentName)
			}
		}
	}
	return ctx, nil
}

func GetUserComponent(componentName string) (interface{}, error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	if _, ok := UserComponentMap[canonicalComponentName]; !ok {
		return nil, fmt.Errorf(errUserComponentNotExists, canonicalComponentName)
	}
	return UserComponentMap[canonicalComponentName], nil
}

func GetUserComponentMap() map[string]interface{} {
	components := make(map[string]interface{}, len(UserComponentMap))
	for componentName, component := range UserComponentMap {
		components[componentName] = component
	}
	return components
}

func GetSortedUseComponent(reverses ...bool) []interface{} {
	components := make([]interface{}, 0)
	if len(UserComponentOrder) > 0 {
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(UserComponentOrder) - 1; i >= 0; i-- {
				if component, ok := UserComponentMap[UserComponentOrder[i]]; ok {
					components = append(components, component)
				}
			}
		} else {
			for i := 0; i < len(UserComponentOrder); i++ {
				if component, ok := UserComponentMap[UserComponentOrder[i]]; ok {
					components = append(components, component)
				}
			}
		}
	}
	return components
}

func LoadComponentFileConfig(componentName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	// create a Config object
	c := libConfig.New()
	err = c.Load(configFile)
	if err != nil {
		return fmt.Errorf(errComponentLoadConfig+": %s", canonicalComponentName, err.Error())
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return fmt.Errorf(errComponentRegisterConfigProvider+": %s", configProviderName, err.Error())
			}
		}
	}
	registeredComponent, err := GetComponent(canonicalComponentName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
		registeredComponent, err = GetBuiltinComponent(canonicalComponentName)
		if err != nil {
			return err
		}
		err = c.Configure(registeredComponent, configTag...)
		if err != nil {
			return fmt.Errorf(errComponentConfigure+": %s", canonicalComponentName, err.Error())
		}
		ComponentMap[canonicalComponentName] = registeredComponent
	} else {
		err = c.Configure(registeredComponent, configTag...)
		if err != nil {
			return fmt.Errorf(errComponentConfigure+": %s", canonicalComponentName, err.Error())
		}
		ComponentMap[canonicalComponentName] = registeredComponent
	}
	return nil
}

// Register As Component
func LoadComponentJsonConfig(componentName string, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	// create a Config object
	c := libConfig.New()
	err = c.LoadJSON(configData)
	if err != nil {
		return fmt.Errorf(errComponentLoadConfig+": %s", canonicalComponentName, err.Error())
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return fmt.Errorf(errComponentRegisterConfigProvider+": %s", configProviderName, err.Error())
			}
		}
	}
	registeredComponent, err := GetComponent(canonicalComponentName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
		registeredComponent, err = GetBuiltinComponent(canonicalComponentName)
		if err != nil {
			return err
		}
		err = c.Configure(registeredComponent, configTag...)
		if err != nil {
			return fmt.Errorf(errComponentConfigure+": %s", canonicalComponentName, err.Error())
		}
		ComponentMap[canonicalComponentName] = registeredComponent
	} else {
		err = c.Configure(registeredComponent, configTag...)
		if err != nil {
			return fmt.Errorf(errComponentConfigure+": %s", canonicalComponentName, err.Error())
		}
		ComponentMap[canonicalComponentName] = registeredComponent
	}
	return nil
}

// Register As Value
func RegisterValue(ctx context.Context, key string, value interface{}, forceOverwrites ...bool) (context.Context, error) {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	forceOverwrite := false
	if len(forceOverwrites) > 0 {
		forceOverwrite = forceOverwrites[0]
	}
	if _, ok := Values[Key]; ok && !forceOverwrite {
		return ctx, fmt.Errorf(errValueExists, Key)
	}
	Values[Key] = value
	return ctx, nil
}

// Unregister As Value
func UnregisterValue(ctx context.Context, keys ...string) (context.Context, error) {
	if len(keys) > 0 {
		for _, key := range keys {
			Key := strings.ToUpper(key[0:1]) + key[1:]
			if _, ok := Values[Key]; !ok {
				return ctx, fmt.Errorf(errValueNotExists, Key)
			}
			delete(Values, Key)
		}
	}
	return ctx, nil
}

func GetValue(key string) (interface{}, error) {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	if _, ok := Values[Key]; !ok {
		return nil, fmt.Errorf(errValueNotExists, Key)
	}
	return Values[Key], nil
}

func GetValues() map[string]interface{} {
	return Values
}

func InjectComponent() error {
	// Components
	components := GetSortedComponents()
	injectTargets := make([]interface{}, len(components))
	for index, injectTarget := range components {
		injectTargets[index] = injectTarget
	}
	return InjectComponentTo(injectTargets)
}

func InjectComponentByName(componentNames []string) error {
	// Components
	components := GetComponents()
	injectTargets := make([]interface{}, 0)
	for _, componentName := range componentNames {
		canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		if injectTarget, ok := components[canonicalComponentName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return InjectComponentTo(injectTargets)
}

func InjectComponentTo(injectTargets []interface{}) error {
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
			if _, ok := BuiltinComponentMap[f.Name()]; ok {
				err := f.Set(BuiltinComponentMap[f.Name()])
				if err != nil {
					return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
				}
			} else if _, ok := ComponentMap[f.Name()]; ok {
				err := f.Set(ComponentMap[f.Name()])
				if err != nil {
					return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
				}
			} else if f.IsExported() && f.Tag(INJECT_TAG) == "true" {
				fieldInjected := false
				if _, ok := UserComponentMap[f.Name()]; ok {
					err := f.Set(UserComponentMap[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
					fieldInjected = true
				}
				if _, ok := Values[f.Name()]; ok {
					err := f.Set(Values[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
					fieldInjected = true
				}
				if !fieldInjected {
					return fmt.Errorf(errInjectComponentTo+": component or key not exists", injectTargetValue.String(), f.Name())
				}
			}
		}
	}
	return nil
}
