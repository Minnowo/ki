package cmd

import (
	"context"
	"fmt"
	"ki/src/api/middleware"
	v1 "ki/src/api/v1"
	"ki/src/config"
	"ki/src/handlers/user"
	"ki/src/ui"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func CmdServerMain(ctx context.Context, c *cli.Command) error {

	registryPath := c.Value("registry").(string)
	bindAddr := c.Value("bind").(string)
	port := c.Value("port").(int32)
	maxUploadByes := c.Value("max-upload-size").(int64)
	maxUploadChunkByes := c.Value("max-upload-chunk-size").(int64)
	tlsCert := c.Value("tls-cert").(string)

	if tlsCert != "" {
		if !fileExists(tlsCert+".crt") || !fileExists(tlsCert+".key") {
			return fmt.Errorf("got TLS cert: %s but could not find %s.crt or %s.key", tlsCert, tlsCert, tlsCert)
		}
	}

	if fileDir, ok := c.Value("file-dir").(string); ok {
		config.SetFileStorageDir(fileDir)
	}

	if trustedProxies, ok := c.Value("trusted-proxy").([]string); ok {
		config.ParseTrustedProxies(trustedProxies)
	}

	if secret, ok := c.Value("master-secret").(string); ok {
		config.SetMasterSecret(secret)
	}

	config.SetMaxUploadSize(maxUploadByes)
	config.SetMaxChunkSize(maxUploadChunkByes)

	var addr = fmt.Sprintf("%s:%d", bindAddr, port)

	r := mux.NewRouter()
	r.Use(middleware.Recoverer)

	ui.RegisterStatic(r)

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

	log.Info().Str("address", addr).Str("tls", tlsCert).Msg("Site is running")

	var err error
	if tlsCert != "" {
		err = srv.ListenAndServeTLS(tlsCert+".crt", tlsCert+".key")
	} else {
		err = srv.ListenAndServe()
	}

	apiv1.Deinit()

	return err
}
