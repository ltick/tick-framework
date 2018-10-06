package ltick

import (
	"context"
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
	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-framework/filesystem"
	"github.com/ltick/tick-framework/session"
)

var (
	errComponentExists                 = "ltick: component '%s' exists"
	errComponentNotExists              = "ltick: component '%s' not exists"
	errComponentInvaildType            = "ltick: component '%s' invalid type"
	errComponentLoadConfig             = "ltick: component '%s' load config error"
	errComponentRegisterConfigProvider = "ltick: component '%s' register config provider error"
	errComponentConfigure              = "ltick: component '%s' configure error"
	errRegisterComponent               = "ltick: register component '%s' error"
	errUnregisterComponent             = "ltick: unregister component '%s' error"
	errInjectComponent                 = "ltick: inject component '%s' field '%s' error"
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
	Name         string
	Component    ComponentInterface
	Dependencies []*Component
}

var (
	BuiltinComponents = []*Component{
		&Component{Name: "Logger", Component: &logger.Logger{}},
		&Component{Name: "Config", Component: &config.Config{}},
	}
	Components = []*Component{
		&Component{Name: "Database", Component: &database.Database{}},
		&Component{Name: "Cache", Component: &cache.Cache{}},
		&Component{Name: "Queue", Component: &queue.Queue{}},
		&Component{Name: "Filesystem", Component: &filesystem.Filesystem{}},
		&Component{Name: "Session", Component: &session.Session{}},
	}
)

/**************** Component ****************/
// Register As Component
func (e *Engine) registerComponent(ctx context.Context, componentName string, component ComponentInterface, ignoreIfExistses ...bool) (context.Context, error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := e.ComponentMap[canonicalComponentName]; ok {
		if !ignoreIfExists {
			return ctx, fmt.Errorf(errComponentExists, canonicalComponentName)
		}
		ctx, err := e.unregisterComponent(ctx, canonicalComponentName)
		if err != nil {
			return ctx, fmt.Errorf(errRegisterComponent+": %s", canonicalComponentName, err.Error())
		}
	}
	e.Components = append(e.Components, component)
	e.ComponentMap[canonicalComponentName] = component
	e.SortedComponents = append(e.SortedComponents, canonicalComponentName)
	return ctx, nil
}

// Unregister As Component
func (e *Engine) unregisterComponent(ctx context.Context, componentNames ...string) (context.Context, error) {
	if len(componentNames) > 0 {
		for _, componentName := range componentNames {
			canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
			// e.ComponentMap
			delete(e.ComponentMap, canonicalComponentName)
			// e.SortedComponents
			for index, sortedComponentName := range e.SortedComponents {
				if canonicalComponentName == sortedComponentName {
					e.SortedComponents = append(e.SortedComponents[:index], e.SortedComponents[index+1:]...)
				}
			}
			// e.Components
			for index, c := range e.Components {
				if component, ok := c.(*Component); ok {
					if canonicalComponentName == component.Name {
						e.Components = append(e.Components[:index], e.Components[index+1:]...)
					}
				}
			}
		}
	}
	return ctx, nil
}

func (e *Engine) GetComponentByName(componentName string) (interface{}, error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	if _, ok := e.ComponentMap[canonicalComponentName]; !ok {
		return nil, fmt.Errorf(errComponentNotExists, canonicalComponentName)
	}
	return e.ComponentMap[canonicalComponentName], nil
}

func (e *Engine) GetComponentMap() map[string]interface{} {
	return e.ComponentMap
}

func (e *Engine) GetSortedComponents(reverses ...bool) []interface{} {
	components := make([]interface{}, len(e.Components))
	if len(e.SortedComponents) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(e.SortedComponents) - 1; i >= 0; i-- {
				if component, ok := e.ComponentMap[e.SortedComponents[i]]; ok {
					components[index] = component
					index++
				}
			}
		} else {
			for i := 0; i < len(e.SortedComponents); i++ {
				if component, ok := e.ComponentMap[e.SortedComponents[i]]; ok {
					components[index] = component
					index++
				}
			}
		}
	}
	return components
}

func (e *Engine) UseComponent(componentNames ...string) error {
	var err error
	// 内置模块注册
	components := make([]*Component, 0)
	for _, componentName := range componentNames {
		canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		componentExists := false
		for _, component := range Components {
			canonicalExistsComponentName := strings.ToUpper(component.Name[0:1]) + component.Name[1:]
			if canonicalComponentName == canonicalExistsComponentName {
				componentExists = true
				components = append(components, component)
				e.ComponentMap[component.Name] = component.Component
				e.Components = append(e.Components, component.Component)
				e.Context, err = e.registerComponent(e.Context, strings.ToLower(component.Name[0:1])+component.Name[1:], component.Component, true)
				if err != nil {
					return fmt.Errorf(errRegisterComponent+": %s", component.Name, err.Error())
				}
			}
		}
		if !componentExists {
			return fmt.Errorf(errComponentNotExists, canonicalComponentName)
		}
	}
	sortedComponents := SortComponent(components)
	for _, name := range sortedComponents {
		err = e.InjectComponentTo([]interface{}{e.ComponentMap[name]})
		if err != nil {
			return fmt.Errorf(errInjectComponent+": %s", name, err.Error())
		}
	}
	return nil
}

