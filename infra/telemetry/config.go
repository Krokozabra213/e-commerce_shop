package telemetry

//type Config struct {
//	// Мастер-выключатель телеметрии.
//	// false = никакой отправки в OTel, только логи в stdout.
//	// Удобно для локального запуска без коллектора.
//	Enabled bool
//
//	// Что включить (работает только при Enabled=true)
//	Logs    bool
//	Metrics bool
//	Traces  bool
//
//	// Идентификация сервиса
//	ServiceName    string
//	ServiceVersion string
//	Environment    string
//
//	// Подключение к коллектору
//	OTELEndpoint string
//
//	// Семплирование трейсов (0.0 - 1.0)
//	// 1.0 = 100%, 0.1 = 10%, 0.0 = ничего
//	// Если не задано — используется 1.0 (всё)
//	TraceSampleRate float64
//
//	// Локальный режим:
//	// true  = логи пишутся И в stdout И в OTel
//	// false = логи пишутся ТОЛЬКО в OTel
//	LogToStdout bool
//
//	// Уровень логирования для stdout
//	// (OTel получает все уровни, фильтрация — на стороне коллектора)
//	StdoutLogLevel slog.Level
//
//	// Семплирование логов привязано к трейсам.
//	// true  = логи фильтруются по решению семплера трейсов
//	// false = все логи отправляются (как раньше)
//	SampleLogs bool
//
//	// Минимальный уровень для логов БЕЗ семплированного трейса.
//	// Логи с уровнем >= этого значения сохраняются ВСЕГДА.
//	// По умолчанию = slog.LevelError
//	UnsampledLogLevel slog.Level
//
//	Insecure bool
//}
