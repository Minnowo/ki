package v1

import (
	"errors"
	"ki/src/api"
	"ki/src/api/auth"
	"ki/src/handlers/storage"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// file_upload_abort cancels an in-progress chunked upload and deletes its temporary file.
func (a *APIV1) file_upload_abort(w http.ResponseWriter, r *http.Request) {

	username, ok := auth.GetUser(r)

	if !ok {
		api.Unauthorized(w)
		return
	}

	var uploadId storage.FileID

	if err := uploadId.FromHex(mux.Vars(r)["uploadId"]); err != nil {
		api.Done(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	err := a.fileStore.AbortChunkedUpload(uploadId, username)

	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "upload session not found")
		} else {
			log.Error().Err(err).Hex("id", uploadId[:]).Msg("abort upload error")
			api.Done(w, http.StatusInternalServerError, "error aborting upload")
		}
		return
	}

	api.Ok(w)
}
