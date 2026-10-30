package siwx

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type issuedNonce struct {
	value     string
	expiresAt time.Time
}

// MemoryNonceStore keeps nonces in one process. Use redisnonce for several.
type MemoryNonceStore struct {
	mutex  sync.Mutex
	nonces map[string]time.Time
	queue  []issuedNonce
	ttl    time.Duration
	now    func() time.Time
}

// NewMemoryNonceStore returns a store. A nil clock means time.Now.
func NewMemoryNonceStore(ttl time.Duration, clock func() time.Time) *MemoryNonceStore {
	if clock == nil {
		clock = time.Now
	}

	return &MemoryNonceStore{nonces: map[string]time.Time{}, ttl: ttl, now: clock}
}

// Issue creates a nonce valid for the store TTL.
func (store *MemoryNonceStore) Issue(context.Context) (string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	moment := store.now()
	store.forgetExpired(moment)

	nonce := rand.Text()
	expiresAt := moment.Add(store.ttl)
	store.nonces[nonce] = expiresAt
	store.queue = append(store.queue, issuedNonce{value: nonce, expiresAt: expiresAt})

	return nonce, nil
}

// Exists reports whether a nonce is live, without consuming it.
func (store *MemoryNonceStore) Exists(_ context.Context, nonce string) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return store.live(nonce), nil
}

// Consume atomically removes a live nonce or returns ErrInvalidNonce.
func (store *MemoryNonceStore) Consume(_ context.Context, nonce string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if !store.live(nonce) {
		return ErrInvalidNonce
	}

	delete(store.nonces, nonce)

	return nil
}

func (store *MemoryNonceStore) live(nonce string) bool {
	expiresAt, found := store.nonces[nonce]

	return found && store.now().Before(expiresAt)
}

func (store *MemoryNonceStore) forgetExpired(moment time.Time) {
	for len(store.queue) > 0 && !moment.Before(store.queue[0].expiresAt) {
		delete(store.nonces, store.queue[0].value)
		store.queue = store.queue[1:]
	}
}

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
