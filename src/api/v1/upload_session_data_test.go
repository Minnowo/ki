package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"ki/src/config"
	"ki/src/handlers/storage"
	"ki/src/handlers/user"
	"ki/src/pkg/csrf"
	"ki/src/ui/formkeys"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func init() {
	// deriving the master key at full strength for every test database is too slow
	config.SetMasterKeyRounds(1000)
}

// setupAPI creates an APIV1 with a named test user and returns the router plus
// a session-cookie value that authenticates as that user.
func setupAPI(t *testing.T, username string) (*APIV1, *mux.Router, string) {
	t.Helper()

	config.SetFileStorageDir(t.TempDir())
	reg := user.NewRegistry()
	token := reg.NewToken(username)

	apiv := &APIV1{UserRegistry: reg}
	apiv.Init()
	router := mux.NewRouter()
	apiv.Register(router)
	apiv.fileStore.FileDir = t.TempDir()

	return apiv, router, token.String()
}

// buildBeginForm returns a multipart body + Content-Type for a begin request.
func buildBeginForm(t *testing.T, csrfTok string) (*bytes.Buffer, string) {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField(formkeys.CSRF_FORM_FIELD, csrfTok)
	w.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DAYS, "0")
	w.WriteField(formkeys.UPLOAD_FORM_EXPIRE_HOURS, "1")
	w.WriteField(formkeys.UPLOAD_FORM_EXPIRE_MINUTES, "0")
	w.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS, "5")
	w.WriteField(formkeys.UPLOAD_FORM_MEMORY_ONLY, "false")
	w.WriteField(formkeys.UPLOAD_FORM_PASSWORD, "")
	w.WriteField(formkeys.UPLOAD_FORM_FILENAME, "hello.txt")
	w.Close()

	return &buf, w.FormDataContentType()
}

