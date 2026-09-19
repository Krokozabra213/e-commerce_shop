package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// sampledLogHandler — фильтрует логи по признаку семплирования трейса.
//
// Правила:
//   - Есть трейс в ctx И трейс семплирован   → пропускаем ВСЕ уровни
//   - Есть трейс в ctx И трейс НЕ семплирован → пропускаем только >= minLevel
//   - Нет трейса (воркер, cron, init)          → пропускаем только >= minLevel
//
// minLevel обычно = slog.LevelError, но можно настроить.
type sampledLogHandler struct {
	// inner — реальный handler, который отправляет логи
	// (otelslog, JSON handler и т.д.)
	inner slog.Handler

	// minLevel — минимальный уровень для логов БЕЗ семплированного трейса.
	// Логи ниже этого уровня отбрасываются, если трейс не семплирован
	// или трейса вообще нет.
	minLevel slog.Level
}

// newSampledLogHandler оборачивает любой slog.Handler
// и добавляет фильтрацию по семплированию трейса.
func newSampledLogHandler(inner slog.Handler, minLevel slog.Level) *sampledLogHandler {
	return &sampledLogHandler{
		inner:    inner,
		minLevel: minLevel,
	}
}

func (h *sampledLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Быстрая проверка: если inner вообще не поддерживает этот уровень —
	// сразу отказываем. Это важно для производительности.
	if !h.inner.Enabled(ctx, level) {
		return false
	}

	// Ошибки всегда пропускаем (если inner handler их поддерживает)
	if level >= h.minLevel {
		return true
	}

	// Для уровней ниже Error — проверяем семплирование трейса.
	// Если трейс семплирован — пропускаем.
	// Если нет трейса или трейс не семплирован — отбрасываем.
	return isTraceSampled(ctx)
}

func (h *sampledLogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Двойная проверка на случай прямого вызова Handle без Enabled
	// record не клонируется, т.к. мы не модифицируем его
	// и передаём только в один inner handler
	if record.Level >= h.minLevel {
		return h.inner.Handle(ctx, record)
	}

	if isTraceSampled(ctx) {
		return h.inner.Handle(ctx, record)
	}

	// Отбрасываем
	return nil
}

func (h *sampledLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &sampledLogHandler{
		inner:    h.inner.WithAttrs(attrs),
		minLevel: h.minLevel,
	}
}

func (h *sampledLogHandler) WithGroup(name string) slog.Handler {
	return &sampledLogHandler{
		inner:    h.inner.WithGroup(name),
		minLevel: h.minLevel,
	}
}

// isTraceSampled проверяет, семплирован ли трейс в контексте.
func isTraceSampled(ctx context.Context) bool {
	sc := trace.SpanContextFromContext(ctx)

	// IsValid — есть traceID и spanID
	// IsSampled — решение самплера: этот трейс записывается
	return sc.IsValid() && sc.IsSampled()
}
