package v1

import (
	"ki/src/api"
	"ki/src/handlers/storage"
	"ki/src/ui/pages"
	"net/http"

	"github.com/rs/zerolog/log"
)

func (a *APIV1) ui_download2(w http.ResponseWriter, r *http.Request) {
	view := pages.NewBaseViewFromReq(r)
	pages.PageDownload(&view).Render(r.Context(), w)
}

func (a *APIV1) ui_download(w http.ResponseWriter, r *http.Request) {

	var key storage.FileID

	if !getFileID(r, &key) {
		api.NotFound(w)
		return
	}

	file := a.fileStore.FileMetadata(key)

	if file == nil {
		api.NotFound(w)
		return
	}

	pages.PageDownloadFile(&pages.PageDownloadFileView{
		BaseView: pages.NewBaseViewFromReq(r),
		File:     file,
		FileID:   key,
	}).Render(r.Context(), w)
}

func (a *APIV1) file_download(w http.ResponseWriter, r *http.Request) {

	var key storage.FileID

	if !getFileID(r, &key) {
		api.NotFound(w)
		return
	}

	_, password, ok := r.BasicAuth()

	if !ok {
		password = ""
	}

	err := a.fileStore.ReadFile(w, key, password)

	switch {
	case err == storage.ErrNeedsAuth:
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		api.Done(w, http.StatusUnauthorized, "This file requires a password. Provide basic auth with any username and the password for this file. (The username will be ignored)")

	case err == storage.ErrFileNotFound:
		api.Done(w, http.StatusNotFound, "The file was not found")

	case err == storage.ErrFileExpired:
		api.Done(w, http.StatusForbidden, "The file has expired")

	case err != nil:
		log.Error().Err(err).Msg("error reading file to client")
		api.Done(w, http.StatusInternalServerError, "Error reading file")
	}
}
