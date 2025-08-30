package pages

import (
	"ki/src/config"
	"ki/src/handlers/tracking"
	"net/http"

	"github.com/rs/zerolog/log"
)

type BaseView struct {
	IsTLS        bool
	Host         string
	Origin       string
	IsCloudflare bool
}

func NewBaseViewFromReq(r *http.Request) BaseView {

	var tls bool

	if r.URL.Scheme == "" {
		tls = r.TLS != nil
	} else {
		tls = r.URL.Scheme == "https"
	}

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

	log.Debug().
		Bool("cloudflare", tracking.IsBehindCloudflare(r)).
		Str("scheme", r.URL.Scheme).
		Bool("tls", tls).
		Str("host", host).
		Msg("base view")

	return BaseView{
		IsTLS:        tls,
		Host:         host,
		Origin:       origin,
		IsCloudflare: tracking.IsBehindCloudflare(r),
	}
}
