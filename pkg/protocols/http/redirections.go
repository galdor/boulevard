package http

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go.n16f.net/boulevard/pkg/boulevard"
)

func redirect(status int, uriString string, header HeaderOps, body *boulevard.FormatString, view *View, ctx *RequestContext) {
	if status == 0 {
		status = 302
	}

	uri, err := url.Parse(uriString)
	if err != nil {
		ctx.Log.Error("cannot parse redirection URI %q: %v", uriString, err)
		ctx.ReplyError(500)
		return
	}

	ctx.Vars["http.redirect.uri"] = uri.String()

	responseHeader := ctx.ResponseWriter.Header()
	responseHeader.Set("Location", uri.String())
	header.Apply(responseHeader, ctx.Vars)

	var bodyReader io.Reader
	if ctx.Request.Method == "GET" {
		if body == nil {
			responseHeader.Set("Content-Type", MediaTypeHTML.String())

			tplData := struct {
				Status int    `json:"status"`
				Reason string `json:"reason"`
				URI    string `json:"uri"`
			}{
				Status: status,
				Reason: http.StatusText(status),
				URI:    uri.String(),
			}

			bodyData, err := view.Render("redirect", tplData, ctx)
			if err != nil {
				ctx.Log.Error("cannot render redirection data: %v", err)
				ctx.ReplyError(500)
				return
			}

			bodyReader = bytes.NewReader(bodyData)
		} else {
			bodyString := body.Expand(ctx.Vars)
			bodyReader = strings.NewReader(bodyString)
		}
	}

	ctx.Reply(status, bodyReader)
}
