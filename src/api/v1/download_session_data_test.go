package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"ki/src/config"
	"ki/src/handlers/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// setupChunkedDownload uploads a 100 byte file, sets the chunk size to 16 bytes, and starts a chunked download of it.
func setupChunkedDownload(t *testing.T) (*mux.Router, string, string, []byte) {
	t.Helper()

	oldChunkSize := config.MaxChunkSize()
	config.SetMaxChunkSize(16)
	t.Cleanup(func() { config.SetMaxChunkSize(oldChunkSize) })

	apiv, router, _ := setupAPI(t, "alice")

	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	id, err := apiv.fileStore.SaveFile(storage.FileUpload{
		FStream:          bytes.NewReader(content),
		ExpiresIn:        time.Hour,
		AllowedDownloads: 1,
		Filename:         "test.bin",
	})
	assert.NoError(t, err)

	r := httptest.NewRequest("GET", "/api/dl/s/init/"+id.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		SessionID string `json:"session_id"`
	}
	assert.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	return router, id.Hex(), resp.SessionID, content
}

// getChunk requests the chunk starting at start, like the client does.
func getChunk(router *mux.Router, fileIdHex string, sessionID string, start int64) *httptest.ResponseRecorder {

	r := httptest.NewRequest("GET", "/api/dl/s/data/"+fileIdHex, nil)
	r.Header.Set("Authorization", "Bearer "+sessionID)
	r.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	return w
}

func TestDownloadSessionData_Retry(t *testing.T) {

	assert := assert.New(t)
	router, fileIdHex, sid, content := setupChunkedDownload(t)

	w := getChunk(router, fileIdHex, sid, 0)
	assert.Equal(http.StatusPartialContent, w.Code)
	assert.Equal("bytes 0-15/100", w.Header().Get("Content-Range"))
	assert.Equal(content[0:16], w.Body.Bytes())

	// the response for this chunk is lost on the way to the client
	w = getChunk(router, fileIdHex, sid, 16)
	assert.Equal(http.StatusPartialContent, w.Code)

	w = getChunk(router, fileIdHex, sid, 16)
	assert.Equal(http.StatusPartialContent, w.Code, "retrying the last chunk should be allowed")
	assert.Equal("bytes 16-31/100", w.Header().Get("Content-Range"), "retry should resend the requested range")
	assert.Equal(content[16:32], w.Body.Bytes())

	w = getChunk(router, fileIdHex, sid, 0)
	assert.Equal(http.StatusRequestedRangeNotSatisfiable, w.Code, "rewinding past the last chunk should be refused")

	// the rest of the file, retrying the final chunk
	var rest []byte
	for start := int64(32); start < 100; start += 16 {
		w = getChunk(router, fileIdHex, sid, start)
		assert.Equal(http.StatusPartialContent, w.Code)
		rest = append(rest, w.Body.Bytes()...)
	}
	assert.Equal(content[32:100], rest)

	w = getChunk(router, fileIdHex, sid, 96)
	assert.Equal(http.StatusPartialContent, w.Code, "the final chunk should be retryable until the client says it's done")
	assert.Equal("bytes 96-99/100", w.Header().Get("Content-Range"))
	assert.Equal(content[96:100], w.Body.Bytes())

	// done closes the session
	r := httptest.NewRequest("GET", "/api/dl/s/done/"+fileIdHex, nil)
	r.Header.Set("Authorization", "Bearer "+sid)
	wd := httptest.NewRecorder()
	router.ServeHTTP(wd, r)
	assert.Equal(http.StatusOK, wd.Code)

	w = getChunk(router, fileIdHex, sid, 96)
	assert.Equal(http.StatusNotFound, w.Code)
}
