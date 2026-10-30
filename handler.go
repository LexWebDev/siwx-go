package siwx

import (
	"encoding/json"
	"net/http"
)

// NonceHandler serves GET requests with {"nonce": "..."}. It has no rate limit,
// and every request stores a nonce, so put a limiter in front of it.
func NonceHandler(store NonceStore) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			writer.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		nonce, err := store.Issue(request.Context())
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"nonce": nonce})
	})
}
