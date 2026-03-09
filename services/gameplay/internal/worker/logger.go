package worker

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Logger struct{}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Print(level zerolog.Level, args ...interface{}) {
	log.WithLevel(level).Msg(fmt.Sprint(args...))
}

func (l *Logger) Printf(_ context.Context, format string, v ...interface{}) {
	log.WithLevel(zerolog.DebugLevel).Msgf(format, v...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.Print(zerolog.DebugLevel, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.Print(zerolog.InfoLevel, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.Print(zerolog.WarnLevel, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.Print(zerolog.ErrorLevel, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.Print(zerolog.FatalLevel, args...)
}
