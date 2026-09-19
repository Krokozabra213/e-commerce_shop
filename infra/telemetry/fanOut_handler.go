package telemetry

import (
	"context"
	"log/slog"
)

type fanOutHandler struct {
	handlers []slog.Handler
}

func (h *fanOutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

func (h *fanOutHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		// Проверяем Enabled для каждого хендлера отдельно
		// чтобы stdout мог фильтровать по уровню,
		// а OTel получал всё
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *fanOutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &fanOutHandler{handlers: handlers}
}

func (h *fanOutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &fanOutHandler{handlers: handlers}
}
