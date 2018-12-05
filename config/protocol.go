package config

import (
	"context"
	"os"
	"time"

	"github.com/go-ozzo/ozzo-config"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"github.com/juju/errors"
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

func NewConfig() *Config {
	instance := &Config{}
	return instance
}

type Config struct {
	Provider string
	handler  Handler

	options               map[string]Option
	bindedEnvironmentKeys []string
}

func (c *Config) Prepare(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (c *Config) Initiate(ctx context.Context) (context.Context, error) {
	if c.options == nil {
		c.options = make(map[string]Option)
	}
	err := Register("viper", NewViperHandler)
	if err != nil {
		return ctx, errors.Annotatef(err, errInitiate, c.Provider)
	}
	err = c.Use(ctx, "viper")
	if err != nil {
		return ctx, errors.Annotatef(err, errInitiate, c.Provider)
	}
	return ctx, nil
}
func (c *Config) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (c *Config) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (c *Config) GetProvider() string {
	return c.Provider
}
func (c *Config) Use(ctx context.Context, Provider string) error {
	handler, err := Use(Provider)
	if err != nil {
		return err
	}
	c.Provider = Provider
	c.handler = handler()
	err = c.handler.Initiate(ctx)
	if err != nil {
		return errors.Annotatef(err, errInitiate, c.Provider)
	}
	return nil
}
func (c *Config) AddConfigPath(in string) {
	c.handler.AddConfigPath(in)
}
func (c *Config) ConfigFileUsed() string {
	return c.handler.ConfigFileUsed()
}
func (c *Config) SetEnvPrefix(in string) {
	c.handler.SetEnvPrefix(in)
}
func (c *Config) SetOptions(options map[string]Option) error {
	if options != nil {
		keys := make([]string, 0)
		for key, option := range options {
			if key != "" {
				if _, ok := c.options[key]; !ok {
					c.options[key] = option
				}
				keys = append(keys, key)
			}
			if option.Default != nil {
				c.handler.SetDefault(key, option.Default)
			}
		}
	}
	return nil
}
func (c *Config) Callbacks(ctx context.Context, callbacks map[string]Callback) (context.Context, error) {
	if callbacks != nil {
		for key, callback := range callbacks {
			value := c.Get(key)
			if value != nil {
				value, err := callback(ctx, value)
				if err != nil {
					return ctx, errors.Annotatef(err, "config: callback '%s' error:", key)
				}
				c.Set(key, value)
			}
		}
	}
	return ctx, nil
}

func (c *Config) ConfigureFileConfig(target interface{}, configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	oc := config.New()
	err = oc.Load(configFile)
	if err != nil {
		return errors.Annotatef(err, "config: load config file '%s'", configFile)
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = oc.Register(configProviderName, configProvider)
			if err != nil {
				return errors.Annotatef(err, "config: register config provider '%s' error", configProviderName)
			}
		}
	}
	err = oc.Configure(target, configTag...)
	if err != nil {
		return errors.Annotatef(err, "config: configure '%v' error", target)
	}
	return nil
}

func (c *Config) ConfigureJsonConfig(target interface{}, configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
	oc := config.New()
	err = oc.LoadJSON(configData)
	if err != nil {
		return errors.Annotatef(err, "config: load config '%s' error", configData)
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = oc.Register(configProviderName, configProvider)
			if err != nil {
				return errors.Annotatef(err, "config: register config provider '%s' error", configProviderName)
			}
		}
	}
	err = oc.Configure(target, configTag...)
	if err != nil {
		return errors.Annotatef(err, "config: configure '%s' error", target)
	}
	return nil
}

