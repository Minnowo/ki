package v1

import (
	"errors"
	"io"
	"ki/src/api"
	"ki/src/config"
	"ki/src/handlers/form"
	"ki/src/handlers/storage"
	"ki/src/ui/formkeys"
	"ki/src/ui/pages"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func (a *APIV1) ui_upload(w http.ResponseWriter, r *http.Request) {
	pages.PageUpload(&pages.PageUploadView{
		BaseView: pages.NewBaseViewFromReq(r),
	}).Render(r.Context(), w)
}

func (a *APIV1) file_upload(w http.ResponseWriter, r *http.Request) {

	r.Body = http.MaxBytesReader(w, r.Body, config.MaxUploadSize())
	rc := http.NewResponseController(w)

	log.Info().Msg("handling an upload post")

	multipartReader, err := r.MultipartReader()

	if err != nil {
		log.Debug().Err(err).Msg("failed to get multipartReader")
		api.Done(w, http.StatusBadRequest, "failed to get a multipart reader")
		return
	}

	var days int32 = -1
	var hours int32 = -1
	var minutes int32 = -1
	var downloads int32 = -1
	var password string = ""
	var key *storage.FileID = nil
	var finished bool = false

	startTime := time.Now()

	for {
		// Note that calling 'return' or 'break' in this loop will not close the part.
		// This is intentional because when the part is closed it will read and discard until the start of the next part.
		// If for example the user uploads a 50gb file in a field we expect a string.
		// If we read 50 bytes of that and then close it, it will read and discard the other 50gb.
		// Instead we abort the connection so the server can ignore the rest of the form.
		//
		// This sadly doesn't give the user any reason as to why their upload failed.
		// They just get a vague message about connection being reset, but this is better than allowing them to waste bandwidth.
		part, err := multipartReader.NextPart()

		if err != nil {
			if err == io.EOF {
				break
			}
			api.Done(w, http.StatusBadRequest, "error getting a part")
			return
		}

		var ok bool

		switch part.FormName() {

		case formkeys.UPLOAD_FORM_EXPIRE_DAYS:

			if days, ok = form.ReadFormInt(part); !ok {
				api.Done(w, http.StatusBadRequest, "error reading expire days")
				return
			}
			break

		case formkeys.UPLOAD_FORM_EXPIRE_HOURS:

			if hours, ok = form.ReadFormInt(part); !ok {
				api.Done(w, http.StatusBadRequest, "error reading expire hours")
				return
			}
			break

		case formkeys.UPLOAD_FORM_EXPIRE_MINUTES:

			if minutes, ok = form.ReadFormInt(part); !ok {
				api.Done(w, http.StatusBadRequest, "error reading expire minutes")
				return
			}
			break

		case formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS:

			if downloads, ok = form.ReadFormInt(part); !ok {
				api.Done(w, http.StatusBadRequest, "error reading expire downloads")
				return
			}
			break

		case formkeys.UPLOAD_FORM_PASSWORD:

			if password, ok = form.ReadFormString(config.MAX_PASSWORD_LENGTH, part); !ok {
				api.Done(w, http.StatusBadRequest, "error reading password")
				return
			}

			if len(password) > config.MAX_PASSWORD_LENGTH {
				api.Donef(w, http.StatusBadRequest, "password exceeds maximum length of %d", config.MAX_PASSWORD_LENGTH)
				return
			}
			break

		case formkeys.UPLOAD_FORM_FILE:

			if days == -1 || hours == -1 || minutes == -1 || downloads == -1 {
				api.Donef(w, http.StatusBadRequest, "the file must be the last item in the form")
				return
			}

			if days < 0 || hours < 0 || minutes < 0 {
				api.Donef(w, http.StatusBadRequest, "expirey days, hours, and minutes must all be greater than or equal to 0")
				return
			}

			if downloads < 1 {
				api.Donef(w, http.StatusBadRequest, "number of downloads must be greater than 0")
				return
			}

			upload := storage.SafeFileUpload{
				AllowedDownloads: int(downloads),
				Filename:         strings.TrimSpace(part.FileName()),
				Password:         password,
				ExpiresIn: ((24 * time.Hour * time.Duration(days)) +
					(time.Hour * time.Duration(hours)) +
					(time.Minute * time.Duration(minutes))),
			}

			timeoutHelper := func(_ int) {

				// make sure the request never times out if we're reading data
				deadline := time.Now().Add(time.Second * 15)
				rc.SetReadDeadline(deadline)
				rc.SetWriteDeadline(deadline)
			}

			key, err = a.fmap.SaveFileWithProgress(upload, part, timeoutHelper)

			if err != nil {

				if errors.Is(err, storage.ErrInvalidUpload) {
					log.Debug().Err(err).Msg("save file error")
					api.Done(w, http.StatusBadRequest, err.Error())
				} else {
					log.Error().Err(err).Msg("save file error")
					api.Donef(w, http.StatusInternalServerError, "error while processing file")
				}

				return
			}

			finished = true

			break

		default:
			api.Donef(w, http.StatusBadRequest, "got unexpected form part")
			return
		}
	}

	if !finished {
		api.Done(w, http.StatusBadRequest, "unexpected EOF")
		return
	}

	url, err := a.router.Get("download").URL("fileIdHex", key.Hex())

	if err != nil {
		log.Panic().Str("fileIdHex", key.Hex()).Msg("could not build route url")
	}

	http.Redirect(w, r, url.String(), http.StatusSeeOther)

	stopTime := time.Now()

	log.Info().
		Str("took", stopTime.Sub(startTime).String()).
		Str("id", key.Hex()).
		Msg("upload success")
}
