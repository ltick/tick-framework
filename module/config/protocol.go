package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate           = "config: initiate '%s' error"
	errConfigure          = "config: configure error"
	errReload             = "config: reload error"
	errLoadFromEnvFile    = "config: load from env file error"
	errLoadFromEnv        = "config: load from env error"
	errLoadFromConfigFile = "config: load from config file error"
	errLoadFromConfigPath = "config: load from config path error"
)

type Type uint

const (
	Invalid Type = iota
	String
	Bool
	Int
	Int64
	Float64
	Time
	Duration
	StringSlice
	StringMap
	StringMapString
	StringMapStringSlice
	SizeInBytes
)

type Callback func(ctx context.Context, value interface{}) (interface{}, error)

type Option struct {
	Type           Type
	Default        interface{}
	EnvironmentKey string
}

func NewInstance() *Instance {
	instance := &Instance{}
	return instance
}

type Instance struct {
	handlerName string
	handler     Handler

	options         map[string]Option
	bindedEnvironmentKeys []string
	pathPrefix      string
}

func (this *Instance) Initiate(ctx context.Context) (context.Context, error) {
	this.options = make(map[string]Option)
	err := Register("viper", NewViperHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	err = this.Use(ctx, "viper")
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return ctx, nil
}
func (this *Instance) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestStartup(ctx context.Context, c *routing.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestShutdown(ctx context.Context, c *routing.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) HandlerName() string {
	return this.handlerName
}
func (this *Instance) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	this.handlerName = handlerName
	this.handler = handler()
	err = this.handler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return nil
}

func (this *Instance) AddConfigPath(in string) {
	this.handler.AddConfigPath(in)
}
func (this *Instance) ConfigFileUsed() string {
	return this.handler.ConfigFileUsed()
}
func (this *Instance) SetEnvPrefix(in string) {
	this.handler.SetEnvPrefix(in)
}
func (this *Instance) SetPathPrefix(pathPrefix string) {
	this.pathPrefix = pathPrefix
}
func (this *Instance) GetPathPrefix() string {
	return this.pathPrefix
}

func (this *Instance) SetOptions(ctx context.Context, options map[string]Option) (context.Context, error) {
	if options != nil {
		keys := make([]string, 0)
		for key, c := range options {
			if key != "" {
				if _, ok := this.options[key]; !ok {
					this.options[key] = c
				}
				keys = append(keys, key)
			}
			if c.Default != nil {
				this.handler.SetDefault(key, c.Default)
			}
		}
	}
	return ctx, nil
}
func (this *Instance) Callbacks(ctx context.Context, callbacks map[string]Callback) (context.Context, error) {
	if callbacks != nil {
		for key, callback := range callbacks {
			value := this.GetValue(key)
			if value != nil {
				value, err := callback(ctx, value)
				if err != nil {
					return ctx, fmt.Errorf("config: '%s' refresh error: callback error: %s", key, err.Error())
				}
				this.SetValue(key, value)
			}
		}
	}
	return ctx, nil
}
func (this *Instance) LoadFromConfigPath(configName string) error {
	this.handler.SetConfigName(configName)
	err := this.handler.ReadInConfig()
	if err != nil {
		return fmt.Errorf(errLoadFromConfigPath+": %s", err.Error())
	}
	return nil
}
func (this *Instance) LoadFromConfigFile(configFile string) error {
	this.handler.SetConfigFile(configFile)
	err := this.handler.ReadInConfig()
	if err != nil {
		return fmt.Errorf(errLoadFromConfigFile+": %s", err.Error())
	}
	return nil
}
func (this *Instance) BindedEnvironmentKeys() []string{
	return this.bindedEnvironmentKeys
}
func (this *Instance) LoadFromEnv() error {
	for key, option := range this.options {
		if option.EnvironmentKey != "" {
			err := this.handler.BindEnv(option.EnvironmentKey)
			if err != nil {
				return fmt.Errorf(errLoadFromEnv+": [key:'%s', env_key:'%s', error:'%s']", key, option.EnvironmentKey, err.Error())
			}
			this.bindedEnvironmentKeys = append(this.bindedEnvironmentKeys, option.EnvironmentKey)
		}
	}
	return nil
}
func (this *Instance) LoadFromEnvFile(dotEnvFile string) error {
	if dotEnvFile != "" {
		if !strings.HasPrefix(dotEnvFile, "/") {
			dotEnvFile = strings.TrimRight(this.pathPrefix, "/") + "/" + dotEnvFile
		}
		_, err := os.Stat(dotEnvFile)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf(errLoadFromEnvFile+": %s", err.Error())
			}
		}
		err = godotenv.Load(dotEnvFile)
		if err != nil {
			return fmt.Errorf(errLoadFromEnvFile+": %s", err.Error())
		}
		err = this.LoadFromEnv()
		if err != nil {
			return fmt.Errorf(errLoadFromEnvFile+": %s", err.Error())
		}
	}
	return nil
}
func (this *Instance) WithContextValue(ctx context.Context, keymaps map[string]string) {
	if this.options != nil {
		for key, c := range this.options {
			contextKey := key
			if _, ok := keymaps[key]; ok {
				contextKey = keymaps[key]
			}
			switch c.Type {
			case String:
				ctx = context.WithValue(ctx, contextKey, this.GetString(key))
			case Bool:
				ctx = context.WithValue(ctx, contextKey, this.GetBool(key))
			case Int:
				ctx = context.WithValue(ctx, contextKey, this.GetInt(key))
			case Int64:
				ctx = context.WithValue(ctx, contextKey, this.GetInt64(key))
			case Float64:
				ctx = context.WithValue(ctx, contextKey, this.GetFloat64(key))
			case Time:
				ctx = context.WithValue(ctx, contextKey, this.GetTime(key))
			case Duration:
				ctx = context.WithValue(ctx, contextKey, this.GetDuration(key))
			case StringSlice:
				ctx = context.WithValue(ctx, contextKey, this.GetStringSlice(key))
			case StringMap:
				ctx = context.WithValue(ctx, contextKey, this.GetStringMap(key))
			case StringMapString:
				ctx = context.WithValue(ctx, contextKey, this.GetStringMapString(key))
			case StringMapStringSlice:
				ctx = context.WithValue(ctx, contextKey, this.GetStringMapStringSlice(key))
			case SizeInBytes:
				ctx = context.WithValue(ctx, contextKey, this.GetSizeInBytes(key))
			}
		}
	}
}
func (this *Instance) SetValue(key string, value interface{}) {
	this.handler.Set(key, value)
}
func (this *Instance) GetValue(key string) interface{} {
	return this.handler.Get(key)
}

