// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
// SPDX-FileCopyrightText: © 2025 Minno
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package csrf provides functions to setup CSRF protection.
// It's mostly a modern port of Gorilla CSRF.
// Copyright (c) 2023 The Gorilla Authors. All rights reserved.
// https://github.com/gorilla/csrf
package csrf

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/rs/zerolog/log"
)

const TOKEN_LENGTH = 24 // 192 bits => 32 characters base64 string
const TOKEN_LENGTH_ENC = ((TOKEN_LENGTH + 2) / 3) * 4

type contextKey struct {
	name string
}

var (
	b64             = base64.URLEncoding
	ctxTokenKey     = &contextKey{"token"}
	ctxFieldNameKey = &contextKey{"fieldname"}
	ctxErrorKey     = &contextKey{"error"}
	safeMethods     = []string{"GET", "HEAD", "OPTIONS", "TRACE"}
)

var (
	errSkippedForm = fmt.Errorf("Skipped parsing form, it must be done by the endpoint")
)

// StorageHandler describes a CSRF token store.
type StorageHandler interface {
	Load(r *http.Request, token any) error
	Save(w http.ResponseWriter, r *http.Request, token any) error
}

// Handler provides the HTTP handler for CSRF protection.
type Handler struct {
	store             StorageHandler
	fieldName         string
	headerName        string
	manualVerifyForms bool
	errorHandler      func(w http.ResponseWriter, r *http.Request)
}

// Option describes a functional option for configuring the CSRF handler.
type Option func(*Handler)

// WithFieldName sets the form field's name.
func WithFieldName(name string) Option {
	return func(ch *Handler) {
		ch.fieldName = name
	}
}

// ManualVerifyFormCSRF sets the form should never be parsed by the middleware.
// The csrf token in forms must be checked manually be the endpoint.
func ManualVerifyFormCSRF(manualVerify bool) Option {
	return func(ch *Handler) {
		ch.manualVerifyForms = manualVerify
	}
}

// WithErrorHandler sets a custom error handler for rejected requests.
func WithErrorHandler(h func(w http.ResponseWriter, r *http.Request)) Option {
	return func(ch *Handler) {
		ch.errorHandler = h
	}
}

// NewCSRFHandler returns a [Handler] instance for a given storage handler.
func NewCSRFHandler(cookie StorageHandler, options ...Option) *Handler {
	h := &Handler{
		store:        cookie,
		fieldName:    "__csrf__",
		headerName:   "X-CSRF-Token",
		errorHandler: defaultErrorHandler,
	}

	for _, fn := range options {
		fn(h)
	}

	return h
}

// Protect is the HTTP middleware that provides
// Cross-Site Request Forgery protection.
//
// It securely generates a token that can be embedded in the HTTP response
// (e.g. form field or HTTP header).
// The token is not masked and it's up to any compression middleware to
// implement BREACH mitigations.
// The original token must be stored in a way that makes it innacessible to
// the page's content. The storage must implement [StorageHandler].
// Requests that do not provide a matching token are served with an
// HTTP 403 Forbidden response.
func (h *Handler) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token []byte

		err := h.store.Load(r, &token)
		if err != nil || len(token) != TOKEN_LENGTH {
			// If there was an error or no token at all, generate
			// a new token and save it in the storage (normally in a cookie)
			if token, err = h.Renew(w, r); err != nil {
				log.Error().Err(err).Msg("error renewing csrf token")
				h.sendError(err, w, r)
				return
			}
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxTokenKey, token)
		ctx = context.WithValue(ctx, ctxFieldNameKey, h.fieldName)

		r = r.WithContext(ctx)

		if !slices.Contains(safeMethods, r.Method) {
			var origin *url.URL
			if header := r.Header.Get("origin"); header != "" {
				origin, err = url.Parse(header)
				if err != nil {
					log.Debug().Err(err).Msg("bad origin in csrf")
					h.sendError(fmt.Errorf("%s: %w", "invalid origin", err), w, r)
					return
				}

				if !sameOrigin(r.URL, origin) {
					log.Debug().Err(err).Msg("origin does not match the request")
					h.sendError(fmt.Errorf("origin %s does not match %s", origin.String(), r.URL.String()), w, r)
					return
				}
			}

			if origin == nil && r.URL.Scheme == "https" {
				origin, err = url.Parse(r.Referer())
				if err != nil {
					log.Debug().Err(err).Msg("bad url scheme")
					h.sendError(fmt.Errorf("%s: %w", "invalid referrer", err), w, r)
					return
				}
				if origin.String() == "" {
					h.sendError(errors.New("no referrer"), w, r)
					return
				}

				if !sameOrigin(r.URL, origin) {
					h.sendError(fmt.Errorf("referrer %s does not match %s", origin.String(), r.URL.String()), w, r)
					return
				}
			}

			rToken, err := h.requestToken(r)

			if err != nil {

				if err == errSkippedForm {
					next.ServeHTTP(w, r)
				} else {
					h.sendError(fmt.Errorf("invalid token: %w", err), w, r)
				}
				return
			}

			if len(rToken) != TOKEN_LENGTH {
				h.sendError(errors.New("invalid token"), w, r)
				return
			}

			// Finally, check that tokens match.
			if subtle.ConstantTimeCompare(rToken, token) != 1 {
				h.sendError(errors.New("token does not match"), w, r)
				return
			}
		}

		// Handle request
		next.ServeHTTP(w, r)
	})
}

