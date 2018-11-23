package ltick

import (
	"context"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/juju/errors"
	"github.com/ltick/tick-framework/database"
	"github.com/ltick/tick-framework/filesystem"
	"github.com/ltick/tick-framework/kvstore"
	"github.com/ltick/tick-framework/logger"
	"github.com/ltick/tick-framework/queue"
	"github.com/ltick/tick-framework/session"
	"github.com/ltick/tick-framework/utility"
)

var (
	errComponentExists                    = "ltick: component '%s' exists"
	errComponentNotExists                 = "ltick: component '%s' not exists"
	errRegisterComponent                  = "ltick: register component '%s' error"
	errUnregisterComponent                = "ltick: unregister component '%s' error"
	errInjectComponent                    = "ltick: inject component '%s' field '%s' error"
	errInjectComponentTo                  = "ltick: inject component '%s' field '%s' error"
	errUseComponent                       = "ltick: use component '%s' error"
	errValueExists                        = "ltick: value '%s' exists"
	errValueNotExists                     = "ltick: value '%s' not exists"
	errConfigureComponentFileConfigByName = "ltick: configure component '%s' file config error"
	errConfigureComponentFileConfig       = "ltick: configure component '%v' file config error"
)

type ComponentState int8

const (
	COMPONENT_STATE_INIT ComponentState = iota
	COMPONENT_STATE_PREPARED
	COMPONENT_STATE_INITIATED
	COMPONENT_STATE_STARTUP
	COMPONENT_STATE_SHUTDOWN
)

type ComponentInterface interface {
	Prepare(ctx context.Context) (context.Context, error)
	Initiate(ctx context.Context) (context.Context, error)
	OnStartup(ctx context.Context) (context.Context, error)
	OnShutdown(ctx context.Context) (context.Context, error)
}

type Components []*Component

func (cs Components) Get(name string) *Component {
	for _, c := range cs {
		canonicalName := canonicalName(name)
		if canonicalName == c.Name {
			return c
		}
	}
	return nil
}

type Component struct {
	Name          string
	Component     ComponentInterface
	ConfigurePath string
	Dependencies  []*Component
}

var (
	OptionalComponents = Components{
		&Component{Name: "Log", Component: &log.Logger{}, ConfigurePath: "components.log"},
		&Component{Name: "Database", Component: &database.Database{}, ConfigurePath: "components.database"},
		&Component{Name: "Kvstore", Component: &kvstore.Kvstore{}, ConfigurePath: "components.kvstore"},
		&Component{Name: "Queue", Component: &queue.Queue{}, ConfigurePath: "components.queue"},
		&Component{Name: "Filesystem", Component: &filesystem.Filesystem{}, ConfigurePath: "components.filesystem"},
		&Component{Name: "Session", Component: &session.Session{}, ConfigurePath: "components.session"},
	}
)

/**************** Component ****************/
func (r *Registry) UseComponent(componentNames ...string) error {
	var err error
	components := make([]*Component, 0)
	for _, componentName := range componentNames {
		canonicalComponentName := canonicalName(componentName)
		component := OptionalComponents.Get(canonicalComponentName)
		if component != nil {
			components = append(components, component)
			err = r.RegisterComponent(component, true)
			if err != nil {
				return errors.Annotatef(err, errUseComponent, component.Name)
			}
		} else {
			return errors.Annotatef(err, errComponentNotExists, canonicalComponentName)
		}
	}
	sortedComponents := SortComponent(components)
	for _, name := range sortedComponents {
		component, err := r.GetComponentByName(name)
		if err != nil {
			return errors.Annotatef(err, errUseComponent, name)
		}
		err = r.InjectComponentTo([]interface{}{component})
		if err != nil {
			return errors.Annotatef(err, errUseComponent, name)
		}
	}
	return nil
}

// Register As Component
func (r *Registry) RegisterComponent(component *Component, ignoreIfExistses ...bool) error {
	if _, ok := r.ComponentStates[component.Name]; !ok {
		r.ComponentStates[component.Name] = COMPONENT_STATE_INIT
	}
	canonicalName := canonicalName(component.Name)
	ignoreIfExists := false
	if len(ignoreIfExistses) > 0 {
		ignoreIfExists = ignoreIfExistses[0]
	}
	if _, ok := r.ComponentMap[canonicalName]; ok {
		if !ignoreIfExists {
			return errors.Errorf(errComponentExists, canonicalName)
		}
		err := r.UnregisterComponent(canonicalName)
		if err != nil {
			return errors.Annotatef(err, errRegisterComponent+": %s", canonicalName)
		}
	}
	r.SortedComponentName = append(r.SortedComponentName, canonicalName)
	r.Components = append(r.Components, component)
	r.ComponentMap[canonicalName] = component
	return nil
}

