package siwx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/memorynonce"
)

func TestNonceHandlerReturnsADifferentNonceEachCall(t *testing.T) {
	handler := siwx.NonceHandler(memorynonce.New(time.Minute))
	seen := map[string]bool{}

	for call := 0; call < 2; call++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/siwx/nonce", nil))

		var body struct {
			Nonce string `json:"nonce"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}

		if recorder.Code != http.StatusOK || body.Nonce == "" || seen[body.Nonce] {
			t.Fatalf("status %d, nonce %q", recorder.Code, body.Nonce)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("content type: %s", contentType)
		}

		seen[body.Nonce] = true
	}
}

func TestNonceHandlerRejectsOtherMethods(t *testing.T) {
	recorder := httptest.NewRecorder()

	siwx.NonceHandler(memorynonce.New(time.Minute)).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/siwx/nonce", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestNonceHandlerAnswers500WhenTheStoreFails(t *testing.T) {
	recorder := httptest.NewRecorder()

	siwx.NonceHandler(newSeededStore()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/siwx/nonce", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", recorder.Code)
	}
}
