// Package logging provides a structured JSON logger using slog.
package logging

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	keyRepo        contextKey = "repo"
	keyCorrelation contextKey = "correlation_id"
	keyK8sVersion  contextKey = "k8s_version"
)

// NewLogger creates a JSON-format slog.Logger at the given level string.
// Valid levels: "debug", "info", "warn", "error". Defaults to "info".
func NewLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: l})
	return slog.New(h)
}

// WithRepo returns a context carrying the repo name for structured logging.
func WithRepo(ctx context.Context, repo string) context.Context {
	return context.WithValue(ctx, keyRepo, repo)
}

// WithCorrelation returns a context carrying a correlation ID.
func WithCorrelation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyCorrelation, id)
}

// WithK8sVersion returns a context carrying the Kubernetes version.
func WithK8sVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, keyK8sVersion, version)
}

// FromContext extracts structured logging attributes from context.
func FromContext(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr
	if v, ok := ctx.Value(keyRepo).(string); ok && v != "" {
		attrs = append(attrs, slog.String("repo", v))
	}
	if v, ok := ctx.Value(keyCorrelation).(string); ok && v != "" {
		attrs = append(attrs, slog.String("correlation_id", v))
	}
	if v, ok := ctx.Value(keyK8sVersion).(string); ok && v != "" {
		attrs = append(attrs, slog.String("k8s_version", v))
	}
	return attrs
}
