package v1

import (
	"ki/src/api"
	"ki/src/api/auth"
	"ki/src/api/cookies"
	"ki/src/api/middleware"
	"ki/src/config"
	"ki/src/handlers/ratelimit"
	"ki/src/handlers/storage"
	"ki/src/handlers/user"
	"ki/src/pkg/csrf"
	"ki/src/pkg/proxy"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type APIV1 struct {
	router       *mux.Router
	fileStore    storage.FileUploadHandler
	rateLimiter  *ratelimit.RateLimiter
	csrfHandler  *csrf.Handler
	UserRegistry *user.UserRegistry
}

func (a *APIV1) Init() {

	store, err := storage.NewMixedFileStore(path.Join(config.FileStorageDir(), "data.db"))

	if err != nil {
		log.Panic().Err(err).Msg("could not create file store")
	}

	a.fileStore = storage.NewFileStore(config.FileStorageDir(), store, bcrypt.DefaultCost)
	a.fileStore.RunExpireCheckLoop(time.Second * 60)
	a.rateLimiter = ratelimit.New(rate.Every(time.Millisecond*1000), 1)
	a.csrfHandler = csrf.NewCSRFHandler(
		cookies.BasicCookieStore{CookieName: config.CSRF_COOKIE},
		csrf.ManualVerifyFormCSRF(true),
	)

	if a.UserRegistry == nil {
		log.Panic().Msg("user registry is nil")
	}
}

func (a *APIV1) Deinit() {
	a.fileStore.ShutdownExpireCheckLoop()
}

func (a *APIV1) Register(r *mux.Router) {

	r.Use(proxy.ProxyHeaders(config.TrustedProxies()))
	r.Use(a.csrfHandler.Protect)
	r.Use(auth.ParseAuth(config.SESSION_COOKIE, a.UserRegistry))

	a.router = r

	ui := r.Methods("GET").Subrouter()
	ui.Use(middleware.TemplNonce)
	ui.HandleFunc(api.ROUTE__LOGIN, a.ui_login)
	ui.HandleFunc(api.ROUTE__LOGOUT, a.ui_logout)
	ui.HandleFunc("/upload", a.ui_upload)
	ui.HandleFunc("/download", a.ui_download2)
	ui.HandleFunc("/download/{fileIdHex}", a.ui_download).Name("download")
	ui.HandleFunc("/", a.ui_upload)

	api := r.NewRoute().Subrouter()
	api.Use(middleware.NoCache)

	apiP := api.Methods("POST").Subrouter()
	apiP.HandleFunc("/api/login", a.api_login)

	apiAuth := apiP.NewRoute().Subrouter()
	apiAuth.Use(auth.RequireAuth())
	apiAuth.HandleFunc("/api/upload", a.file_upload)
	apiAuth.HandleFunc("/api/upload/begin", a.file_upload_begin)
	apiAuth.HandleFunc("/api/upload/{uploadId}/chunk", a.file_upload_chunk)
	apiAuth.HandleFunc("/api/upload/{uploadId}/complete", a.file_upload_complete)
	apiAuth.HandleFunc("/api/upload/{uploadId}/abort", a.file_upload_abort)

	apiG := api.Methods("GET").Subrouter()
	apiG.HandleFunc("/api/download/{fileIdHex}", a.file_download)
}
