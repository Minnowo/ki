package v1

import (
	"errors"
	"ki/src/api"
	"ki/src/api/auth"
	"ki/src/config"
	"ki/src/handlers/storage"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// file_upload_chunk appends a raw binary chunk to an in-progress upload session.
// Chunks must be sent sequentially; the AES-CTR cipher state is maintained server-side.
func (a *APIV1) file_upload_chunk(w http.ResponseWriter, r *http.Request) {

	username, ok := auth.GetUser(r)

	if !ok {
		api.Unauthorized(w)
		return
	}

	var uploadId storage.FileID

	if err := uploadId.FromHex(mux.Vars(r)["uploadId"]); err != nil {
		api.Done(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	log.Info().Str("username", username).Hex("g", uploadId[:]).Msg("chunk")

	r.Body = http.MaxBytesReader(w, r.Body, config.MaxChunkSize())
	rc := http.NewResponseController(w)

	timeoutHelper := func(_ int) {
		deadline := time.Now().Add(time.Second * time.Duration(config.UPLOAD_TIMEOUT_PER_READ_SECONDS))
		rc.SetReadDeadline(deadline)
		rc.SetWriteDeadline(deadline)
	}

	n, err := a.fileStore.AppendChunk(uploadId, username, r.Body, config.MaxChunkSize(), timeoutHelper)

	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			api.Done(w, http.StatusNotFound, "upload session not found")
		} else if errors.Is(err, storage.ErrMaxUploadSizeExceeded) {
			api.Done(w, http.StatusRequestEntityTooLarge, "max upload limit exceeded")
		} else {
			log.Error().Err(err).Hex("id", uploadId[:]).Msg("chunk write error")
			api.Done(w, http.StatusInternalServerError, "error writing chunk")
		}
		return
	}

	api.WriteJSON(w, struct {
		BytesReceived int64 `json:"bytes_received"`
	}{
		BytesReceived: n,
	})
}
