package assets

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
)

//go:embed static
var fsStatic embed.FS

// Register registers the ui on the root path.
func Register(r *mux.Router) {

	var assetsStatic, _ = fs.Sub(fsStatic, "static")
	var static = http.StripPrefix("/static/", http.FileServer(http.FS(assetsStatic)))
	r.PathPrefix("/static/i/").Handler(static) // images
	r.PathPrefix("/static/c/").Handler(static) // css
}
