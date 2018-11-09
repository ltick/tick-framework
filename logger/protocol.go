package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	libLog "github.com/ltick/tick-log"
)

var (
	errInitiate       = "logger: initiate error"
	errStartup        = "logger: startup error"
	errInvalidLogType = "logger: invalid log type '%s'"
)

type LogConfig struct {
	Name      string
	Formatter string
	Type      string
	Filename  string
	Writer    string // the writer name of writer (stdout, stderr, discard)
	MaxLevel  string
}

type Log struct {
	Name      string
	Formatter Formatter
	Type      Type
	Filename  string
	Writer    Writer // the writer name of writer (stdout, stderr, discard)
	MaxLevel  Level
}

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
	FormatterRaw:     "Raw",
	FormatterSys:     "Sys",
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
	WriterUnknown: "unknown",
	WriterStdout:  "stdout",
	WriterStderr:  "stderr",
	WriterDiscard: "discard",
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
	Config   *config.Config `inject:"true"`
	Provider string
	Logs     []*LogConfig
	handler  Handler
}

func (l *Logger) Initiate(ctx context.Context) (context.Context, error) {
	err := Register("tick", NewTickHandler)
	if err != nil {
		return ctx, errors.Annotatef(err, errInitiate)
	}
	err = l.Use(ctx, "tick")
	if err != nil {
		return ctx, errors.Annotatef(err, errInitiate)
	}
	if l.Logs != nil {
		logs := make([]*Log, 0)
		for _, logConfig := range l.Logs {
			logConfigMaxLevel := LevelDebug
			for level, levelName := range LevelNames {
				if levelName == logConfig.MaxLevel {
					logConfigMaxLevel = level
					break
				}
			}
			switch StringToType(logConfig.Type) {
			case TypeFile:
				logs = append(logs, &Log{
					Name:      logConfig.Name,
					Type:      TypeFile,
					Formatter: StringToFormatter(logConfig.Formatter),
					Filename:  logConfig.Filename,
					MaxLevel:  logConfigMaxLevel,
				})
			case TypeConsole:
				logs = append(logs, &Log{
					Name:      logConfig.Name,
					Type:      TypeConsole,
					Formatter: StringToFormatter(logConfig.Formatter),
					Writer:    StringToWriter(logConfig.Writer),
					MaxLevel:  logConfigMaxLevel,
				})
			default:
				return ctx, errors.Errorf(errInvalidLogType, StringToType(logConfig.Type))
			}
		}
		var logProviders map[string]interface{} = make(map[string]interface{})
		logConfigProviderConfigs := make([]string, len(logs))
		for index, lg := range logs {
			switch lg.Type {
			case TypeFile:
				logFilename, err := filepath.Abs(lg.Filename)
				if err != nil {
					return ctx, err
				}
				_, err = os.Stat(logFilename)
				if err != nil {
					if os.IsNotExist(err) {
						_, err = os.Create(logFilename)
						if err != nil {
							return ctx, err
						}
					} else {
						return ctx, err
					}
				}
				logConfigProviderName := lg.Name + "FileTarget"
				logProviders[logConfigProviderName] = NewFileTarget
				logConfigProviderConfigs[index] = `"` + lg.Name + `":{"type": "` + logConfigProviderName + `","FileName":"` + logFilename + `","Rotate":true,"MaxBytes":` + strconv.Itoa(1<<22) + `}`
			case TypeConsole:
				logWriter := lg.Writer
				logConfigProviderName := lg.Name + "ConsoleTarget"
				logProviders[logConfigProviderName] = NewConsoleTarget
				logConfigProviderConfigs[index] = `"` + lg.Name + `":{"type": "` + logConfigProviderName + `","WriterName":"` + logWriter.String() + `"}`
				index++
			}
		}
		logConfig := `{`
		if len(logConfigProviderConfigs) > 0 {
			logConfig = logConfig + `"Targets": {` + strings.Join(logConfigProviderConfigs, ",") + `}`
		}
		logConfig = logConfig + `}`
		err := l.Config.ConfigureJsonConfig(l.handler, []byte(logConfig), logProviders)
		if err != nil {
			return ctx, errors.Annotate(err, errInitiate)
		}
		// logger
		for _, lg := range logs {
			l.NewLogger(lg.Name)
			switch lg.Formatter {
			case FormatterRaw:
				err = l.SetLoggerFormatter(lg.Name, RawLogFormatter())
				if err != nil {
					return ctx, errors.Annotate(err, errInitiate)
				}
			case FormatterSys:
				err = l.SetLoggerFormatter(lg.Name, SysLogFormatter())
				if err != nil {
					return ctx, errors.Annotate(err, errInitiate)
				}
			case FormatterDefault:
				err = l.SetLoggerFormatter(lg.Name, DefaultLogFormatter())
				if err != nil {
					return ctx, errors.Annotate(err, errInitiate)
				}
			}
			err = l.SetLoggerTarget(lg.Name, lg.Name)
			if err != nil {
				return ctx, errors.Annotate(err, errInitiate)
			}
			err = l.SetLoggerMaxLevel(lg.Name, lg.MaxLevel)
			if err != nil {
				return ctx, errors.Annotate(err, errInitiate)
			}
			err = l.OpenLogger(lg.Name)
			if err != nil {
				return ctx, errors.Annotate(err, errInitiate)
			}
		}
	}
	return ctx, nil
}
func (l *Logger) OnStartup(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (l *Logger) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (l *Logger) GetProvider() string {
	return l.Provider
}
func (l *Logger) Use(ctx context.Context, Provider string) error {
	handler, err := Use(Provider)
	if err != nil {
		return err
	}
	l.Provider = Provider
	l.handler = handler()
	err = l.handler.Initiate(ctx)
	if err != nil {
		return errors.Annotatef(err, errInitiate, l.Provider)
	}
	return nil
}

func (l *Logger) NewLogger(name string) *libLog.Logger {
	return l.handler.NewLogger(name)
}
func (l *Logger) GetLogger(name string) (*libLog.Logger, error) {
	return l.handler.GetLogger(name)
}
func (l *Logger) GetLoggerTarget(name string) (libLog.Target, error) {
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
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelEmergency)
	case LevelAlert:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelAlert)
	case LevelCritical:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelCritical)
	case LevelError:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelError)
	case LevelWarning:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelWarning)
	case LevelNotice:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelNotice)
	case LevelInfo:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelInfo)
	case LevelDebug:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelDebug)
	default:
		return l.handler.SetLoggerMaxLevel(name, libLog.LevelDebug)
	}
	return nil
}
func (l *Logger) SetLoggerCallStackDepth(name string, d int) error {
	return l.handler.SetLoggerCallStackDepth(name, d)
}
func (l *Logger) SetLoggerCallStackFilter(name string, f string) error {
	return l.handler.SetLoggerCallStackFilter(name, f)
}
func (l *Logger) SetLoggerFormatter(name string, f libLog.Formatter) error {
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

func DefaultLogFormatter() libLog.Formatter {
	return func(l *libLog.Logger, e *libLog.Entry) string {
		return fmt.Sprintf("%s|%s|%v%v", e.Time.Format(time.RFC3339), e.Level, e.Message, e.CallStack)
	}
}
func RawLogFormatter() libLog.Formatter {
	return func(l *libLog.Logger, e *libLog.Entry) string {
		return fmt.Sprintf("%v%v", e.Message, e.CallStack)
	}
}
func SysLogFormatter() libLog.Formatter {
	return func(l *libLog.Logger, e *libLog.Entry) string {
		return fmt.Sprintf(`%s %s`, e.Time.Format("2006/01/02 15:04:05"), e.Message)
	}
}
func NewConsoleTarget() *libLog.ConsoleTarget {
	return libLog.NewConsoleTarget()
}
func NewFileTarget() *libLog.FileTarget {
	return libLog.NewFileTarget()
}

type logHandler func() Handler

var logHandlers = make(map[string]logHandler)

func Register(name string, logHandler logHandler) error {
	if logHandler == nil {
		return errors.New("logger: Register log is nil")
	}
	if _, ok := logHandlers[name]; !ok {
		logHandlers[name] = logHandler
	}
	return nil
}
func Use(name string) (logHandler, error) {
	if _, exist := logHandlers[name]; !exist {
		return nil, errors.New(fmt.Sprintf("logger: unknown log '%s' (forgotten register?)", name))
	}
	return logHandlers[name], nil
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewLogger(name string) *libLog.Logger
	GetLogger(name string) (*libLog.Logger, error)
	GetLoggerTarget(name string) (libLog.Target, error)
	RegisterLoggerTarget(name string, targetType string, targetConfig string) error
	SetLoggerTarget(name string, targetName string) error
	SetLoggerMaxLevel(name string, level libLog.Level) error
	SetLoggerCallStackDepth(name string, d int) error
	SetLoggerCallStackFilter(name string, f string) error
	SetLoggerFormatter(name string, f libLog.Formatter) error
	SetLoggerBufferSize(name string, b int) error
	OpenLogger(name string) error
	CloseLogger(name string) error
}
