package siwx

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type MemoryNonceStore struct {
	mutex  sync.Mutex
	nonces map[string]time.Time
	ttl    time.Duration
	now    func() time.Time
}

func NewMemoryNonceStore(ttl time.Duration, clock func() time.Time) *MemoryNonceStore {
	if clock == nil {
		clock = time.Now
	}

	return &MemoryNonceStore{nonces: map[string]time.Time{}, ttl: ttl, now: clock}
}

func (store *MemoryNonceStore) Issue(context.Context) (string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	moment := store.now()

	for nonce, expiresAt := range store.nonces {
		if !moment.Before(expiresAt) {
			delete(store.nonces, nonce)
		}
	}

	nonce := rand.Text()
	store.nonces[nonce] = moment.Add(store.ttl)

	return nonce, nil
}

func (store *MemoryNonceStore) Exists(_ context.Context, nonce string) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return store.live(nonce), nil
}

func (store *MemoryNonceStore) Consume(_ context.Context, nonce string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if !store.live(nonce) {
		return ErrInvalidNonce
	}

	delete(store.nonces, nonce)

	return nil
}

func (store *MemoryNonceStore) Len() int {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return len(store.nonces)
}

func (store *MemoryNonceStore) live(nonce string) bool {
	expiresAt, found := store.nonces[nonce]

	return found && store.now().Before(expiresAt)
}

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
