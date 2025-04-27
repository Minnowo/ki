package cmd

import (
	"context"
	"fmt"
	"ki/src/api/middleware"
	v1 "ki/src/api/v1"
	"ki/src/assets"
	"ki/src/config"
	"ki/src/handlers/user"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func CmdServerMain(ctx context.Context, c *cli.Command) error {

	registryPath := c.Value("registry").(string)
	bindAddr := c.Value("bind").(string)
	port := c.Value("port").(int32)

	var addr = fmt.Sprintf("%s:%d", bindAddr, port)

	r := mux.NewRouter()
	r.Use(middleware.Recoverer)

	assets.Register(r)

	srv := &http.Server{
		Handler: r,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout:   15 * time.Second,
		ReadTimeout:    15 * time.Second,
		MaxHeaderBytes: 2 * config.KB,
	}

	userReg := user.NewRegistry()

	if err := userReg.LoadFromFile(registryPath); err != nil {
		return fmt.Errorf("could not load registry path: %w", err)
	}

	apiv1 := v1.APIV1{
		UserRegistry: userReg,
	}
	apiv1.Init()
	apiv1.Register(r)

	log.Info().Str("address", addr).Msg("Site is running")

	err := srv.ListenAndServe()

	apiv1.Deinit()

	return err
}
