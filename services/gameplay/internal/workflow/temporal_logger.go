package workflow

import "github.com/rs/zerolog"

type TemporalLogger struct {
	logger zerolog.Logger
}

func NewTemporalLogger(logger zerolog.Logger) *TemporalLogger {
	return &TemporalLogger{logger: logger}
}

func (l *TemporalLogger) Debug(msg string, keyVals ...interface{}) {
	l.logger.Debug().Fields(toFields(keyVals)).Msg(msg)
}

func (l *TemporalLogger) Info(msg string, keyVals ...interface{}) {
	l.logger.Info().Fields(toFields(keyVals)).Msg(msg)
}

func (l *TemporalLogger) Warn(msg string, keyVals ...interface{}) {
	l.logger.Warn().Fields(toFields(keyVals)).Msg(msg)
}

func (l *TemporalLogger) Error(msg string, keyVals ...interface{}) {
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
