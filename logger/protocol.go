package logger

import (
	"context"
	"errors"
	"fmt"
	"time"
	"strings"

	libLogger "github.com/ltick/tick-log"
)

var (
	errInitiate = "logger: initiate '%s' error"
)


// Formatter describes the formatter of a log message.
type Formatter int

const (
	FormatterDefault Formatter = iota
	FormatterRaw
	FormatterSys
)

// LevelNames maps log levels to names
var FormatterNames = map[Formatter]string{
	FormatterDefault: "Default",
	FormatterRaw:  "Raw",
	FormatterSys:  "Sys",
}

// String returns the string representation of the log level
func (w Formatter) String() string {
	if name, ok := FormatterNames[w]; ok {
		return name
	}
	return FormatterNames[FormatterDefault]
}

func StringToFormatter(name string) Formatter {
	for formatter, formatterName := range FormatterNames {
		if strings.ToLower(name) == strings.ToLower(formatterName) {
			return formatter
		}
	}
	return FormatterDefault
}

// Writer describes the writer of a log message.
type Writer int

const (
	WriterUnknown Writer = iota
	WriterStdout
	WriterStderr
	WriterDiscard
)

// LevelNames maps log levels to names
var WriterNames = map[Writer]string{
	WriterUnknown: "Unknown",
	WriterStdout:  "Stdout",
	WriterStderr:  "Stderr",
	WriterDiscard: "Discard",
}

// String returns the string representation of the log level
func (w Writer) String() string {
	if name, ok := WriterNames[w]; ok {
		return name
	}
	return WriterNames[WriterUnknown]
}

func StringToWriter(name string) Writer {
	for writer, writerName := range WriterNames {
		if strings.ToLower(name) == strings.ToLower(writerName) {
			return writer
		}
	}
	return WriterUnknown
}

// Type describes the type of a log message.
type Type int

const (
	TypeUnknown Type = iota
	TypeFile
	TypeConsole
)

// LevelNames maps log levels to names
var TypeNames = map[Type]string{
	TypeUnknown: "Unknown",
	TypeFile:    "File",
	TypeConsole: "Console",
}

// String returns the string representation of the log level
func (t Type) String() string {
	if name, ok := TypeNames[t]; ok {
		return name
	}
	return TypeNames[TypeUnknown]
}

func StringToType(name string) Type {
	for typ, typeName := range TypeNames {
		if strings.ToLower(name) == strings.ToLower(typeName) {
			return typ
		}
	}
	return TypeUnknown
}

// Level describes the level of a log message.
type Level int

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

func NewLogger() *Logger {
	logger := &Logger{}
	return logger
}

type Logger struct {
	handlerName string
	handler     Handler
}

func (l *Logger) Initiate(ctx context.Context) (context.Context, error) {
	err := Register("tick", NewTickHandler)
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), l.handlerName))
	}
	err = l.Use(ctx, "tick")
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), l.handlerName))
	}
	return ctx, nil
}
func (l *Logger) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (l *Logger) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (l *Logger) HandlerName() string {
	return l.handlerName
}
func (l *Logger) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	l.handlerName = handlerName
	l.handler = handler()
	err = l.handler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), l.handlerName))
	}
	return nil
}

func (l *Logger) NewLogger(name string) *libLogger.Logger {
	return l.handler.NewLogger(name)
}
func (l *Logger) GetLogger(name string) (*libLogger.Logger, error) {
	return l.handler.GetLogger(name)
}
func (l *Logger) GetLoggerTarget(name string) (libLogger.Target, error) {
	return l.handler.GetLoggerTarget(name)
}
func (l *Logger) RegisterLoggerTarget(name string, targetType string, targetConfig string) error {
	return l.handler.RegisterLoggerTarget(name, targetType, targetConfig)
}
func (l *Logger) SetLoggerTarget(name string, targetName string) error {
	return l.handler.SetLoggerTarget(name, targetName)
}
func (l *Logger) SetLoggerMaxLevel(name string, level Level) error {
	switch level {
	case LevelEmergency:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelEmergency)
	case LevelAlert:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelAlert)
	case LevelCritical:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelCritical)
	case LevelError:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelError)
	case LevelWarning:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelWarning)
	case LevelNotice:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelNotice)
	case LevelInfo:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelInfo)
	case LevelDebug:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelDebug)
	default:
		return l.handler.SetLoggerMaxLevel(name, libLogger.LevelDebug)
	}
	return nil
}
func (l *Logger) SetLoggerCallStackDepth(name string, d int) error {
	return l.handler.SetLoggerCallStackDepth(name, d)
}
func (l *Logger) SetLoggerCallStackFilter(name string, f string) error {
	return l.handler.SetLoggerCallStackFilter(name, f)
}
func (l *Logger) SetLoggerFormatter(name string, f libLogger.Formatter) error {
	return l.handler.SetLoggerFormatter(name, f)
}
func (l *Logger) SetLoggerBufferSize(name string, b int) error {
	return l.handler.SetLoggerBufferSize(name, b)
}
func (l *Logger) OpenLogger(name string) error {
	return l.handler.OpenLogger(name)
}
func (l *Logger) CloseLogger(name string) error {
	return l.handler.CloseLogger(name)
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
