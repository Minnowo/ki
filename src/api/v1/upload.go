package v1

import (
	"io"
	"ki/src/config"
	"ki/src/handlers/form"
	"ki/src/handlers/storage"
	"ki/src/ui/formkeys"
	"ki/src/ui/pages"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func (a *APIV1) ui_upload(w http.ResponseWriter, r *http.Request) {
	pages.PageUpload().Render(r.Context(), w)
}

func (a *APIV1) file_upload(w http.ResponseWriter, r *http.Request) {

	r.Body = http.MaxBytesReader(w, r.Body, config.MAX_UPLOAD_SIZE)
	rc := http.NewResponseController(w)

	log.Info().Msg("handling an upload post")

	multipartReader, err := r.MultipartReader()

	if err != nil {
		log.Debug().Err(err).Msg("failed to get multipartReader")
		http.Error(w, "failed to get a multipart reader", http.StatusBadRequest)
		return
	}

	var days int32 = -1
	var hours int32 = -1
	var minutes int32 = -1
	var downloads int32 = -1
	var fileSize int64 = 0
	var password string = ""
	var key *storage.FileID = nil
	var finished bool = false

	startTime := time.Now()

	for {
		part, err := multipartReader.NextPart()

		if err != nil {
			if err == io.EOF {
				break
			}
			http.Error(w, "error getting a part", http.StatusBadRequest)
			return
		}

		var ok bool

		switch part.FormName() {

		case formkeys.UPLOAD_FORM_EXPIRE_DAYS:

			if days, ok = form.ReadFormInt(part); !ok {
				http.Error(w, "error reading expire days", http.StatusBadRequest)
				return
			}
			break

		case formkeys.UPLOAD_FORM_EXPIRE_HOURS:

			if hours, ok = form.ReadFormInt(part); !ok {
				http.Error(w, "error reading expire hours", http.StatusBadRequest)
				return
			}
			break

		case formkeys.UPLOAD_FORM_EXPIRE_MINUTES:

			if minutes, ok = form.ReadFormInt(part); !ok {
				http.Error(w, "error reading expire minutes", http.StatusBadRequest)
				return
			}
			break

		case formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS:

			if downloads, ok = form.ReadFormInt(part); !ok {
				http.Error(w, "error reading expire downloads", http.StatusBadRequest)
				return
			}
			break

		case formkeys.UPLOAD_FORM_PASSWORD:

			if password, ok = form.ReadFormString(config.MAX_PASSWORD_LENGTH, part); !ok {
				http.Error(w, "error reading password", http.StatusBadRequest)
				return
			}
			break

		case formkeys.UPLOAD_FORM_FILE:

			if days == -1 || hours == -1 || minutes == -1 || downloads == -1 {
				http.Error(w, "the file must be the last item in the form", http.StatusBadRequest)
				return
			}

			filename := part.FileName()

			if len(filename) == 0 {
				http.Error(w, "could not read filename", http.StatusBadRequest)
				return
			}

			log.Info().
				Int32("days", days).
				Int32("hours", hours).
				Int32("minutes", minutes).
				Int32("downloads", downloads).
				Str("password", password).
				Msg("got expirey")

			expires := time.Now().
				Add(time.Hour * time.Duration(24) * time.Duration(days)).
				Add(time.Hour * time.Duration(hours)).
				Add(time.Minute * time.Duration(minutes))

			timeoutHelper := func(n int) {
				rc.SetReadDeadline(time.Now().Add(time.Second * 15))
				rc.SetWriteDeadline(time.Now().Add(time.Second * 15))
				fileSize += int64(n)
			}

			key, err = a.fmap.SaveFile(part, expires, int(downloads), filename, password, timeoutHelper)

			if err != nil {
				log.Error().Err(err).Msg("error processing file")
				http.Error(w, "unknown error while processing file", http.StatusInternalServerError)
				return
			}

			finished = true

			break

		default:
			http.Error(w, "error getting a part", http.StatusBadRequest)
			return
		}

		part.Close()
	}

	if !finished {
		http.Error(w, "unexpected EOF", http.StatusBadRequest)
		return
	}

	log.Info().Str("hash", key.Hex()).Msg("got file hash")
	http.Redirect(w, r, "/download/"+key.Hex(), http.StatusSeeOther)

	stopTime := time.Now()
	duration := startTime.Sub(stopTime)

	log.Info().
		Str("time", duration.String()).
		Int64("size", fileSize).
		Msg("upload success")
}
