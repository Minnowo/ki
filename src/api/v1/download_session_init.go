package v1

import (
	"errors"
	"ki/src/api"
	"ki/src/handlers/storage"
	"net/http"
)

// download_session_init creates a chunked download session for the given file.
// The download counts as one download regardless of how many chunks are fetched.
func (a *APIV1) download_session_init(w http.ResponseWriter, r *http.Request) {

	var fileID storage.FileID

	if !getFileID(r, &fileID) {
		api.NotFound(w)
		return
	}

	_, password, ok := r.BasicAuth()

	if !ok {
		password = ""
	}

	sessionID, meta, err := a.fileStore.BeginChunkedDownload(fileID, password)

	if err != nil {
		switch {
		case errors.Is(err, storage.ErrNeedsAuth):
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			api.Done(w, http.StatusUnauthorized, "This file requires a password. Provide basic auth with any username and the file password.")
		case errors.Is(err, storage.ErrFileNotFound):
			api.Done(w, http.StatusNotFound, "The file was not found")
		case errors.Is(err, storage.ErrFileExpired):
			api.Done(w, http.StatusForbidden, "The file has expired")
		default:
			api.Done(w, http.StatusInternalServerError, "Error starting download")
		}
		return
	}

	api.WriteJSON(w, struct {
		SessionID string `json:"session_id"`
		FileSize  int64  `json:"file_size"`
		Filename  string `json:"filename"`
	}{
		SessionID: sessionID.Hex(),
		FileSize:  meta.Size,
		Filename:  meta.Name,
	})
}
