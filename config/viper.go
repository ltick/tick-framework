package config

import (
	"context"
	"time"

	"github.com/samt42/viper"
)

type ViperHandler struct {
	Viper *viper.Viper
}

func NewViperHandler() Handler {
	return &ViperHandler{
		Viper: viper.New(),
	}
}

func (this *ViperHandler) Initiate(ctx context.Context) error {
	return nil
}

func (this *ViperHandler) AddConfigPath(in string) {
	this.Viper.AddConfigPath(in)
}
func (this *ViperHandler) SetConfigName(in string) {
	this.Viper.SetConfigName(in)
}
func (this *ViperHandler) SetConfigFile(in string) {
	this.Viper.SetConfigFile(in)
}
func (this *ViperHandler) ConfigFileUsed() string {
	return this.Viper.ConfigFileUsed()
}
func (this *ViperHandler) SetDefault(key string, value interface{}) {
	this.Viper.SetDefault(key, value)
}
func (this *ViperHandler) SetEnvPrefix(in string) {
	this.Viper.SetEnvPrefix(in)
}
func (this *ViperHandler) BindEnv(in string) error {
	return this.Viper.BindEnv(in)
}
func (this *ViperHandler) ReadInConfig() error {
	return this.Viper.ReadInConfig()
}
func (this *ViperHandler) Set(key string, value interface{}) {
	this.Viper.Set(key, value)
}
func (this *ViperHandler) Get(key string) interface{} {
	return this.Viper.Get(key)
}

// GetString returns the value associated with the key as a string.
func (this *ViperHandler) GetString(key string) string {
	return this.Viper.GetString(key)
}

// GetBool returns the value associated with the key as a boolean.
func (this *ViperHandler) GetBool(key string) bool {
	return this.Viper.GetBool(key)
}

// GetInt returns the value associated with the key as an integer.
func (this *ViperHandler) GetInt(key string) int {
	return this.Viper.GetInt(key)
}

// GetInt64 returns the value associated with the key as an integer.
func (this *ViperHandler) GetInt64(key string) int64 {
	return this.Viper.GetInt64(key)
}

// GetFloat64 returns the value associated with the key as a float64.
func (this *ViperHandler) GetFloat64(key string) float64 {
	return this.Viper.GetFloat64(key)
}

// GetTime returns the value associated with the key as time.
func (this *ViperHandler) GetTime(key string) time.Time {
	return this.Viper.GetTime(key)
}

// GetDuration returns the value associated with the key as a duration.
func (this *ViperHandler) GetDuration(key string) time.Duration {
	return this.Viper.GetDuration(key)
}

// GetStringSlice returns the value associated with the key as a slice of strings.
func (this *ViperHandler) GetStringSlice(key string) []string {
	return this.Viper.GetStringSlice(key)
}

// GetStringMap returns the value associated with the key as a map of interfaces.
func (this *ViperHandler) GetStringMap(key string) map[string]interface{} {
	return this.Viper.GetStringMap(key)
}

// GetStringMapString returns the value associated with the key as a map of strings.
func (this *ViperHandler) GetStringMapString(key string) map[string]string {
	return this.Viper.GetStringMapString(key)
}

// GetStringMapStringSlice returns the value associated with the key as a map to a slice of strings.
func (this *ViperHandler) GetStringMapStringSlice(key string) map[string][]string {
	return this.Viper.GetStringMapStringSlice(key)
}

// GetSizeInBytes returns the size of the value associated with the given key
// in bytes.
func (this *ViperHandler) GetSizeInBytes(key string) uint {
	return this.Viper.GetSizeInBytes(key)
}
