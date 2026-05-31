package logger

import (
	"github.com/rs/zerolog"
	temporallogger "go.temporal.io/sdk/log"
)

type Logger struct {
	logger   zerolog.Logger
	isDevEnv bool
}

func New(logger zerolog.Logger, isDevEnv bool) *Logger {
	return &Logger{
		logger:   logger,
		isDevEnv: isDevEnv,
	}
}

func (l *Logger) Debug(msg string, keyVals ...interface{}) {
	if !l.isDevEnv {
		return
	}
	l.logger.Debug().Fields(toFields(keyVals)).Msg(msg)
}

func (l *Logger) Info(msg string, keyVals ...interface{}) {
	l.logger.Info().Fields(toFields(keyVals)).Msg(msg)
}

func (l *Logger) Warn(msg string, keyVals ...interface{}) {
	l.logger.Warn().Fields(toFields(keyVals)).Msg(msg)
}

func (l *Logger) Error(msg string, keyVals ...interface{}) {
	l.logger.Error().Fields(toFields(keyVals)).Msg(msg)
}

func (l *Logger) With(keyVals ...interface{}) temporallogger.Logger {
	return &Logger{
		logger: l.logger.With().Fields(toFields(keyVals)).Logger(),
	}
}

func (l *Logger) WithCallerSkip(_ int) temporallogger.Logger {
	return l
}

func toFields(keyVals []interface{}) map[string]interface{} {
	fields := map[string]interface{}{}
	for i := 0; i < len(keyVals)-1; i += 2 {
		key, ok := keyVals[i].(string)
		if !ok {
			continue
		}
		fields[key] = keyVals[i+1]
	}
	return fields
}
