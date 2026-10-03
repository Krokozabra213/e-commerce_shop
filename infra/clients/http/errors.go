package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
)

type downstreamErrorResponse struct {
	Error string `json:"error"`
}

func mapHTTPStatusToAppCode(status int) apperror.Code {
	switch status {
	case http.StatusBadRequest:
		return apperror.CodeBadRequest
	case http.StatusUnauthorized:
		return apperror.CodeUnauthorized
	case http.StatusForbidden:
		return apperror.CodeForbidden
	case http.StatusNotFound:
		return apperror.CodeNotFound
	case http.StatusConflict:
		return apperror.CodeAlreadyExists
	default:
		return apperror.CodeInternal
	}
}

func ParseDownstreamError(serviceName string, resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	code := mapHTTPStatusToAppCode(resp.StatusCode)

	var errResp downstreamErrorResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return apperror.NewBusiness(code, errResp.Error)
	}

	msg := string(body)
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}

	if code == apperror.CodeInternal {
		return apperror.NewInternal(
			fmt.Sprintf("%s.%s", serviceName, http.StatusText(resp.StatusCode)),
			fmt.Errorf("status %d: %s", resp.StatusCode, msg),
			"Ошибка downstream сервиса",
			nil,
		)
	}

	return apperror.NewBusiness(code, msg)
}
