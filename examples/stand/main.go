package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
)

const maxBodyBytes = 8192

type verifyRequest struct {
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

func main() {
	store := siwx.NewMemoryNonceStore(nonceTTL(), nil)

	verifier, err := siwx.New(siwx.Config{
		Domains: strings.Split(setting("SIWX_ALLOWED_DOMAINS", "localhost:3000"), ","),
		Nonces:  store,
	})
	if err != nil {
		log.Fatal(err)
	}

	origin := setting("SIWX_ALLOWED_ORIGIN", "http://localhost:3000")
	router := http.NewServeMux()
	router.Handle("/siwx/nonce", withCORS(origin, siwx.NonceHandler(store)))
	router.Handle("/api/siwx/verify", withCORS(origin, verifyHandler(verifier)))

	server := &http.Server{
		Addr:              setting("SIWX_STAND_ADDR", ":8000"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}

func verifyHandler(verifier *siwx.Verifier) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body verifyRequest

		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxBodyBytes)).Decode(&body); err != nil {
			respond(writer, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_payload"})

			return
		}

		session, err := verifier.Verify(request.Context(), body.Message, body.Signature)
		if err != nil {
			respondWithFailure(writer, err)

			return
		}

		log.Printf("verified %s on %s", session.Address, session.ChainID)
		respond(writer, http.StatusOK, map[string]string{
			"address":   session.Address,
			"namespace": session.Namespace,
			"chainId":   session.ChainID,
			"domain":    session.Domain,
			"issuedAt":  session.IssuedAt.UTC().Format(time.RFC3339),
		})
	})
}

func respondWithFailure(writer http.ResponseWriter, err error) {
	var failure *siwx.Error

	if errors.As(err, &failure) {
		log.Printf("rejected: %s", failure.Code)
		respond(writer, http.StatusUnprocessableEntity, map[string]string{"code": failure.Code})

		return
	}

	log.Printf("failed: %v", err)
	writer.WriteHeader(http.StatusInternalServerError)
}

func respond(writer http.ResponseWriter, status int, body map[string]string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)

	if err := json.NewEncoder(writer).Encode(body); err != nil {
		log.Printf("write failed: %v", err)
	}
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", origin)
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)

			return
		}

		next.ServeHTTP(writer, request)
	})
}

func nonceTTL() time.Duration {
	seconds, err := strconv.Atoi(setting("SIWX_NONCE_TTL", "600"))
	if err != nil || seconds <= 0 {
		log.Fatal("SIWX_NONCE_TTL must be a positive number of seconds")
	}

	return time.Duration(seconds) * time.Second
}

func setting(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
