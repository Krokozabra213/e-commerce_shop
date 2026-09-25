package inframiddleware

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorInterceptor struct {
	logger *slog.Logger
}

func NewErrorInterceptor(logger *slog.Logger) *ErrorInterceptor {
	return &ErrorInterceptor{
		logger: logger,
	}
}

func (i *ErrorInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		return nil, i.handleError(ctx, err, info.FullMethod)
	}
}

func (i *ErrorInterceptor) handleError(ctx context.Context, err error, method string) error {
	if _, ok := status.FromError(err); ok {
		return err
	}

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		i.logger.Error("internal server error",
			slog.Any("error", err),
			slog.String("method", method),
		)
		return status.Error(codes.Internal, "Internal server error")
	}

	slogLevel := mapAppLevelToSlog(appErr.LogLevel())
	grpcCode := mapAppCodeToGRPC(appErr.Code())

	attrs := []slog.Attr{
		slog.String("method", method),
		slog.String("code", appErr.Code().String()),
		slog.String("message", appErr.Message()),
	}

	if appErr.Op() != "" {
		attrs = append(attrs, slog.String("op", appErr.Op()))
	}

	if appErr.Err() != nil {
		attrs = append(attrs, slog.String("error", appErr.Err().Error()))
	}

	if appErr.Attrs() != nil {
		attrs = append(attrs, appErr.Attrs().ToAttrs()...)
	}

	i.logger.LogAttrs(ctx, slogLevel, appErr.Message(), attrs...)

	if grpcCode == codes.Internal || grpcCode == codes.Unknown {
		return status.Error(grpcCode, "Internal server error")
	}

	return status.Error(grpcCode, appErr.Message())
}

var codeMapping = map[apperror.Code]codes.Code{
	apperror.CodeNotFound:      codes.NotFound,
	apperror.CodeValidation:    codes.InvalidArgument,
	apperror.CodeAlreadyExists: codes.AlreadyExists,
	apperror.CodeConflict:      codes.AlreadyExists,
	apperror.CodeUnauthorized:  codes.Unauthenticated,
	apperror.CodeForbidden:     codes.PermissionDenied,
	apperror.CodeBadRequest:    codes.InvalidArgument,
	apperror.CodeInternal:      codes.Internal,
}

func mapAppCodeToGRPC(code apperror.Code) codes.Code {
	if grpcCode, ok := codeMapping[code]; ok {
		return grpcCode
	}
	return codes.Internal
}

func mapAppLevelToSlog(level apperror.Level) slog.Level {
	switch level {
	case apperror.LevelDebug:
		return slog.LevelDebug
	case apperror.LevelInfo:
		return slog.LevelInfo
	case apperror.LevelWarn:
		return slog.LevelWarn
	case apperror.LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
