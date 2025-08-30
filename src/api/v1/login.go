package v1

import (
	"ki/src/api"
	"ki/src/config"
	"ki/src/pkg/request"
	"ki/src/ui/formkeys"
	"ki/src/ui/pages"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func (a *APIV1) ui_login(w http.ResponseWriter, r *http.Request) {
	view := pages.NewBaseViewFromReq(r)
	pages.PageLogin(&view).Render(r.Context(), w)
}

func (a *APIV1) api_login(w http.ResponseWriter, r *http.Request) {

	r.Body = http.MaxBytesReader(w, r.Body, config.MAX_LOGIN_FORM_SIZE)

	if err := r.ParseMultipartForm(config.MAX_LOGIN_FORM_SIZE); err != nil {
		log.Debug().Err(err).Msg("failed to get multipartReader")
		api.Done(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	log.Info().Msg("handling login call")

	csrfTok := r.PostFormValue(formkeys.CSRF_FORM_FIELD)

	if csrfTok == "" || !a.csrfHandler.VerifyStr(r, csrfTok) {
		api.Done(w, http.StatusForbidden, "invalid csrf")
		return
	}

	username := r.PostFormValue(formkeys.LOGIN_FORM_USERNAME)
	password := r.PostFormValue(formkeys.LOGIN_FORM_PASSWORD)

	if username == "" || password == "" {
		api.Done(w, http.StatusBadRequest, "missing username or password")
		return
	}

	if len(username) > config.MAX_USERNAME_LENGTH || len(password) > config.MAX_USER_PASSWORD_LENGTH {
		api.Done(w, http.StatusBadRequest, "username or password is to long")
		return
	}

	if !a.UserRegistry.HasUser(username) {
		api.Done(w, http.StatusUnauthorized, "username or password is incorrect")
		return
	}

	// guard this behind the HasUser to prevent someone from trying a million different names to waste memory
	if !a.rateLimiter.Allow(username) {
		api.Done(w, http.StatusTooManyRequests, "logins are rate limted")
		return
	}

	token, ok := a.UserRegistry.Login(username, password)

	if !ok {
		api.Done(w, http.StatusUnauthorized, "username or password is incorrect")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:     "/",
		Name:     config.SESSION_COOKIE,
		Value:    token,
		MaxAge:   int(config.SESSION_MAX_AGE.Seconds()),
		Expires:  time.Now().Add(config.SESSION_MAX_AGE),
		SameSite: http.SameSiteStrictMode,
		Secure:   request.IsTLS(r),
		HttpOnly: true,
	})

	http.Redirect(w, r, "/upload", http.StatusSeeOther)
}
