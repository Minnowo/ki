package v1

import (
	"encoding/hex"
	"ki/src/config"
	"ki/src/ui/formkeys"
	"ki/src/ui/pages"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

func (a *APIV1) ui_upload(w http.ResponseWriter, r *http.Request) {
	pages.PageUpload().Render(r.Context(), w)
}

func (a *APIV1) file_upload(w http.ResponseWriter, r *http.Request) {

	r.ParseMultipartForm(config.MAX_UPLOAD_MEMORY)

	days, err := strconv.Atoi(r.FormValue(formkeys.UPLOAD_FORM_EXPIRE_DAYS))

	if err != nil {
		http.Error(w, "Bad number of days", http.StatusBadRequest)
		return
	}

	hours, err := strconv.Atoi(r.FormValue(formkeys.UPLOAD_FORM_EXPIRE_HOURS))

	if err != nil {
		http.Error(w, "Bad number of hours", http.StatusBadRequest)
		return
	}

	minutes, err := strconv.Atoi(r.FormValue(formkeys.UPLOAD_FORM_EXPIRE_MINUTES))

	if err != nil {
		http.Error(w, "Bad number of minutes", http.StatusBadRequest)
		return
	}

	downloads, err := strconv.Atoi(r.FormValue(formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS))

	if err != nil {
		http.Error(w, "Bad number of downloads", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile(formkeys.UPLOAD_FORM_FILE)
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	log.Info().
		Str("file", handler.Filename).
		Int64("size", handler.Size).
		Msg("Got file")

	expires := time.Now().
		Add(time.Hour*time.Duration(24)*time.Duration(days) + time.Hour*time.Duration(hours) + time.Minute*time.Duration(minutes))

	key, err := a.fmap.SaveFile(file, expires, downloads, handler.Filename)

	if err != nil {
		log.Error().Err(err).Msg("error handling file")
		http.Error(w, "unknown error while processing file", http.StatusInternalServerError)
		return
	}
	log.Info().Str("hash", hex.EncodeToString(key[:])).Msg("got file hash")

	defer http.Redirect(w, r, "/download/"+key.Hex(), http.StatusSeeOther)

}
