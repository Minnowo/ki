package v1

import (
	"encoding/hex"
	"ki/src/config"
	"ki/src/handlers/storage"
	"ki/src/ui/pages"
	"net/http"

	"github.com/gorilla/mux"
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

	a.fmap.ReadFile(w, *key)
}
