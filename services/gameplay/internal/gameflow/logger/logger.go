package logger

import "github.com/rs/zerolog"

type Logger struct {
	logger zerolog.Logger
}

func NewGlobalLogger(logger zerolog.Logger) *Logger {
	return &Logger{
		logger: logger,
	}
}

func NewWorkflowLogger(logger zerolog.Logger, static map[string]interface{}) *Logger {
	return &Logger{
		logger: logger.With().Fields(static).Logger(),
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
