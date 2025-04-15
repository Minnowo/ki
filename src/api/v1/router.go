package v1

import (
	"net/http"

	"ki/src/handlers/storage"
	"ki/src/ui/pages"

	"github.com/gorilla/mux"
)

type APIV1 struct {
	fmap storage.FileMap
}

func (a *APIV1) Init() {
	a.fmap = storage.NewFileMap()
}

func (a *APIV1) Register(r *mux.Router) {

	r.HandleFunc("/", a.ui_home)
	r.HandleFunc("/about", a.ui_about)
	r.HandleFunc("/upload", a.ui_upload)
	r.HandleFunc("/download/{fileIdHex}", a.ui_download)
	r.HandleFunc("/links", a.ui_links)

	r.HandleFunc("/api/upload", a.file_upload)
	r.HandleFunc("/api/download/{fileIdHex}", a.file_download)
}

func (a *APIV1) ui_home(w http.ResponseWriter, r *http.Request) {
	pages.Portal().Render(r.Context(), w)
}
func (a *APIV1) ui_about(w http.ResponseWriter, r *http.Request) {
	pages.PageAbout().Render(r.Context(), w)
}
func (a *APIV1) ui_links(w http.ResponseWriter, r *http.Request) {
	pages.PageLinks().Render(r.Context(), w)
}
