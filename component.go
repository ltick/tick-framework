package ltick

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/juju/errors"
	"github.com/ltick/tick-framework/cache"
	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/database"
	"github.com/ltick/tick-framework/filesystem"
	"github.com/ltick/tick-framework/logger"
	"github.com/ltick/tick-framework/queue"
	"github.com/ltick/tick-framework/session"
	"github.com/ltick/tick-framework/utility"
)

var (
	errComponentExists                 = "ltick: component '%s' exists"
	errComponentNotExists              = "ltick: component '%s' not exists"
	errRegisterComponent               = "ltick: register component '%s' error"
	errUnregisterComponent             = "ltick: unregister component '%s' error"
	errInjectComponent                 = "ltick: inject component '%s' field '%s' error"
	errInjectComponentTo               = "ltick: inject component '%s' field '%s' error"
	errUseComponent                    = "ltick: use component error"
	errValueExists                     = "ltick: value '%s' exists"
	errValueNotExists                  = "ltick: value '%s' not exists"
)

func (r *Registry) GetComponentMap() map[string]interface{} {
	return r.ComponentMap
}

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
		&Component{Name: "Log", Component: &log.Logger{}},
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

func (r *Registry) UseComponent(componentNames ...string) error {
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
				err = r.RegisterComponent(strings.ToLower(component.Name[0:1])+component.Name[1:], component.Component, true)
				if err != nil {
					return fmt.Errorf(errUseComponent+": %s", component.Name, err.Error())
				}
			}
		}
		if !componentExists {
			return fmt.Errorf(errComponentNotExists, canonicalComponentName)
		}
	}
	sortedComponents := SortComponent(components)
	for _, name := range sortedComponents {
		component, err := r.GetComponentByName(name)
		if err != nil {
			return fmt.Errorf(errUseComponent+": %s", name, err.Error())
		}
		err = r.InjectComponentTo([]interface{}{component})
		if err != nil {
			return fmt.Errorf(errUseComponent+": %s", name, err.Error())
		}
	}
	return nil
}

/**************** Component ****************/
// Register As Component
func (r *Registry) RegisterComponent(componentName string, component ComponentInterface, ignoreIfExistses ...bool) error {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := r.ComponentMap[canonicalComponentName]; ok {
		if !ignoreIfExists {
			return fmt.Errorf(errComponentExists, canonicalComponentName)
		}
		err := r.UnregisterComponent(canonicalComponentName)
		if err != nil {
			return fmt.Errorf(errRegisterComponent+": %s", canonicalComponentName, err.Error())
		}
	}
	r.Components = append(r.Components, component)
	r.ComponentMap[canonicalComponentName] = component
	r.SortedComponentName = append(r.SortedComponentName, canonicalComponentName)
	return nil
}

// Unregister As Component
func (r *Registry) UnregisterComponent(componentNames ...string) error {
	if len(componentNames) > 0 {
		for _, componentName := range componentNames {
			canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
			// r.ComponentMap
			delete(r.ComponentMap, canonicalComponentName)
			// r.SortedComponentName
			for index, sortedComponentName := range r.SortedComponentName {
				if canonicalComponentName == sortedComponentName {
					r.SortedComponentName = append(r.SortedComponentName[:index], r.SortedComponentName[index+1:]...)
				}
			}
			// r.Components
			for index, c := range r.Components {
				if component, ok := c.(*Component); ok {
					if canonicalComponentName == component.Name {
						r.Components = append(r.Components[:index], r.Components[index+1:]...)
					}
				}
			}
		}
	}
	return nil
}

func (r *Registry) GetComponentByName(componentName string) (interface{}, error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	if _, ok := r.ComponentMap[canonicalComponentName]; !ok {
		return nil, fmt.Errorf(errComponentNotExists, canonicalComponentName)
	}
	return r.ComponentMap[canonicalComponentName], nil
}

