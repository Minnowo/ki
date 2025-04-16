package v1

import (
	"ki/src/handlers/storage"
	"time"

	"github.com/gorilla/mux"
)

type APIV1 struct {
	fmap      storage.FileMap
	fmapDChan chan bool
}

func (a *APIV1) Init() {
	a.fmap = storage.NewFileMap()
	a.fmapDChan = storage.NewFileMapExpireCheckD(time.Minute, &a.fmap)
}
func (a *APIV1) Deinit() {
	a.fmapDChan <- true
}

func (a *APIV1) Register(r *mux.Router) {

	r.HandleFunc("/", a.ui_upload)
	// r.HandleFunc("/about", a.ui_about)
	r.HandleFunc("/upload", a.ui_upload)
	r.HandleFunc("/download/{fileIdHex}", a.ui_download)
	// r.HandleFunc("/links", a.ui_links)

	r.HandleFunc("/api/upload", a.file_upload)
	r.HandleFunc("/api/download/{fileIdHex}", a.file_download)
}

// func (a *APIV1) ui_home(w http.ResponseWriter, r *http.Request) {
// 	pages.Portal().Render(r.Context(), w)
// }
// func (a *APIV1) ui_about(w http.ResponseWriter, r *http.Request) {
// 	pages.PageAbout().Render(r.Context(), w)
// }
// func (a *APIV1) ui_links(w http.ResponseWriter, r *http.Request) {
// 	pages.PageLinks().Render(r.Context(), w)
// }
