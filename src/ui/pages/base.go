package pages

import (
	"ki/src/config"
	"net/http"
)

type BaseView struct {
	IsTLS  bool
	Host   string
	Origin string
}

func NewBaseViewFromReq(r *http.Request) BaseView {

	tls := r.TLS != nil
	host := r.Host

	if host == "" {
		host = config.HOST
	}

	origin := host

	if tls {
		origin = "https://" + host
	} else {
		origin = "http://" + host
	}

	return BaseView{
		IsTLS:  tls,
		Host:   host,
		Origin: origin,
	}
}
