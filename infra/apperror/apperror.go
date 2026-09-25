package apperror

import (
	"errors"
	"fmt"
	"log/slog"
)

type Code string

const (
	CodeNotFound      Code = "NOT_FOUND"
	CodeValidation    Code = "VALIDATION_ERROR"
	CodeAlreadyExists Code = "ALREADY_EXISTS"
	CodeInternal      Code = "INTERNAL_ERROR"
	CodeUnauthorized  Code = "UNAUTHORIZED"
	CodeForbidden     Code = "FORBIDDEN"
	CodeBadRequest    Code = "BAD_REQUEST"
	CodeConflict      Code = "CONFLICT"
)

func (c Code) String() string {
	return string(c)
}

type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
)

type AppError struct {
	code     Code
	message  string
	op       string
	err      error
	attrs    Fields
	logLevel Level
}

func NewInternal(op string, err error, message string, attrs Fields) *AppError {
	return &AppError{
		code:     CodeInternal,
		message:  message,
		op:       op,
		err:      err,
		logLevel: LevelError,
		attrs:    attrs,
	}
}

func (e *AppError) WithAttr(key string, val any) *AppError {
	if e.attrs == nil {
		e.attrs = make(Fields, 1)
	}
	e.attrs = append(e.attrs, Field{key, val})
	return e
}

func NewBusiness(code Code, message string) *AppError {
	return &AppError{
		code:     code,
		message:  message,
		logLevel: LevelDebug,
	}
}

func (e *AppError) Err() error {
	return e.err
}

func (e *AppError) Code() Code {
	return e.code
}

// Message возвращает сообщение об ошибке
func (e *AppError) Message() string {
	return e.message
}

// Op возвращает операцию, на которой возникла ошибка
func (e *AppError) Op() string {
	return e.op
}

// Attrs возвращает структурированные атрибуты для логирования
func (e *AppError) Attrs() Fields {
	return e.attrs
}

// LogLevel возвращает уровень логирования
func (e *AppError) LogLevel() Level {
	return e.logLevel
}

func (e *AppError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.op, e.message, e.err)
	}
	return fmt.Sprintf("[%s] %s", e.op, e.message)
}

func (e *AppError) Unwrap() error {
	return e.err
}

func NewAppErr(code Code, op, message string, err error, level Level, fields Fields) *AppError {
	return &AppError{
		code:     code,
		op:       op,
		message:  message,
		err:      err,
		attrs:    fields,
		logLevel: level,
	}
}

func (e *AppError) AddFields(fields ...Field) *AppError {
	if e == nil {
		return nil
	}
	newErr := *e
	newErr.attrs = make(Fields, len(e.attrs), len(e.attrs)+len(fields))
	copy(newErr.attrs, e.attrs)
	newErr.attrs = append(newErr.attrs, fields...)
	return &newErr
}

func GetAppErr(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

type Field struct {
	Key   string
	Value any
}

type Fields []Field

func (f Fields) ToSlogArgs() []any {
	args := make([]any, 0, len(f)*2)
	for _, f := range f {
		args = append(args, f.Key, f.Value)
	}
	return args
}

func (f Fields) ToAttrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, len(f))
	for _, field := range f {
		attrs = append(attrs, slog.Any(field.Key, field.Value))
	}
	return attrs
}

func (f Fields) Add(fields ...Field) Fields {
	newFields := make(Fields, 0, len(f)+len(fields))
	newFields = append(newFields, f...)
	newFields = append(newFields, fields...)
	return newFields
}
