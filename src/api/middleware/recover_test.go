package middleware

import (
	"ki/src/config"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func init() {
	config.InitLogging()
}

func TestRecover(t *testing.T) {

	r := mux.NewRouter()
	r.Use(Recoverer)

	r.PathPrefix("/").HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("some panic!")
	})

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

}