// Verify checks if the token is valid
func (h *Handler) Verify(r *http.Request, rToken []byte) bool {

	var token []byte

	if err := h.store.Load(r, &token); err != nil {
		return false
	}

	if len(rToken) != TOKEN_LENGTH {
		return false
	}

	if subtle.ConstantTimeCompare(rToken, token) != 1 {
		return false
	}

	return true
}

// Verify checks if the token is valid
func (h *Handler) VerifyStr(r *http.Request, rToken string) bool {

	tokenBytes, err := DecodeToken(rToken)
	if err != nil {
		return false
	}

	return h.Verify(r, tokenBytes)
}

// Renew generates a new token and saves it in the storage (usually a cookie).
func (h *Handler) Renew(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	token := make([]byte, TOKEN_LENGTH)
	rand.Read(token)

	// Save the new token
	if err := h.store.Save(w, r, token); err != nil {
		return nil, err
	}

	return token, nil
}

func (h *Handler) requestToken(r *http.Request) ([]byte, error) {
	// 1. check the header first
	issued := r.Header.Get(h.headerName)

	// 2. fall back to the form value
	// this takes care of multipart or regular forms.
	if issued == "" {

		if h.manualVerifyForms {
			return nil, errSkippedForm
		}

		issued = r.PostFormValue(h.fieldName)
	}

	// return empty when no token was found
	if issued == "" {
		return nil, nil
	}

	// decode the token
	return DecodeToken(issued)
}

func (h *Handler) sendError(err error, w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), ctxErrorKey, err)
	h.errorHandler(w, r.WithContext(ctx))
}

// Token returns a CSRF token ready for passing into HTML template or
// a JSON response body. An empty token will be returned if the middleware
// has not been applied (which will fail subsequent validation).
func Token(r *http.Request) string {
	if t, ok := r.Context().Value(ctxTokenKey).([]byte); ok {
		return EncodeToken(t)
	}
	return ""
}

// EncodeToken encode the given CSRF token as a string
func EncodeToken(token []byte) string {
	return b64.EncodeToString(token)
}

// DecodeToken decodes the given CSRF token
func DecodeToken(token string) ([]byte, error) {
	return b64.DecodeString(token)
}

// FieldName returns the CSRF form field name.
func FieldName(r *http.Request) string {
	if n, ok := r.Context().Value(ctxFieldNameKey).(string); ok {
		return n
	}
	return ""
}

// GetError returns the CSRF error reason from the request's context.
func GetError(r *http.Request) error {
	err, _ := r.Context().Value(ctxErrorKey).(error)
	return err
}

func defaultErrorHandler(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}

func sameOrigin(a, b *url.URL) bool {
	log.Debug().
		Str("a", a.Scheme+"-"+a.Host).
		Str("b", b.Scheme+"-"+b.Host).
		Msg("compare origin")
	return a.Scheme == b.Scheme && a.Host == b.Host
}
