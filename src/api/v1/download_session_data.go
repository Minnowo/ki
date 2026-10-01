package v1

import (
	"errors"
	"fmt"
	"io"
	"ki/src/api"
	"ki/src/config"
	"ki/src/handlers/storage"
	"ki/src/pkg/request"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// download_session_data writes a chunk of an active download session as a binary response.
//
// The chunk starts at the Range header's start, or where the last chunk ended if there is no Range header.
// The last chunk can be requested again if it failed to arrive, but nothing before it, see DownloadSession.StartChunk.
// The session stays open until the client calls download_session_done, so the final chunk can be retried too.
func (a *APIV1) download_session_data(w http.ResponseWriter, r *http.Request) {

	var fileID storage.FileID

	if !getFileID(r, &fileID) {
		api.Done(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	var sessionID storage.SessionToken

	if !getSessionID(r, &sessionID) {
		api.Done(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	start, stop, err := request.ParseRangeHeader(r)

	if err != nil {
		start = -1
		stop = -1
	}

	rc := http.NewResponseController(w)

	err = a.fileStore.WithDownloadSession(sessionID, fileID, func(session *storage.DownloadSession) error {

		if start < 0 {
			start = session.BytesWritten
		}

		if start >= session.TotalBytes {
			return storage.ErrInvalidSeek
		}

		length := min(config.MaxChunkSize(), session.TotalBytes-start)

		// the Range header's end is inclusive
		if stop != -1 {
			if stop < start {
				return storage.ErrInvalidSeek
			}
			length = min(length, stop-start+1)
		}

		log.Info().Int64("start", start).Int64("stop", stop).Int64("size", length).Msg("chunk download range")

		// before setting any headers, so a refused start can still be reported as an error
		if r.Method != http.MethodHead {
			if err := session.StartChunk(start); err != nil {
				return err
			}
		}

		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))

		if session.File.Name == "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", session.FileID.Hex()))
		} else {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", session.File.Name))
		}

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return nil
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, session.TotalBytes))
		w.WriteHeader(http.StatusPartialContent)

		for {
			rc.SetWriteDeadline(time.Now().Add(time.Minute * 1))

			n, err := session.WriteToN(w, min(128*1024, length))

			length -= n

			if err != nil {
				return err
			}

			if length == 0 {
				break
			}
		}

		return nil
	})

	if err != nil {

		log.Error().Err(err).Msg("got error")

		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "download session not found")
			return
		}

		if errors.Is(err, storage.ErrInvalidSeek) {
			api.Done(w, http.StatusRequestedRangeNotSatisfiable, "this range can no longer be downloaded")
			return
		}

		if err != io.EOF {

			log.Error().Err(err).Hex("id", fileID[:]).Msg("chunk read error")

			return
		}
	}
}