func (e *Engine) GetSortedComponent(reverses ...bool) []interface{} {
	components := make([]interface{}, 0)
	if len(e.SortedComponents) > 0 {
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(e.SortedComponents) - 1; i >= 0; i-- {
				if component, ok := e.ComponentMap[e.SortedComponents[i]]; ok {
					components = append(components, component)
				}
			}
		} else {
			for i := 0; i < len(e.SortedComponents); i++ {
				if component, ok := e.ComponentMap[e.SortedComponents[i]]; ok {
					components = append(components, component)
				}
			}
		}
	}
	return components
}

func (e *Engine) LoadComponentFileConfig(componentName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
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
	registeredComponent, err := e.GetComponentByName(canonicalComponentName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
	}
	err = c.Configure(registeredComponent, configTag...)
	if err != nil {
		return fmt.Errorf(errComponentConfigure+": %s", canonicalComponentName, err.Error())
	}
	e.ComponentMap[canonicalComponentName] = registeredComponent
	return nil
}

// Register As Component
func (e *Engine) LoadComponentJsonConfig(componentName string, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
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
	registeredComponent, err := e.GetComponentByName(canonicalComponentName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
	}
	err = c.Configure(registeredComponent, configTag...)
	if err != nil {
		return fmt.Errorf(errComponentConfigure+": %s", canonicalComponentName, err.Error())
	}
	e.ComponentMap[canonicalComponentName] = registeredComponent
	return nil
}

// Register As Value
func (e *Engine) RegisterValue(key string, value interface{}, forceOverwrites ...bool) error {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	forceOverwrite := false
	if len(forceOverwrites) > 0 {
		forceOverwrite = forceOverwrites[0]
	}
	if _, ok := e.Values[Key]; ok && !forceOverwrite {
		return fmt.Errorf(errValueExists, Key)
	}
	e.Values[Key] = value
	return nil
}

// Unregister As Value
func (e *Engine) UnregisterValue(ctx context.Context, keys ...string) (context.Context, error) {
	if len(keys) > 0 {
		for _, key := range keys {
			Key := strings.ToUpper(key[0:1]) + key[1:]
			if _, ok := e.Values[Key]; !ok {
				return ctx, fmt.Errorf(errValueNotExists, Key)
			}
			delete(e.Values, Key)
		}
	}
	return ctx, nil
}

func (e *Engine) GetValue(key string) (interface{}, error) {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	if _, ok := e.Values[Key]; !ok {
		return nil, fmt.Errorf(errValueNotExists, Key)
	}
	return e.Values[Key], nil
}

func (e *Engine) GetValues() map[string]interface{} {
	return e.Values
}

func (e *Engine) InjectComponent() error {
	return e.InjectComponentTo(e.GetSortedComponents())
}

func (e *Engine) InjectComponentByName(componentNames []string) error {
	componentMap := e.GetComponentMap()
	injectTargets := make([]interface{}, 0)
	for _, componentName := range componentNames {
		canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		if injectTarget, ok := componentMap[canonicalComponentName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return e.InjectComponentTo(injectTargets)
}

func (e *Engine) InjectComponentTo(injectTargets []interface{}) error {
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
			if f.IsExported() && f.Tag(INJECT_TAG) == "true" {
				fieldInjected := false
				if _, ok := e.ComponentMap[f.Name()]; ok {
					err := f.Set(e.ComponentMap[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
					fieldInjected = true
				}
				if _, ok := e.Values[f.Name()]; ok {
					err := f.Set(e.Values[f.Name()])
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

/******** Component Dependency Manage ********/
// SortComponent - Sort user components.
func SortComponent(components []*Component) []string {
	// 初始化依赖关系
	for _, c := range components {
		if c.Dependencies == nil {
			c.Dependencies = make([]*Component, 0)
		}
		s := structs.New(c.Component)
		componentType := reflect.TypeOf((*ComponentInterface)(nil)).Elem()
		for _, f := range s.Fields() {
			if f.IsExported() && f.Tag(INJECT_TAG) == "true" {
				if reflect.TypeOf(f.Value()).Implements(componentType) {
					if dc, ok := f.Value().(ComponentInterface); ok {
						c.Dependencies = append(c.Dependencies, &Component{
							Name:      f.Name(),
							Component: dc,
						})
					}
				}
			}
		}
	}
	// root components
	roots := []*Component{}
	for _, c := range components {
		if len(c.Dependencies) == 0 {
			roots = append(roots, c)
		}
	}
	sortedComponents := make([]string, 0)
	return sortComponent(components, roots, sortedComponents)
}

// sortComponent
func sortComponent(components []*Component, currentComponents []*Component, sortedComponents []string) []string {
	if components != nil && currentComponents != nil && sortedComponents != nil {
		// 没有下级依赖
		if len(currentComponents) == 0 {
			return sortedComponents
		}
		for _, currentComponent := range currentComponents {
			// 当前层级组件
			index := utility.InArrayString(currentComponent.Name, sortedComponents, false)
			if index == nil {
				sortedComponents = append(sortedComponents, currentComponent.Name)
			} else {
				sortedComponents = append(sortedComponents[:*index], append(sortedComponents[*index+1:], sortedComponents[*index])...)
			}
			// 依赖当前级别组件的组件
			componentsNextLevel := make([]*Component, 0)
			for _, component := range components {
				for _, componentDependencie := range component.Dependencies {
					if strings.Compare(componentDependencie.Name, currentComponent.Name) == 0 {
						componentsNextLevel = append(componentsNextLevel, component)
					}
				}
			}
			sortedComponents = sortComponent(components, componentsNextLevel, sortedComponents)
		}
	}
	return sortedComponents
}
