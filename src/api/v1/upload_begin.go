package v1

import (
	"errors"
	"ki/src/api"
	"ki/src/api/auth"
	"ki/src/config"
	"ki/src/handlers/form"
	"ki/src/handlers/storage"
	"ki/src/ui/formkeys"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// file_upload_begin starts a chunked upload session. The client sends all file metadata
// (expiry, downloads, password, filename) here and receives an upload_id to use for
// subsequent chunk and complete requests.
func (a *APIV1) file_upload_begin(w http.ResponseWriter, r *http.Request) {

	username, ok := auth.GetUser(r)

	if !ok {
		api.Unauthorized(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, config.MAX_UPLOAD_SESSION_FORM_SIZE)

	if err := r.ParseMultipartForm(config.MAX_UPLOAD_SESSION_FORM_SIZE); err != nil {
		api.Done(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	// Verify CSRF manually (ManualVerifyFormCSRF is enabled on this router).
	csrfTok := r.FormValue(formkeys.CSRF_FORM_FIELD)

	if !a.csrfHandler.VerifyStr(r, csrfTok) {
		api.Done(w, http.StatusForbidden, "invalid csrf token")
		return
	}

	var days int32
	var hours int32
	var minutes int32
	var downloads int32

	if !form.ReadInt32(r, &days, formkeys.UPLOAD_FORM_EXPIRE_DAYS) {
		api.Done(w, http.StatusBadRequest, "error reading expire days")
		return
	}

	if !form.ReadInt32(r, &hours, formkeys.UPLOAD_FORM_EXPIRE_HOURS) {
		api.Done(w, http.StatusBadRequest, "error reading expire hours")
		return
	}

	if !form.ReadInt32(r, &minutes, formkeys.UPLOAD_FORM_EXPIRE_MINUTES) {
		api.Done(w, http.StatusBadRequest, "error reading expire minutes")
		return
	}

	if !form.ReadInt32(r, &downloads, formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS) {
		api.Done(w, http.StatusBadRequest, "error reading expire downloads")
		return
	}

	if days < 0 || hours < 0 || minutes < 0 {
		api.Done(w, http.StatusBadRequest, "expiry days, hours, and minutes must all be >= 0")
		return
	}

	if downloads < 1 {
		api.Done(w, http.StatusBadRequest, "number of downloads must be greater than 0")
		return
	}

	memoryOnly, err := strconv.ParseBool(r.FormValue(formkeys.UPLOAD_FORM_MEMORY_ONLY))

	if err != nil {
		api.Done(w, http.StatusBadRequest, "invalid boolean value for memory only")
		return
	}

	password := r.FormValue(formkeys.UPLOAD_FORM_PASSWORD)

	if len(password) > config.MAX_PASSWORD_LENGTH {
		api.Donef(w, http.StatusBadRequest, "password exceeds maximum length of %d", config.MAX_PASSWORD_LENGTH)
		return
	}

	filename := strings.TrimSpace(r.FormValue(formkeys.UPLOAD_FORM_FILENAME))

	upload := storage.FileUpload{
		AllowedDownloads: int(downloads),
		Filename:         filename,
		Password:         password,
		MemoryOnly:       memoryOnly,
		ExpiresIn: (24*time.Hour*time.Duration(days) +
			time.Hour*time.Duration(hours) +
			time.Minute*time.Duration(minutes)),
	}

	uploadId, err := a.fileStore.BeginChunkedUpload(upload, username)

	if err != nil {
		if errors.Is(err, storage.ErrInvalidUpload) {
			log.Debug().Err(err).Msg("begin chunked upload error")
			api.Done(w, http.StatusBadRequest, err.Error())
		} else {
			log.Error().Err(err).Msg("begin chunked upload error")
			api.Done(w, http.StatusInternalServerError, "error creating upload session")
		}
		return
	}

	api.WriteJSON(w, struct {
		UploadID     string `json:"upload_id"`
		MaxChunkSize int64  `json:"max_chunk_size"`
	}{
		UploadID:     uploadId.Hex(),
		MaxChunkSize: config.MaxChunkSize() - config.KB,
	})
}
