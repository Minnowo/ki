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

	"github.com/rs/zerolog/log"
)

// download_session_data reads the next chunk from an active download session and writes
// it as a binary response. When all bytes have been delivered the session is
// automatically closed; subsequent requests return 404.
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

	err = a.fileStore.WithDownloadSession(sessionID, fileID, func(session *storage.DownloadSession) error {

		start := max(start, session.BytesWritten)

		if start >= session.TotalBytes {
			return storage.ErrInvalidSeek
		}

		length := min(config.MaxChunkSize(), session.TotalBytes-start)

		if stop != -1 {
			length = min(length, stop-start)
		}

		log.Info().Int64("start", start).Int64("stop", stop).Int64("size", length).Msg("chunk download range")

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

		if _, err := session.SeekUntil(start); err != nil {
			return err
		}

		n, err := session.WriteToN(w, length)

		log.Debug().Int64("n", n).Hex("id", fileID[:]).Msg("sent download chunk")

		return err
	})

	if err != nil {

		log.Error().Err(err).Msg("got error")

		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "download session not found")
			return
		}

		if err != io.EOF {

			log.Error().Err(err).Hex("id", fileID[:]).Msg("chunk read error")

			return
		}
	}
}
