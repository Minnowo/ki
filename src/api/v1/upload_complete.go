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

// file_upload_complete finalises a chunked upload and redirects the client to the download page.
func (a *APIV1) file_upload_complete(w http.ResponseWriter, r *http.Request) {

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

	key, err := a.fileStore.CompleteChunkedUpload(uploadId, username)

	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "upload session not found")
		} else {
			log.Error().Err(err).Hex("id", uploadId[:]).Msg("complete chunked upload error")
			api.Done(w, http.StatusInternalServerError, "error finalising upload")
		}
		return
	}

	url, err := a.router.Get("download").URL("fileIdHex", key.Hex())

	if err != nil {
		log.Panic().Str("fileIdHex", key.Hex()).Msg("could not build route url")
	}

	if r.Header.Get("X-Requested-With") == "js-form" {
		w.Header().Set("Location", url.String())
		api.Ok(w)
		return
	}

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}
