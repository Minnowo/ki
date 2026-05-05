package ui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
)

//go:embed static
var fsStatic embed.FS

// RegisterStatic registers the ui on the root path.
func RegisterStatic(r *mux.Router) {
	var assetsStatic, _ = fs.Sub(fsStatic, "static")

	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(assetsStatic)))

	withSWHeader := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		if req.URL.Path == "/static/js/sw.js" {
			w.Header().Set("Service-Worker-Allowed", ServiceWorkerPrefix)
		}
		w.Header().Set("Content-Type", "application/javascript; charset=UTF-8")
		fileServer.ServeHTTP(w, req)
	})

	r.PathPrefix("/static/i/").Handler(fileServer)    // images
	r.PathPrefix("/static/c/").Handler(fileServer)    // css
	r.PathPrefix("/static/js/").Handler(withSWHeader) // javascript (with SW header)
}