func (c *Config) LoadFromConfigPath(configName string) error {
	c.handler.SetConfigName(configName)
	err := c.handler.ReadInConfig()
	if err != nil {
		return errors.Annotatef(err, errLoadFromConfigPath)
	}
	return nil
}
func (c *Config) LoadFromConfigFile(configFile string) error {
	c.handler.SetConfigFile(configFile)
	err := c.handler.ReadInConfig()
	if err != nil {
		return errors.Annotatef(err, errLoadFromConfigFile)
	}
	return nil
}
func (c *Config) BindedEnvironmentKeys() []string {
	return c.bindedEnvironmentKeys
}
func (c *Config) LoadFromEnv() error {
	for key, option := range c.options {
		if option.EnvironmentKey != "" {
			err := c.handler.BindEnv(option.EnvironmentKey)
			if err != nil {
				return errors.Annotatef(err, errLoadFromEnv+": [key:'%s', env_key:'%s']", key, option.EnvironmentKey)
			}
			c.bindedEnvironmentKeys = append(c.bindedEnvironmentKeys, option.EnvironmentKey)
		}
	}
	return nil
}
func (c *Config) LoadFromEnvFile(dotEnvFile string) error {
	if dotEnvFile != "" {
		_, err := os.Stat(dotEnvFile)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.Annotatef(err, errLoadFromEnvFile)
			}
		}
		err = godotenv.Load(dotEnvFile)
		if err != nil {
			return errors.Annotatef(err, errLoadFromEnvFile)
		}
		err = c.LoadFromEnv()
		if err != nil {
			return errors.Annotatef(err, errLoadFromEnvFile)
		}
	}
	return nil
}
func (c *Config) WithContextValue(ctx context.Context, keymaps map[string]string) {
	if c.options != nil {
		for key, option := range c.options {
			contextKey := key
			if _, ok := keymaps[key]; ok {
				contextKey = keymaps[key]
			}
			switch option.Type {
			case String:
				ctx = context.WithValue(ctx, contextKey, c.GetString(key))
			case Bool:
				ctx = context.WithValue(ctx, contextKey, c.GetBool(key))
			case Int:
				ctx = context.WithValue(ctx, contextKey, c.GetInt(key))
			case Int64:
				ctx = context.WithValue(ctx, contextKey, c.GetInt64(key))
			case Float64:
				ctx = context.WithValue(ctx, contextKey, c.GetFloat64(key))
			case Time:
				ctx = context.WithValue(ctx, contextKey, c.GetTime(key))
			case Duration:
				ctx = context.WithValue(ctx, contextKey, c.GetDuration(key))
			case StringSlice:
				ctx = context.WithValue(ctx, contextKey, c.GetStringSlice(key))
			case StringMap:
				ctx = context.WithValue(ctx, contextKey, c.GetStringMap(key))
			case StringMapString:
				ctx = context.WithValue(ctx, contextKey, c.GetStringMapString(key))
			case StringMapStringSlice:
				ctx = context.WithValue(ctx, contextKey, c.GetStringMapStringSlice(key))
			case SizeInBytes:
				ctx = context.WithValue(ctx, contextKey, c.GetSizeInBytes(key))
			}
		}
	}
}
func (c *Config) Set(key string, value interface{}) {
	c.handler.Set(key, value)
}

// GetString returns the value associated with the key as a string.
func (c *Config) Get(key string) interface{} {
	return c.handler.Get(key)
}

// GetString returns the value associated with the key as a string.
func (c *Config) GetString(key string) string {
	return c.handler.GetString(key)
}

// GetBool returns the value associated with the key as a boolean.
func (c *Config) GetBool(key string) bool {
	return c.handler.GetBool(key)
}

// GetInt returns the value associated with the key as an integer.
func (c *Config) GetInt(key string) int {
	return c.handler.GetInt(key)
}

// GetInt64 returns the value associated with the key as an integer.
func (c *Config) GetInt64(key string) int64 {
	return c.handler.GetInt64(key)
}

// GetFloat64 returns the value associated with the key as a float64.
func (c *Config) GetFloat64(key string) float64 {
	return c.handler.GetFloat64(key)
}

// GetTime returns the value associated with the key as time.
func (c *Config) GetTime(key string) time.Time {
	return c.handler.GetTime(key)
}

// GetDuration returns the value associated with the key as a duration.
func (c *Config) GetDuration(key string) time.Duration {
	return c.handler.GetDuration(key)
}

// GetStringSlice returns the value associated with the key as a slice of strings.
func (c *Config) GetStringSlice(key string) []string {
	return c.handler.GetStringSlice(key)
}

// GetStringMap returns the value associated with the key as a map of interfaces.
func (c *Config) GetStringMap(key string) map[string]interface{} {
	return c.handler.GetStringMap(key)
}

// GetStringMapString returns the value associated with the key as a map of strings.
func (c *Config) GetStringMapString(key string) map[string]string {
	return c.handler.GetStringMapString(key)
}

// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
func (c *Config) GetStringMapStringSlice(key string) map[string][]string {
	return c.handler.GetStringMapStringSlice(key)
}

// GetSizeInBytes returns the size of the value associated with the given key
// in bytes.
func (c *Config) GetSizeInBytes(key string) uint {
	return c.handler.GetSizeInBytes(key)
}

func (c *Config) GetAll() map[string]interface{} {
	return c.handler.AllSettings()
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
	AllSettings() map[string]interface{}
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
