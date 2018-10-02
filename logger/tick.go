package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	libLogger "github.com/ltick/tick-log"
)

var (
	errGetLogger = "logger: get '%s' logger error"
	errGetLoggerTarget = "logger: get '%s' logger target error"
)
func NewTickHandler() Handler {
	return &TickHandler{}
}

type TickHandler struct {
	Loggers map[string]*libLogger.Logger
	Targets map[string]libLogger.Target
	loggerLocker  sync.RWMutex
	targetLocker  sync.RWMutex
}

func (this *TickHandler) Initiate(ctx context.Context) error {
	this.Loggers = make(map[string]*libLogger.Logger, 0)
	this.Targets = make(map[string]libLogger.Target, 0)
	this.loggerLocker = sync.RWMutex{}
	this.targetLocker = sync.RWMutex{}
	return nil
}

// SetLogger provides a given logger adapter into Logger with config string.
// config need to be correct JSON as string:
// File config sample:
// {
//     "MaxLevel": 2,
//     "Rotate": true,
//     "BackupCount": 100000,
//     "MaxBytes": 1024,
// }
func (this *TickHandler) NewLogger(name string) *libLogger.Logger {
	if this.Loggers != nil {
		l, err := this.GetLogger(name)
		if err == nil {
			return l
		}
		l = libLogger.NewLogger()
		this.loggerLocker.Lock()
		this.Loggers[name] = l
		this.loggerLocker.Unlock()
		return l
	}
	return nil
}

// GetLogger.
func (this *TickHandler) GetLogger(name string) (*libLogger.Logger, error) {
	this.loggerLocker.RLock()
	l, ok := this.Loggers[name]
	this.loggerLocker.RUnlock()
	if !ok {
		return nil, fmt.Errorf(errGetLogger + ": logger not exists", name)
	}
	return l, nil
}

func (this *TickHandler) GetLoggerTarget(name string) (libLogger.Target, error) {
	this.targetLocker.RLock()
	t, ok := this.Targets[name]
	this.targetLocker.RUnlock()
	if !ok {
		return nil, fmt.Errorf(errGetLoggerTarget, name)
	}
	return t, nil
}

func (this *TickHandler) RegisterLoggerTarget(name string, targetType string, targetConfig string) error {
	target, _ := this.GetLoggerTarget(name)
	if target == nil {
		switch targetType {
		case "file":
			fileLogTarget := libLogger.NewFileTarget()
			err := json.Unmarshal([]byte(targetConfig), fileLogTarget)
			if err != nil {
				return fmt.Errorf("application: invalid target config '%q' '%q'", targetConfig, err.Error())
			}
			this.targetLocker.Lock()
			this.Targets[name] = fileLogTarget
			this.targetLocker.Unlock()
		case "console":
			consoleLogTarget := libLogger.NewConsoleTarget()
			err := json.Unmarshal([]byte(targetConfig), consoleLogTarget)
			if err != nil {
				return fmt.Errorf("application: invalid target config '%q' '%q'", targetConfig, err.Error())
			}
			this.targetLocker.Lock()
			this.Targets[name] = consoleLogTarget
			this.targetLocker.Unlock()
		case "mail":
			mailLogTarget := libLogger.NewMailTarget()
			err := json.Unmarshal([]byte(targetConfig), mailLogTarget)
			if err != nil {
				return fmt.Errorf("application: invalid target config '%q' '%q'", targetConfig, err.Error())
			}
			this.targetLocker.Lock()
			this.Targets[name] = mailLogTarget
			this.targetLocker.Unlock()
		case "network":
			networkLogTarget := libLogger.NewNetworkTarget()
			err := json.Unmarshal([]byte(targetConfig), networkLogTarget)
			if err != nil {
				return fmt.Errorf("application: invalid target config '%q' '%q'", targetConfig, err.Error())
			}
			this.targetLocker.Lock()
			this.Targets[name] = networkLogTarget
			this.targetLocker.Unlock()
		default:
			return fmt.Errorf("application: invalid target %q", targetType)
		}
	}
	return nil
}

func (this *TickHandler) SetLoggerTarget(name string, targetName string) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	target, err := this.GetLoggerTarget(targetName)
	if err != nil {
		return err
	}
	this.loggerLocker.Lock()
	logger.Targets = append(logger.Targets, target)
	this.loggerLocker.Unlock()
	return nil
}

// SetLoggerMaxLevel.
func (this *TickHandler) SetLoggerMaxLevel(name string, level libLogger.Level) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	this.loggerLocker.Lock()
	logger.MaxLevel = level
	this.loggerLocker.Unlock()
	return nil
}

// SetLoggerCallStackDepth.
func (this *TickHandler) SetLoggerCallStackDepth(name string, d int) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	this.loggerLocker.Lock()
	logger.CallStackDepth = d
	this.loggerLocker.Unlock()
	return nil
}

// SetLoggerCallStackFilter.
func (this *TickHandler) SetLoggerCallStackFilter(name string, f string) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	this.loggerLocker.Lock()
	logger.CallStackFilter = f
	this.loggerLocker.Unlock()
	return nil
}

// SetLoggerSetFormatter.
func (this *TickHandler) SetLoggerFormatter(name string, f libLogger.Formatter) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	this.loggerLocker.Lock()
	logger.Formatter = f
	this.loggerLocker.Unlock()
	return nil
}

// SetLoggerCallStackFilter.
func (this *TickHandler) SetLoggerBufferSize(name string, b int) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	this.loggerLocker.Lock()
	logger.BufferSize = b
	this.loggerLocker.Unlock()
	return nil
}

// CloseLogger close all targets in Logger.
func (this *TickHandler) OpenLogger(name string) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	logger.Open()
	return nil
}

// CloseLogger close all targets in Logger.
func (this *TickHandler) CloseLogger(name string) error {
	logger, err := this.GetLogger(name)
	if err != nil {
		return err
	}
	logger.Close()
	return nil
}
