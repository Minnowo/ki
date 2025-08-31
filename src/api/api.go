package api

import (
	"github.com/gorilla/mux"
)

type API interface {
	Register(r *mux.Router)
	Init()
	Deinit()
}

const (
	ROUTE__LOGIN  = "/login"
	ROUTE__LOGOUT = "/logout"
)
