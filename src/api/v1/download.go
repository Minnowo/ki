package v1

import (
	"encoding/hex"
	"ki/src/config"
	"ki/src/handlers/storage"
	"ki/src/ui/pages"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

func getFileID(a *APIV1, w http.ResponseWriter, r *http.Request) *storage.FileID {

	vars := mux.Vars(r)
	hexStr, ok := vars["fileIdHex"]
	if !ok {
		http.Error(w, "", http.StatusNotFound)
		return nil
	}

	id, err := hex.DecodeString(hexStr)

	if err != nil || len(id) != config.FILE_ID_SIZE {
		http.Error(w, "", http.StatusNotFound)
		return nil
	}

	fileID := storage.FileID(id)

	return &fileID
}

func (a *APIV1) ui_download(w http.ResponseWriter, r *http.Request) {

	key := getFileID(a, w, r)

	if key == nil {
		return
	}

	file := a.fmap.GetFile(*key)

	if file == nil {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	pages.PageDownload(&pages.PageDownloadView{
		BaseView: pages.NewBaseViewFromReq(r),
		File:     file,
		FileID:   *key,
	}).Render(r.Context(), w)
}

func (a *APIV1) file_download(w http.ResponseWriter, r *http.Request) {

	key := getFileID(a, w, r)

	if key == nil {
		return
	}

	_, password, ok := r.BasicAuth()

	if !ok {
		password = ""
	}

	err := a.fmap.ReadFile(w, *key, password)

	if err == storage.ErrNeedsAuth {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "This file requires a password. Provide basic auth with any username and the password for this file. (The username will be ignored)", http.StatusUnauthorized)
	} else if err != nil {
		log.Error().Err(err).Msg("error reading file to client")
	}
}