// beginUpload is a test helper that calls POST /api/upload/begin and returns the upload_id.
func beginUpload(t *testing.T, router *mux.Router, sessionTok string) string {
	t.Helper()

	csrfTok := csrf.NewToken()
	body, ct := buildBeginForm(t, csrfTok)

	r := httptest.NewRequest("POST", "/api/ul/s/init", body)
	r.Header.Set("Content-Type", ct)
	r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
	r.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrfTok})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code, "begin should return 200")

	var resp struct {
		UploadID string `json:"upload_id"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	assert.NotEmpty(t, resp.UploadID, "upload_id should not be empty")

	return resp.UploadID
}

// --- Begin ---

func TestFileUploadBegin(t *testing.T) {

	t.Run("valid request returns upload_id and max_chunk_size", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")

		csrfTok := csrf.NewToken()
		body, ct := buildBeginForm(t, csrfTok)

		r := httptest.NewRequest("POST", "/api/ul/s/init", body)
		r.Header.Set("Content-Type", ct)
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		r.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrfTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			UploadID     string `json:"upload_id"`
			MaxChunkSize int64  `json:"max_chunk_size"`
		}
		assert.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.NotEmpty(t, resp.UploadID)
		assert.Greater(t, resp.MaxChunkSize, int64(0))
	})

	t.Run("missing session cookie returns 401", func(t *testing.T) {
		_, router, _ := setupAPI(t, "alice")

		csrfTok := csrf.NewToken()
		body, ct := buildBeginForm(t, csrfTok)

		r := httptest.NewRequest("POST", "/api/ul/s/init", body)
		r.Header.Set("Content-Type", ct)
		r.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrfTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("bad CSRF token returns 403", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")

		body, ct := buildBeginForm(t, csrf.NewToken()) // token doesn't match cookie

		r := httptest.NewRequest("POST", "/api/ul/s/init", body)
		r.Header.Set("Content-Type", ct)
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		r.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrf.NewToken()}) // mismatch

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("zero downloads returns 400", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")

		csrfTok := csrf.NewToken()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField(formkeys.CSRF_FORM_FIELD, csrfTok)
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DAYS, "0")
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_HOURS, "1")
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_MINUTES, "0")
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS, "0") // invalid
		mw.WriteField(formkeys.UPLOAD_FORM_MEMORY_ONLY, "false")
		mw.WriteField(formkeys.UPLOAD_FORM_PASSWORD, "")
		mw.WriteField(formkeys.UPLOAD_FORM_FILENAME, "f.txt")
		mw.Close()

		r := httptest.NewRequest("POST", "/api/ul/s/init", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		r.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrfTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("expiry under 1 minute returns 400", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")

		csrfTok := csrf.NewToken()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField(formkeys.CSRF_FORM_FIELD, csrfTok)
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DAYS, "0")
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_HOURS, "0")
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_MINUTES, "0") // 0+0+0 = expired immediately
		mw.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS, "1")
		mw.WriteField(formkeys.UPLOAD_FORM_MEMORY_ONLY, "false")
		mw.WriteField(formkeys.UPLOAD_FORM_PASSWORD, "")
		mw.WriteField(formkeys.UPLOAD_FORM_FILENAME, "f.txt")
		mw.Close()

		r := httptest.NewRequest("POST", "/api/ul/s/init", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		r.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrfTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// --- Chunk ---

func TestFileUploadChunk(t *testing.T) {

	t.Run("valid chunk returns bytes_received", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		uploadId := beginUpload(t, router, sessionTok)

		data := []byte("chunk payload data")
		r := httptest.NewRequest("POST", "/api/ul/s/data?sid="+uploadId, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/octet-stream")
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			BytesReceived int64 `json:"bytes_received"`
		}
		assert.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.Equal(t, int64(len(data)), resp.BytesReceived)
	})

	t.Run("no auth returns 401", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		uploadId := beginUpload(t, router, sessionTok)

		r := httptest.NewRequest("POST", "/api/ul/s/data?sid="+uploadId, bytes.NewReader([]byte("x")))
		r.Header.Set("Content-Type", "application/octet-stream")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid upload ID returns 400", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")

		r := httptest.NewRequest("POST", "/api/ul/s/data?sid=notvalidhex", bytes.NewReader([]byte("x")))
		r.Header.Set("Content-Type", "application/octet-stream")
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown upload ID returns 404", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		unknownId := strings.Repeat("ab", 16) // valid hex, no session

		r := httptest.NewRequest("POST", "/api/ul/s/data?sid="+unknownId, bytes.NewReader([]byte("x")))
		r.Header.Set("Content-Type", "application/octet-stream")
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- Complete ---

func TestFileUploadComplete(t *testing.T) {

	t.Run("redirects to download page", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		uploadId := beginUpload(t, router, sessionTok)

		// Send one chunk
		r := httptest.NewRequest("POST", "/api/ul/s/data?sid="+uploadId, bytes.NewReader([]byte("content")))
		r.Header.Set("Content-Type", "application/octet-stream")
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		router.ServeHTTP(httptest.NewRecorder(), r)

		// Complete
		r2 := httptest.NewRequest("POST", "/api/ul/s/done?sid="+uploadId, nil)
		r2.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, r2)

		assert.Equal(t, http.StatusSeeOther, w2.Code)
		assert.Contains(t, w2.Header().Get("Location"), "/download/")
	})

	t.Run("js-form header returns 200 with Location", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		uploadId := beginUpload(t, router, sessionTok)

		r := httptest.NewRequest("POST", "/api/ul/s/done?sid="+uploadId, nil)
		r.Header.Set("X-Requested-With", "js-form")
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			FileID string `json:"file_id"`
		}
		assert.Nil(t, json.Unmarshal(w.Body.Bytes(), &resp), "expected valid json response")

		var fileId storage.FileID
		assert.Nil(t, fileId.FromHex(resp.FileID), "expected valid file ID")
	})

	t.Run("unknown upload ID returns 404", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		unknownId := strings.Repeat("cd", 16)

		r := httptest.NewRequest("POST", "/api/ul/s/done?sid="+unknownId, nil)
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- Abort ---

func TestFileUploadAbort(t *testing.T) {

	t.Run("returns 200 and session is gone", func(t *testing.T) {
		apiv, router, sessionTok := setupAPI(t, "alice")
		uploadId := beginUpload(t, router, sessionTok)

		r := httptest.NewRequest("POST", "/api/ul/s/abort?sid="+uploadId, nil)
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		// A subsequent chunk to the same ID should now 404.
		r2 := httptest.NewRequest("POST", "/api/ul/s/abort?sid="+uploadId, bytes.NewReader([]byte("x")))
		r2.Header.Set("Content-Type", "application/octet-stream")
		r2.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		w2 := httptest.NewRecorder()
		_ = apiv
		router.ServeHTTP(w2, r2)
		assert.Equal(t, http.StatusNotFound, w2.Code)
	})

	t.Run("unknown upload ID returns 404", func(t *testing.T) {
		_, router, sessionTok := setupAPI(t, "alice")
		unknownId := strings.Repeat("ef", 16)

		r := httptest.NewRequest("POST", "/api/ul/s/abort?sid="+unknownId, nil)
		r.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- Full HTTP flow ---

func TestChunkedUploadHTTP_FullFlow(t *testing.T) {

	content := []byte("the quick brown fox jumps over the lazy dog")

	for _, jsForm := range []bool{false, true} {
		t.Run("jsForm="+func() string {
			if jsForm {
				return "true"
			}
			return "false"
		}(), func(t *testing.T) {
			assert := assert.New(t)
			_, router, sessionTok := setupAPI(t, "alice")

			// 1. Begin
			csrfTok := csrf.NewToken()
			beginBody, ct := buildBeginForm(t, csrfTok)

			r1 := httptest.NewRequest("POST", "/api/ul/s/init", beginBody)
			r1.Header.Set("Content-Type", ct)
			r1.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
			r1.AddCookie(&http.Cookie{Name: config.CSRF_COOKIE, Value: csrfTok})
			w1 := httptest.NewRecorder()
			router.ServeHTTP(w1, r1)
			assert.Equal(http.StatusOK, w1.Code)

			var beginResp struct {
				UploadID     string `json:"upload_id"`
				MaxChunkSize int64  `json:"max_chunk_size"`
			}
			assert.NoError(json.NewDecoder(w1.Body).Decode(&beginResp))
			uploadId := beginResp.UploadID

			// 2. Two chunks
			half := len(content) / 2
			for _, chunk := range [][]byte{content[:half], content[half:]} {
				rc := httptest.NewRequest("POST", "/api/ul/s/data?sid="+uploadId, bytes.NewReader(chunk))
				rc.Header.Set("Content-Type", "application/octet-stream")
				rc.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
				wc := httptest.NewRecorder()
				router.ServeHTTP(wc, rc)
				assert.Equal(http.StatusOK, wc.Code)
			}

			// 3. Complete
			r3 := httptest.NewRequest("POST", "/api/ul/s/done?sid="+uploadId, nil)
			r3.AddCookie(&http.Cookie{Name: config.SESSION_COOKIE, Value: sessionTok})
			if jsForm {
				r3.Header.Set("X-Requested-With", "js-form")
			}
			w3 := httptest.NewRecorder()
			router.ServeHTTP(w3, r3)

			var location string
			if jsForm {
				assert.Equal(http.StatusOK, w3.Code)
				var resp struct {
					FileID string `json:"file_id"`
				}
				assert.Nil(json.Unmarshal(w3.Body.Bytes(), &resp), "expected valid json response")

				var fileId storage.FileID
				err := fileId.FromHex(resp.FileID)
				assert.Nil(err, "expected valid file ID")

				location = "/download/" + resp.FileID
			} else {
				assert.Equal(http.StatusSeeOther, w3.Code)
				location = w3.Header().Get("Location")
			}
			assert.NotEmpty(location)
			assert.Contains(location, "/download/")

			// 4. Download and verify content
			fileIdHex := strings.TrimPrefix(location, "/download/")
			r4 := httptest.NewRequest("GET", "/api/dl/full/"+fileIdHex, nil)
			w4 := httptest.NewRecorder()
			router.ServeHTTP(w4, r4)
			assert.Equal(http.StatusOK, w4.Code)

			downloaded, err := io.ReadAll(w4.Body)
			assert.NoError(err)
			assert.Equal(content, downloaded)
		})
	}
}
