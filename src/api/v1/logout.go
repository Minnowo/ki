package v1

import (
	"ki/src/config"
	"net/http"
)

func (a *APIV1) ui_logout(w http.ResponseWriter, r *http.Request) {

	authToken, err := r.Cookie(config.SESSION_COOKIE)

	if err == nil {

		authToken.MaxAge = -1
		authToken.Value = ""
		http.SetCookie(w, authToken)
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
