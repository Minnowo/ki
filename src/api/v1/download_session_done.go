package v1

import (
	"ki/src/api"
	"ki/src/handlers/storage"
	"net/http"
)

// download_session_done cancels an in-progress chunked download session.
func (a *APIV1) download_session_done(w http.ResponseWriter, r *http.Request) {

	var fileID storage.FileID

	if !getFileID(r, &fileID) {
		api.Done(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	var sessionID storage.SessionToken

	if !getSessionID(r, &sessionID) {
		api.Done(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	a.fileStore.AbortDownloadSession(sessionID, fileID)

	api.Done(w, http.StatusOK, "ok")
}
