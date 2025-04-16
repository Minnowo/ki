package main

import (
	v1 "ki/src/api/v1"
	"ki/src/assets"
	"ki/src/config"
	"ki/src/logging"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func main() {

	logging.InitFromEnv()

	r := mux.NewRouter()

	assets.Register(r)

	const addr = "0.0.0.0:3001"

	srv := &http.Server{
		Handler: r,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout:   15 * time.Second,
		ReadTimeout:    15 * time.Second,
		MaxHeaderBytes: 50 * config.KB,
	}

	apiv1 := v1.APIV1{}
	apiv1.Init()
	apiv1.Register(r)

	log.Info().Str("address", addr).Msg("Site is running")

	err := srv.ListenAndServe()

	apiv1.Deinit()

	log.Fatal().Err(err).Msg("Site is dead")
}
