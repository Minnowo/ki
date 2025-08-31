package api

import (
	"fmt"
	"io"
	"ki/src/logging"
	"net/http"
)

func Done(w http.ResponseWriter, code int, msg string) {
	logging.ApiLog.Debug().Int("code", code).Msg(msg)
	http.Error(w, msg, code)
}

func Donef(w http.ResponseWriter, code int, msg string, a ...any) {
	Done(w, code, fmt.Sprintf(msg, a...))
}

func NotFound(w http.ResponseWriter) {
	Done(w, http.StatusNotFound, "404 not found")
}

func Unauthorized(w http.ResponseWriter) {
	Done(w, http.StatusUnauthorized, "401 unauthorized")
}

func Redirect(w http.ResponseWriter, r *http.Request, route string) {
	http.Redirect(w, r, route, http.StatusSeeOther)
}

func Close(r io.ReadCloser, w http.ResponseWriter, code int, msg string) {
	Done(w, code, msg)
}

func Closef(r io.ReadCloser, w http.ResponseWriter, code int, msg string, a ...any) {
	Donef(w, code, msg, a...)
	r.Close()
}
