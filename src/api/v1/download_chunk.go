package v1

import (
	"errors"
	"io"
	"ki/src/api"
	"ki/src/handlers/storage"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// download_chunk reads the next chunk from an active download session and writes
// it as a binary response. When all bytes have been delivered the session is
// automatically closed; subsequent requests return 404.
func (a *APIV1) download_chunk(w http.ResponseWriter, r *http.Request) {

	var downloadID storage.FileID

	if err := downloadID.FromHex(mux.Vars(r)["downloadId"]); err != nil {
		api.Done(w, http.StatusBadRequest, "invalid download ID")
		return
	}

	n, err := a.fileStore.ReadNextChunk(w, downloadID)

	if err != nil {

		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "download session not found")
			return
		}

		if err != io.EOF {

			log.Error().Err(err).Hex("id", downloadID[:]).Msg("chunk read error")

			return
		}
	}

	log.Debug().Int64("n", n).Hex("id", downloadID[:]).Msg("sent download chunk")
}