// GetString returns the value associated with the key as a string.
func (this *Instance) GetString(key string) string {
	return this.handler.GetString(key)
}

// GetBool returns the value associated with the key as a boolean.
func (this *Instance) GetBool(key string) bool {
	return this.handler.GetBool(key)
}

// GetInt returns the value associated with the key as an integer.
func (this *Instance) GetInt(key string) int {
	return this.handler.GetInt(key)
}

// GetInt64 returns the value associated with the key as an integer.
func (this *Instance) GetInt64(key string) int64 {
	return this.handler.GetInt64(key)
}

// GetFloat64 returns the value associated with the key as a float64.
func (this *Instance) GetFloat64(key string) float64 {
	return this.handler.GetFloat64(key)
}

// GetTime returns the value associated with the key as time.
func (this *Instance) GetTime(key string) time.Time {
	return this.handler.GetTime(key)
}

// GetDuration returns the value associated with the key as a duration.
func (this *Instance) GetDuration(key string) time.Duration {
	return this.handler.GetDuration(key)
}

// GetStringSlice returns the value associated with the key as a slice of strings.
func (this *Instance) GetStringSlice(key string) []string {
	return this.handler.GetStringSlice(key)
}

// GetStringMap returns the value associated with the key as a map of interfaces.
func (this *Instance) GetStringMap(key string) map[string]interface{} {
	return this.handler.GetStringMap(key)
}

// GetStringMapString returns the value associated with the key as a map of strings.
func (this *Instance) GetStringMapString(key string) map[string]string {
	return this.handler.GetStringMapString(key)
}

// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
func (this *Instance) GetStringMapStringSlice(key string) map[string][]string {
	return this.handler.GetStringMapStringSlice(key)
}

// GetSizeInBytes returns the size of the value associated with the given key
// in bytes.
func (this *Instance) GetSizeInBytes(key string) uint {
	return this.handler.GetSizeInBytes(key)
}

type configHandler func() Handler

var configHandlers = make(map[string]configHandler)

func Register(name string, configHandler configHandler) error {
	if configHandler == nil {
		return errors.New("config: Register config is nil")
	}
	if _, ok := configHandlers[name]; !ok {
		configHandlers[name] = configHandler
	}
	return nil
}
func Use(name string) (configHandler, error) {
	if _, exist := configHandlers[name]; !exist {
		return nil, errors.New("config: unknown config " + name + " (forgotten register?)")
	}
	return configHandlers[name], nil
}

type Handler interface {
	Initiate(ctx context.Context) error
	AddConfigPath(in string)
	SetConfigName(in string)
	SetConfigFile(in string)
	ConfigFileUsed() string
	SetDefault(key string, value interface{})
	BindEnv(in string) error
	SetEnvPrefix(in string)
	ReadInConfig() error
	Set(key string, value interface{})
	Get(key string) interface{}
	// GetString returns the value associated with the key as a string.
	GetString(key string) string
	// GetBool returns the value associated with the key as a boolean.
	GetBool(key string) bool
	// GetInt returns the value associated with the key as an integer.
	GetInt(key string) int
	// GetInt64 returns the value associated with the key as an integer.
	GetInt64(key string) int64
	// GetFloat64 returns the value associated with the key as a float64.
	GetFloat64(key string) float64
	// GetTime returns the value associated with the key as time.
	GetTime(key string) time.Time
	// GetDuration returns the value associated with the key as a duration.
	GetDuration(key string) time.Duration
	// GetStringSlice returns the value associated with the key as a slice of strings.
	GetStringSlice(key string) []string
	// GetStringMap returns the value associated with the key as a map of interfaces.
	GetStringMap(key string) map[string]interface{}
	// GetStringMapString returns the value associated with the key as a map of strings.
	GetStringMapString(key string) map[string]string
	// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
	GetStringMapStringSlice(key string) map[string][]string
	// GetSizeInBytes returns the size of the value associated with the given key
	// in bytes.
	GetSizeInBytes(key string) uint
}
