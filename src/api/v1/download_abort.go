package v1

import (
	"ki/src/api"
	"ki/src/handlers/storage"
	"net/http"

	"github.com/gorilla/mux"
)

// download_abort cancels an in-progress chunked download session.
func (a *APIV1) download_abort(w http.ResponseWriter, r *http.Request) {

	var downloadID storage.FileID

	if err := downloadID.FromHex(mux.Vars(r)["downloadId"]); err != nil {
		api.Done(w, http.StatusBadRequest, "invalid download ID")
		return
	}

	// ErrSessionNotFound is fine — session may have already finished or expired.
	a.fileStore.AbortChunkedDownload(downloadID)

	api.Done(w, http.StatusOK, "ok")
}
