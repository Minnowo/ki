package v1

import (
	"encoding/hex"
	"ki/src/api"
	"ki/src/config"
	"ki/src/handlers/storage"
	"ki/src/ui/pages"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func getFileID(r *http.Request) *storage.FileID {

	vars := mux.Vars(r)
	hexStr, ok := vars["fileIdHex"]
	if !ok {
		return nil
	}

	id, err := hex.DecodeString(hexStr)

	if err != nil || len(id) != config.FILE_ID_SIZE {
		return nil
	}

	fileID := storage.FileID(id)

	return &fileID
}

func (a *APIV1) ui_download2(w http.ResponseWriter, r *http.Request) {
	view := pages.NewBaseViewFromReq(r)
	pages.PageDownload(&view).Render(r.Context(), w)
}

func (a *APIV1) ui_download(w http.ResponseWriter, r *http.Request) {

	key := getFileID(r)

	if key == nil {
		api.NotFound(w)
		return
	}

	file := a.fmap.GetFile(*key)

	if file == nil {
		api.NotFound(w)
		return
	}

	pages.PageDownloadFile(&pages.PageDownloadFileView{
		BaseView: pages.NewBaseViewFromReq(r),
		File:     file,
		FileID:   *key,
	}).Render(r.Context(), w)
}

func (a *APIV1) file_download(w http.ResponseWriter, r *http.Request) {

	key := getFileID(r)

	if key == nil {
		api.NotFound(w)
		return
	}

	_, password, ok := r.BasicAuth()

	if !ok {
		password = ""
	}

	err := a.fmap.ReadFile(w, *key, password)

	if err == storage.ErrNeedsAuth {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		api.Done(w, http.StatusUnauthorized, "This file requires a password. Provide basic auth with any username and the password for this file. (The username will be ignored)")
	} else if err != nil {
		log.Error().Err(err).Msg("error reading file to client")
		api.Done(w, http.StatusInternalServerError, "Error reading file")
	}
}
