package service

import (
	"fmt"

	"go.n16f.net/boulevard/pkg/modules/httpserver"
)

type ControlAPIHandler struct {
	Ctx *httpserver.RequestContext
}

func (h *ControlAPIHandler) ReplyJSON(status int, value any) {
	h.Ctx.ReplyJSON(status, value)
}

func (h *ControlAPIHandler) ReplyError(status int, code string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)

	errData := ControlAPIError{
		Code:    code,
		Message: msg,
	}

	h.Ctx.ReplyJSON(status, &errData)
}

func (h *ControlAPIHandler) ReplyInternalError(status int, format string, args ...any) {
	h.Ctx.Log.Error(format, args...)
	h.ReplyError(status, "internal_error", format, args...)
}
