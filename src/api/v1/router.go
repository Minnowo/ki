package v1

import (
	"ki/src/api/middleware"
	"ki/src/config"
	"ki/src/handlers/ratelimit"
	"ki/src/handlers/storage"
	"ki/src/handlers/user"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type APIV1 struct {
	router       *mux.Router
	fmap         storage.FileMap
	fmapDChan    chan bool
	rateLimiter  *ratelimit.RateLimiter
	UserRegistry *user.UserRegistry
}

func (a *APIV1) Init() {
	a.fmap = storage.NewFileMap(config.AES_KEY_SIZE, bcrypt.DefaultCost)
	a.fmapDChan = storage.NewFileMapExpireCheckD(time.Minute, &a.fmap)
	a.rateLimiter = ratelimit.New(rate.Every(time.Millisecond*1000), 1)

	if a.UserRegistry == nil {
		log.Panic().Msg("user registry is nil")
	}
}

func (a *APIV1) Deinit() {
	a.fmapDChan <- true
}

func (a *APIV1) Register(r *mux.Router) {

	a.router = r

	ui := r.Methods("GET").Subrouter()
	ui.HandleFunc("/login", a.ui_login)
	ui.HandleFunc("/logout", a.ui_logout)
	ui.HandleFunc("/upload", a.ui_upload)
	ui.HandleFunc("/download", a.ui_download2)
	ui.HandleFunc("/download/{fileIdHex}", a.ui_download).Name("download")
	ui.HandleFunc("/", a.ui_upload)

	api := r.NewRoute().Subrouter()
	api.Use(middleware.NoCache)

	apiP := api.Methods("POST").Subrouter()
	apiP.HandleFunc("/api/login", a.api_login)

	apiAuth := apiP.NewRoute().Subrouter()
	apiAuth.Use(middleware.Auth(a.UserRegistry), middleware.RequireAuth())
	apiAuth.HandleFunc("/api/upload", a.file_upload)

	apiG := api.Methods("GET").Subrouter()
	apiG.HandleFunc("/api/download/{fileIdHex}", a.file_download)
}
