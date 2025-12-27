package logger

import (
	"github.com/rs/zerolog"
	temporallogger "go.temporal.io/sdk/log"
)

type Logger struct {
	logger zerolog.Logger
}

func New(logger zerolog.Logger) *Logger {
	return &Logger{
		logger: logger,
	}
}

func (l *Logger) Debug(msg string, keyVals ...interface{}) {
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

// WithFields returns a new Temporal logger with additional fields.
func WithFields(l temporallogger.Logger, fields map[string]interface{}) temporallogger.Logger {
	withLogger, ok := l.(temporallogger.WithLogger)
	if !ok {
		return l
	}

	keyVals := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		keyVals = append(keyVals, k, v)
	}

	return withLogger.With(keyVals...)
}
