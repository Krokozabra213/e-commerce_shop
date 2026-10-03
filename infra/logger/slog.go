package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/Krokozabra213/e-commerce_shop/infra/config"
)

func Init(opts *infracfg.SlogConfig, handler ...slog.Handler) *slog.Logger {
	var h slog.Handler
	if len(handler) > 0 && handler[0] != nil {
		h = handler[0]
	} else {
		h = defaultHandler(opts)
	}

	log := slog.New(h)
	slog.SetDefault(log)
	return log
}

func ErrAttr(err error) slog.Attr {
	return slog.Any("error", err)
}

func OpAttr(op string) slog.Attr {
	return slog.String("op.name", op)
}

func defaultHandler(cfg *infracfg.SlogConfig) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     parseLevel(cfg.Level),
		AddSource: cfg.AddSource,
	}

	switch cfg.Format {
	case "text":
		return slog.NewTextHandler(os.Stdout, opts)
	default:
		return slog.NewJSONHandler(os.Stdout, opts)
	}
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type ctxKey struct{}
type requestIDKey struct{}

func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.Default()
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func FromContextOK(ctx context.Context) (*slog.Logger, bool) {
	l, ok := ctx.Value(ctxKey{}).(*slog.Logger)
	return l, ok && l != nil
}
