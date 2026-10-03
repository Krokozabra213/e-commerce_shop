package grpcclient

import (
	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var grpcToAppCode = map[codes.Code]apperror.Code{
	codes.NotFound:         apperror.CodeNotFound,
	codes.InvalidArgument:  apperror.CodeValidation,
	codes.AlreadyExists:    apperror.CodeAlreadyExists,
	codes.Unauthenticated:  apperror.CodeUnauthorized,
	codes.PermissionDenied: apperror.CodeForbidden,
	codes.Internal:         apperror.CodeInternal,
	codes.Unknown:          apperror.CodeInternal,
}

func ParseGRPCError(serviceName string, err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return apperror.NewInternal(serviceName, err, "Ошибка соединения с "+serviceName, nil)
	}

	code, ok := grpcToAppCode[st.Code()]
	if !ok {
		code = apperror.CodeInternal
	}

	msg := st.Message()
	if msg == "" {
		msg = st.Code().String()
	}

	if code == apperror.CodeInternal {
		return apperror.NewInternal(serviceName, err, "Ошибка downstream сервиса", nil)
	}

	return apperror.NewBusiness(code, msg)
}
