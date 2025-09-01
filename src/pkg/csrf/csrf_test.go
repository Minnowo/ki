// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
// SPDX-FileCopyrightText: © 2025 Minno
//
// SPDX-License-Identifier: AGPL-3.0-only

package csrf

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	cookieStore func() []byte
	noopStore   struct{}
)

func mustHexDecode(s string) []byte {
	res, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return res
}

func (s cookieStore) Load(_ *http.Request, token any) error {
	if t, ok := token.(*[]byte); ok {
		*t = s()
	}
	return nil
}

func (s cookieStore) Save(w http.ResponseWriter, _ *http.Request, token any) error {
	w.Header().Set("x-token", hex.EncodeToString(token.([]byte)))
	return nil
}

func (s noopStore) Load(_ *http.Request, _ any) error {
	return nil
}

func (s noopStore) Save(_ http.ResponseWriter, _ *http.Request, _ any) error {
	return nil
}

var (
	okToken = mustHexDecode("41680df3178004cd56c1810295bc7a79012e6e1afa671621")
	okStore = cookieStore(func() []byte {
		return okToken
	})
)

func testRequest(store cookieStore,
	prepareRequest func(assert *require.Assertions, r *http.Request),
	checkResponse func(assert *require.Assertions, w *httptest.ResponseRecorder, err error),
) func(t *testing.T) {
	return testRequest2(store, prepareRequest, checkResponse, false)
}

func testRequest2(store cookieStore,
	prepareRequest func(assert *require.Assertions, r *http.Request),
	checkResponse func(assert *require.Assertions, w *httptest.ResponseRecorder, err error),
	manualForm bool,
) func(t *testing.T) {
	return func(t *testing.T) {
		assert := require.New(t)
		var innerRequest *http.Request
		mux := http.NewServeMux()
		mux.Handle("/", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			innerRequest = r
		}))

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		var err error

		handler := NewCSRFHandler(
			store,
			WithErrorHandler(func(w http.ResponseWriter, r *http.Request) {
				err = GetError(r)
				w.WriteHeader(412)
			}),
			ManualVerifyFormCSRF(manualForm),
		).Protect(mux)
		handler.ServeHTTP(w, r)
		assert.Equal(200, w.Result().StatusCode)

		prepareRequest(assert, innerRequest)
		r = innerRequest.WithContext(context.Background())
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		checkResponse(assert, w, err)
	}
}

func BenchmarkRenew(b *testing.B) {
	handler := NewCSRFHandler(noopStore{})
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	for b.Loop() {
		_, err := handler.Renew(w, r)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestCSRF(t *testing.T) {
	t.Run("generate token", testRequest(
		cookieStore(func() []byte {
			return nil
		}),
		func(_ *require.Assertions, _ *http.Request) {},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, _ error) {
			assert.Equal(200, w.Result().StatusCode)
			assert.NotEmpty(w.Header().Get("x-token"))
		},
	))

	t.Run("with header token", testRequest(
		okStore,
		func(assert *require.Assertions, r *http.Request) {
			token, err := DecodeToken(Token(r))
			assert.NoError(err)
			assert.Equal(okToken, token)
			r.Method = "POST"
			r.Header.Set("X-CSRF-Token", Token(r))
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, _ error) {
			assert.Equal(200, w.Result().StatusCode)
		},
	))

	t.Run("https with header token", testRequest(
		okStore,
		func(assert *require.Assertions, r *http.Request) {
			token, err := DecodeToken(Token(r))
			assert.NoError(err)
			assert.Equal(okToken, token)
			r.Method = "POST"
			r.URL.Scheme = "https"
			r.URL.Host = "example.net"
			r.Header.Set("Referer", "https://example.net/test")
			r.Header.Set("X-CSRF-Token", Token(r))
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, _ error) {
			assert.Equal(200, w.Result().StatusCode)
		},
	))

	t.Run("ok with form token", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Body = io.NopCloser(strings.NewReader(url.Values{
				FieldName(r): []string{Token(r)},
			}.Encode()))
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, _ error) {
			assert.Equal(200, w.Result().StatusCode)
		},
	))

	t.Run("ok with multipart token", testRequest(
		okStore,
		func(assert *require.Assertions, r *http.Request) {
			body := new(bytes.Buffer)
			mp := multipart.NewWriter(body)
			f, err := mp.CreateFormField(FieldName(r))
			assert.NoError(err)
			_, _ = io.WriteString(f, Token(r))
			_ = mp.Close()

			r.Method = "PATCH"
			r.Header.Set("Content-Type", mp.FormDataContentType())
			r.Body = io.NopCloser(body)
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, _ error) {
			assert.Equal(200, w.Result().StatusCode)
		},
	))

	t.Run("tampered token", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			token := Token(r)
			m, _ := DecodeToken(token)
			m[18] ^= m[18]

			r.Header.Set("X-CSRF-Token", EncodeToken(m))
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "token does not match")
		},
	))

	t.Run("invalid token", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			r.Header.Set("X-CSRF-Token", "🙂test")
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "invalid token")
			assert.ErrorContains(err, "base64")
		},
	))

	t.Run("ko with form token", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Body = io.NopCloser(strings.NewReader(url.Values{
				"foo": []string{"abcd"},
			}.Encode()))
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "invalid token")
		},
	))

	t.Run("ko with multipart token", testRequest(
		okStore,
		func(assert *require.Assertions, r *http.Request) {
			body := new(bytes.Buffer)
			mp := multipart.NewWriter(body)
			f, err := mp.CreateFormField("foo")
			assert.NoError(err)
			_, _ = io.WriteString(f, "abcd")
			_ = mp.Close()

			r.Method = "PATCH"
			r.Header.Set("Content-Type", mp.FormDataContentType())
			r.Body = io.NopCloser(body)
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "invalid token")
		},
	))

	t.Run("https no referer", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			r.URL.Scheme = "https"
			r.URL.Host = "example.net"
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "no referrer")
		},
	))

	t.Run("https origin, no referer", testRequest(
		okStore,
		func(assert *require.Assertions, r *http.Request) {
			token, err := DecodeToken(Token(r))
			assert.NoError(err)
			assert.Equal(okToken, token)
			r.Method = "POST"
			r.URL.Scheme = "https"
			r.URL.Host = "example.net"
			r.Header.Set("Origin", "https://example.net")
			r.Header.Set("X-CSRF-Token", Token(r))
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, _ error) {
			assert.Equal(200, w.Result().StatusCode)
		},
	))

	t.Run("https invalid referer", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			r.URL.Scheme = "https"
			r.URL.Host = "example.net"
			r.Header.Set("Referer", "http://example.org/")
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "referrer http://example.org/ does not match https://example.net/")
		},
	))

	t.Run("http invalid origin", testRequest(
		okStore,
		func(_ *require.Assertions, r *http.Request) {
			r.Method = "POST"
			r.URL.Scheme = "https"
			r.URL.Host = "example.net"
			r.Header.Set("Origin", "http://example.org/")
		},
		func(assert *require.Assertions, w *httptest.ResponseRecorder, err error) {
			assert.Equal(412, w.Result().StatusCode)
			assert.ErrorContains(err, "origin http://example.org/ does not match https://example.net/")
		},
	))
}
