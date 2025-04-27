package v1

import (
	"ki/src/api"
	"ki/src/config"
	"ki/src/ui/formkeys"
	"ki/src/ui/pages"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func (a *APIV1) ui_login(w http.ResponseWriter, r *http.Request) {
	pages.PageLogin().Render(r.Context(), w)
}

func (a *APIV1) api_login(w http.ResponseWriter, r *http.Request) {

	r.Body = http.MaxBytesReader(w, r.Body, config.MAX_LOGIN_FORM_SIZE)

	err := r.ParseMultipartForm(config.MAX_LOGIN_FORM_SIZE)

	if err != nil {
		log.Debug().Err(err).Msg("failed to get multipartReader")
		api.Done(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	log.Info().Msg("handling login call")

	username := r.PostFormValue(formkeys.LOGIN_FORM_USERNAME)
	password := r.PostFormValue(formkeys.LOGIN_FORM_PASSWORD)

	if username == "" || password == "" {
		api.Done(w, http.StatusBadRequest, "missing username or password")
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
		Expires:  time.Now().Add(time.Hour * 24),
		Secure:   true,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/upload", http.StatusSeeOther)
}
