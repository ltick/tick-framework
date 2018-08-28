package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-ozzo/ozzo-config"
	libLogger "github.com/ltick/tick-log"
	"github.com/ltick/tick-routing"
)

var (
	errInitiate = "logger: initiate '%s' error"
)

// RFC5424 log message levels.
const (
	LevelEmergency Level = iota
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInfo
	LevelDebug
)

// Level describes the level of a log message.
type Level int

// LevelNames maps log levels to names
var LevelNames = map[Level]string{
	LevelDebug:     "Debug",
	LevelInfo:      "Info",
	LevelNotice:    "Notice",
	LevelWarning:   "Warning",
	LevelError:     "Error",
	LevelCritical:  "Critical",
	LevelAlert:     "Alert",
	LevelEmergency: "Emergency",
}

// String returns the string representation of the log level
func (l Level) String() string {
	if name, ok := LevelNames[l]; ok {
		return name
	}
	return "Unknown"
}
type Logger struct {
	*libLogger.Logger
}
func NewInstance() *Instance {
	instance := &Instance{}
	return instance
}

type Instance struct {
	handlerName string
	handler     Handler
}

func (this *Instance) Initiate(ctx context.Context) (context.Context, error) {
	err := Register("tick", NewTickHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	err = this.Use(ctx, "tick")
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
func (this *Instance) OnRequestStartup(c *routing.Context) error {
	return ctx, nil
}
func (this *Instance) OnRequestShutdown(c *routing.Context) error {
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

func (this *Instance) LoadModuleFileConfig(configFile string, configProviders map[string]interface{}, configTag ...string) (err error) {
	c := config.New()
	err = c.Load(configFile)
	if err != nil {
		return errors.New(fmt.Sprintf("logger: handler '%s' load config file '%s' error '%s'", this.handlerName, configFile, err.Error()))
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return errors.New(fmt.Sprintf("logger: handler '%s' register config provider '%s' error '%s'", this.handlerName, configProviderName, err.Error()))
			}
		}
	}
	err = c.Configure(this.handler, configTag...)
	if err != nil {
		return errors.New(fmt.Sprintf("logger: handler '%s' configure error '%s'", this.handlerName, err.Error()))
	}
	return nil
}

func (this *Instance) LoadModuleJsonConfig(configData []byte, configProviders map[string]interface{}, configTag ...string) (err error) {
	c := config.New()
	err = c.LoadJSON(configData)
	if err != nil {
		return errors.New(fmt.Sprintf("application: handler '%s' load config '%s' error '%s'", this.handlerName, configData, err.Error()))
	}
	if len(configProviders) > 0 {
		for configProviderName, configProvider := range configProviders {
			err = c.Register(configProviderName, configProvider)
			if err != nil {
				return errors.New(fmt.Sprintf("application: handler '%s' register config provider '%s' error '%s'", this.handlerName, configProviderName, err.Error()))
			}
		}
	}
	err = c.Configure(this.handler, configTag...)
	if err != nil {
		return errors.New(fmt.Sprintf("application: handler '%s' configure error '%s'", this.handlerName, err.Error()))
	}
	return nil
}

func (this *Instance) NewLogger(name string) *libLogger.Logger {
	return this.handler.NewLogger(name)
}
func (this *Instance) GetLogger(name string) (*libLogger.Logger, error) {
	return this.handler.GetLogger(name)
}
func (this *Instance) GetLoggerTarget(name string) (libLogger.Target, error) {
	return this.handler.GetLoggerTarget(name)
}
func (this *Instance) RegisterLoggerTarget(name string, targetType string, targetConfig string) error {
	return this.handler.RegisterLoggerTarget(name, targetType, targetConfig)
}
func (this *Instance) SetLoggerTarget(name string, targetName string) error {
	return this.handler.SetLoggerTarget(name, targetName)
}
func (this *Instance) SetLoggerMaxLevel(name string, level Level) error {
	switch level {
	case LevelEmergency:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelEmergency)
	case LevelAlert:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelAlert)
	case LevelCritical:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelCritical)
	case LevelError:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelError)
	case LevelWarning:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelWarning)
	case LevelNotice:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelNotice)
	case LevelInfo:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelInfo)
	case LevelDebug:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelDebug)
	default:
		return this.handler.SetLoggerMaxLevel(name, libLogger.LevelDebug)
	}
	return nil
}
func (this *Instance) SetLoggerCallStackDepth(name string, d int) error {
	return this.handler.SetLoggerCallStackDepth(name, d)
}
func (this *Instance) SetLoggerCallStackFilter(name string, f string) error {
	return this.handler.SetLoggerCallStackFilter(name, f)
}
func (this *Instance) SetLoggerFormatter(name string, f libLogger.Formatter) error {
	return this.handler.SetLoggerFormatter(name, f)
}
func (this *Instance) SetLoggerBufferSize(name string, b int) error {
	return this.handler.SetLoggerBufferSize(name, b)
}
func (this *Instance) OpenLogger(name string) error {
	return this.handler.OpenLogger(name)
}
func (this *Instance) CloseLogger(name string) error {
	return this.handler.CloseLogger(name)
}

func DefaultLogFormatter() libLogger.Formatter {
	return func(l *libLogger.Logger, e *libLogger.Entry) string {
		return fmt.Sprintf("%s|%s|%v%v", e.Time.Format(time.RFC3339), e.Level, e.Message, e.CallStack)
	}
}
func RawLogFormatter() libLogger.Formatter {
	return func(l *libLogger.Logger, e *libLogger.Entry) string {
		return fmt.Sprintf("%v%v", e.Message, e.CallStack)
	}
}
func SysLogFormatter() libLogger.Formatter {
	return func(l *libLogger.Logger, e *libLogger.Entry) string {
		return fmt.Sprintf(`%s %s`, e.Time.Format("2006/01/02 15:04:05"), e.Message)
	}
}
func NewConsoleTarget() *libLogger.ConsoleTarget {
	return libLogger.NewConsoleTarget()
}
func NewFileTarget() *libLogger.FileTarget {
	return libLogger.NewFileTarget()
}

type logHandler func() Handler

var logHandlers = make(map[string]logHandler)

func Register(name string, logHandler logHandler) error {
	if logHandler == nil {
		return errors.New("log: Register log is nil")
	}
	if _, ok := logHandlers[name]; !ok {
		logHandlers[name] = logHandler
	}
	return nil
}
func Use(name string) (logHandler, error) {
	if _, exist := logHandlers[name]; !exist {
		return nil, errors.New(fmt.Sprintf("log: unknown log '%s' (forgotten register?)", name))
	}
	return logHandlers[name], nil
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewLogger(name string) *libLogger.Logger
	GetLogger(name string) (*libLogger.Logger, error)
	GetLoggerTarget(name string) (libLogger.Target, error)
	RegisterLoggerTarget(name string, targetType string, targetConfig string) error
	SetLoggerTarget(name string, targetName string) error
	SetLoggerMaxLevel(name string, level libLogger.Level) error
	SetLoggerCallStackDepth(name string, d int) error
	SetLoggerCallStackFilter(name string, f string) error
	SetLoggerFormatter(name string, f libLogger.Formatter) error
	SetLoggerBufferSize(name string, b int) error
	OpenLogger(name string) error
	CloseLogger(name string) error
}