func (r *Registry) GetSortedComponents(reverses ...bool) []interface{} {
	components := make([]interface{}, len(r.Components))
	if len(r.SortedComponentName) > 0 {
		index := 0
		reverse := false
		if len(reverses) > 0 {
			reverse = reverses[0]
		}
		if reverse {
			for i := len(r.SortedComponentName) - 1; i >= 0; i-- {
				if component, ok := r.ComponentMap[r.SortedComponentName[i]]; ok {
					components[index] = component
					index++
				}
			}
		} else {
			for i := 0; i < len(r.SortedComponentName); i++ {
				if component, ok := r.ComponentMap[r.SortedComponentName[i]]; ok {
					components[index] = component
					index++
				}
			}
		}
	}
	return components
}

func (r *Registry) GetSortedComponentName() []string {
	return r.SortedComponentName
}

// Register As Value
func (r *Registry) RegisterValue(key string, value interface{}, forceOverwrites ...bool) error {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	forceOverwrite := false
	if len(forceOverwrites) > 0 {
		forceOverwrite = forceOverwrites[0]
	}
	if _, ok := r.Values[Key]; ok && !forceOverwrite {
		return fmt.Errorf(errValueExists, Key)
	}
	r.Values[Key] = value
	return nil
}

// Unregister As Value
func (r *Registry) UnregisterValue(keys ...string) error {
	if len(keys) > 0 {
		for _, key := range keys {
			Key := strings.ToUpper(key[0:1]) + key[1:]
			if _, ok := r.Values[Key]; !ok {
				return fmt.Errorf(errValueNotExists, Key)
			}
			delete(r.Values, Key)
		}
	}
	return nil
}

func (r *Registry) GetValue(key string) (interface{}, error) {
	Key := strings.ToUpper(key[0:1]) + key[1:]
	if _, ok := r.Values[Key]; !ok {
		return nil, fmt.Errorf(errValueNotExists, Key)
	}
	return r.Values[Key], nil
}

func (r *Registry) GetValues() map[string]interface{} {
	return r.Values
}

func (r *Registry) InjectComponent() error {
	return r.InjectComponentTo(r.GetSortedComponents())
}

func (r *Registry) InjectComponentByName(componentNames []string) error {
	componentMap := r.GetComponentMap()
	injectTargets := make([]interface{}, 0)
	for _, componentName := range componentNames {
		canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
		if injectTarget, ok := componentMap[canonicalComponentName]; ok {
			injectTargets = append(injectTargets, injectTarget)
		}
	}
	return r.InjectComponentTo(injectTargets)
}

func (r *Registry) InjectComponentTo(injectTargets []interface{}) error {
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
								return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
							}
							fieldInjected = true
						}
						if !fieldInjected {
							return fmt.Errorf(errInjectComponentTo+": component or key not exists", injectTargetValue.String(), f.Name())
						}
					}
				}
				if _, ok := r.Values[f.Name()]; ok {
					err := f.Set(r.Values[f.Name()])
					if err != nil {
						return fmt.Errorf(errInjectComponentTo+": %s", injectTargetValue.String(), f.Name(), err.Error())
					}
				}
			}
		}
	}
	return nil
}

func (r *Registry) LoadComponentFileConfig(componentName string, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	canonicalComponentName := strings.ToUpper(componentName[0:1]) + componentName[1:]
	component, err := r.GetComponentByName(canonicalComponentName)
	if err != nil {
		if !strings.Contains(err.Error(), "not exists") {
			return err
		}
	}
	// configer
	configComponent, err := r.GetComponentByName("Config")
	if err != nil {
		e := errors.Annotate(err, errLoadComponentFileConfig)
		fmt.Println(errors.ErrorStack(e))
	}
	configer, ok := configComponent.(*config.Config)
	if !ok {
		e := errors.Annotate(errors.Errorf("invalid 'Config' component type"), errLoadComponentFileConfig)
		fmt.Println(errors.ErrorStack(e))
	}
	// create a Config object
	err = configer.LoadComponentFileConfig(component, componentName, configFile, configProviders, configTag...)
	if err != nil {
		return fmt.Errorf(errLoadComponentFileConfig+": %s", canonicalComponentName, err.Error())
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
