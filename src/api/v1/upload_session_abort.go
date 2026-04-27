package v1

import (
	"errors"
	"ki/src/api"
	"ki/src/api/auth"
	"ki/src/handlers/storage"
	"net/http"

	"github.com/rs/zerolog/log"
)

// file_upload_session_abort finalises a chunked upload and redirects the client to the download page.
func (a *APIV1) file_upload_session_abort(w http.ResponseWriter, r *http.Request) {

	username, ok := auth.GetUser(r)

	if !ok {
		api.Unauthorized(w)
		return
	}

	var sessionID storage.SessionToken

	if !getSessionID(r, &sessionID) {
		api.Done(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	err := a.fileStore.AbortUploadSession(sessionID, username)

	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "upload session not found")
		} else {
			log.Error().Err(err).Hex("id", sessionID[:]).Msg("complete chunked upload error")
			api.Done(w, http.StatusInternalServerError, "error finalising upload")
		}
		return
	}
}
