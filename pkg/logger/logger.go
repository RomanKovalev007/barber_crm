package logger

import (
	"log/slog"
	"os"
)

func New(service string) *slog.Logger {
	var handler slog.Handler

	env := os.Getenv("APP_ENV")
	level := slog.LevelInfo
	
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler).With("service", service)
}