// Unregister As Component
func (r *Registry) UnregisterComponent(componentNames ...string) error {
	if len(componentNames) > 0 {
		for _, componentName := range componentNames {
			canonicalComponentName := canonicalName(componentName)
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
				if canonicalComponentName == c.Name {
					r.Components = append(r.Components[:index], r.Components[index+1:]...)
				}
			}
		}
	}
	return nil
}

func (r *Registry) GetComponentMap() map[string]*Component {
	return r.ComponentMap
}

func (r *Registry) GetComponentByName(name string) (*Component, error) {
	name = canonicalName(name)
	if _, ok := r.ComponentMap[name]; !ok {
		return nil, errors.Errorf(errComponentNotExists, name)
	}
	return r.ComponentMap[name], nil
}

func (r *Registry) GetSortedComponentName() []string {
	return r.SortedComponentName
}

func (r *Registry) GetSortedComponents(reverses ...bool) []*Component {
	components := make([]*Component, len(r.Components))
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

// Register As Value
func (r *Registry) RegisterValue(key string, value interface{}, forceOverwrites ...bool) error {
	key = canonicalName(key)
	forceOverwrite := false
	if len(forceOverwrites) > 0 {
		forceOverwrite = forceOverwrites[0]
	}
	if _, ok := r.Values[key]; ok && !forceOverwrite {
		return errors.Errorf(errValueExists, key)
	}
	r.Values[key] = value
	return nil
}

// Unregister As Value
func (r *Registry) UnregisterValue(keys ...string) error {
	if len(keys) > 0 {
		for _, key := range keys {
			key = canonicalName(key)
			if _, ok := r.Values[key]; !ok {
				return errors.Errorf(errValueNotExists, key)
			}
			delete(r.Values, key)
		}
	}
	return nil
}

func (r *Registry) GetValue(key string) (interface{}, error) {
	key = canonicalName(key)
	if _, ok := r.Values[key]; !ok {
		return nil, errors.Errorf(errValueNotExists, key)
	}
	return r.Values[key], nil
}

func (r *Registry) GetValues() map[string]interface{} {
	return r.Values
}

func (r *Registry) InjectComponent() error {
	components := r.GetSortedComponents()
	injectTargets := make([]interface{}, len(components))
	for i, c := range components {
		injectTargets[i] = c.Component
	}
	return r.InjectComponentTo(injectTargets)
}

func (r *Registry) InjectComponentByName(componentNames []string) error {
	componentMap := r.GetComponentMap()
	injectTargets := make([]interface{}, 0)
	for _, componentName := range componentNames {
		canonicalComponentName := canonicalName(componentName)
		if injectTarget, ok := componentMap[canonicalComponentName]; ok {
			injectTargets = append(injectTargets, injectTarget.Component)
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
							err := f.Set(r.ComponentMap[f.Name()].Component)
							if err != nil {
								return errors.Annotatef(err, errInjectComponentTo, injectTargetValue.String(), f.Name())
							}
							fieldInjected = true
						}
						if !fieldInjected {
							return errors.Errorf(errInjectComponentTo+": component or key not exists", injectTargetValue.String(), f.Name())
						}
					}
				}
				if _, ok := r.Values[f.Name()]; ok {
					err := f.Set(r.Values[f.Name()])
					if err != nil {
						return errors.Annotatef(err, errInjectComponentTo, injectTargetValue.String(), f.Name())
					}
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
	rootDependenciesCount := 0
	for {
		for _, c := range components {
			if len(c.Dependencies) == rootDependenciesCount {
				roots = append(roots, c)
			}
		}
		if len(roots) > 0 {
			break
		} else {
			rootDependenciesCount++
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

/******** Common Function ********/
func canonicalName(name string) string {
	return strings.ToUpper(name[0:1]) + name[1:]
}
