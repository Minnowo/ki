package v1

import (
	"ki/src/api"
	"ki/src/config"
	"ki/src/handlers/storage"
	"ki/src/ui/formkeys"
	"ki/src/ui/pages"
	"net/http"

	"github.com/rs/zerolog/log"
)

// ui_download3 redirects to the download page.
// This route is never supposed to be called by the UI, since the service worker should intercept it and start the download.
// If the service worker doesn't, then we get here and can redirect to the download page.
func (a *APIV1) ui_download3(w http.ResponseWriter, r *http.Request) {

	var fileID storage.FileID

	if !getFileID(r, &fileID) {
		api.Done(w, http.StatusBadRequest, "invalid FileID")
		return
	}

	url, err := a.router.Get("download").URL("fileIdHex", fileID.Hex())

	if err != nil {
		log.Panic().Str("fileIdHex", fileID.Hex()).Msg("could not build route url")
	}

	http.Redirect(w, r, url.String(), http.StatusSeeOther)
}

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

// file_download serves a file, reading the password from basic auth (for curl / wget).
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

	a.serveFile(w, key, password, true)
}

// file_download_form serves a file, reading the password from a posted form (for browsers).
func (a *APIV1) file_download_form(w http.ResponseWriter, r *http.Request) {

	var key storage.FileID

	if !getFileID(r, &key) {
		api.NotFound(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, config.MAX_DOWNLOAD_FORM_SIZE)

	if err := r.ParseForm(); err != nil {
		api.Done(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	csrfTok := r.PostFormValue(formkeys.CSRF_FORM_FIELD)

	if csrfTok == "" || !a.csrfHandler.VerifyStr(r, csrfTok) {
		api.Done(w, http.StatusForbidden, "invalid csrf")
		return
	}

	a.serveFile(w, key, r.PostFormValue(formkeys.DOWNLOAD_FORM_PASSWORD), false)
}

// serveFile writes the file to the client.
// basicAuth controls whether a missing / wrong password asks the browser for basic auth credentials.
func (a *APIV1) serveFile(w http.ResponseWriter, key storage.FileID, password string, basicAuth bool) {

	err := a.fileStore.ReadFile(w, key, password)

	switch {
	case err == storage.ErrNeedsAuth && basicAuth:
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		api.Done(w, http.StatusUnauthorized, "This file requires a password. Provide basic auth with any username and the password for this file. (The username will be ignored)")

	case err == storage.ErrNeedsAuth:
		api.Done(w, http.StatusUnauthorized, "The password is incorrect")

	case err == storage.ErrFileNotFound:
		api.Done(w, http.StatusNotFound, "The file was not found")

	case err == storage.ErrFileExpired:
		api.Done(w, http.StatusForbidden, "The file has expired")

	case err != nil:
		log.Error().Err(err).Msg("error reading file to client")
		api.Done(w, http.StatusInternalServerError, "Error reading file")
	}
}
