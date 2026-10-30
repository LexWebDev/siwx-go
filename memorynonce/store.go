// Package memorynonce stores siwx nonces in one process.
package memorynonce

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
)

type issuedNonce struct {
	value     string
	expiresAt time.Time
}

// Store is a siwx.NonceStore for one process. Use redisnonce for several.
type Store struct {
	mutex  sync.Mutex
	nonces map[string]time.Time
	queue  []issuedNonce
	ttl    time.Duration
	now    func() time.Time
}

// New returns a store whose nonces live for ttl.
func New(ttl time.Duration) *Store {
	return &Store{nonces: map[string]time.Time{}, ttl: ttl, now: time.Now}
}

// Issue creates a nonce valid for the store TTL.
func (store *Store) Issue(context.Context) (string, error) {
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
func (store *Store) Exists(_ context.Context, nonce string) (bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return store.live(nonce), nil
}

// Consume atomically removes a live nonce or returns siwx.ErrInvalidNonce.
func (store *Store) Consume(_ context.Context, nonce string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if !store.live(nonce) {
		return siwx.ErrInvalidNonce
	}

	delete(store.nonces, nonce)

	return nil
}

func (store *Store) live(nonce string) bool {
	expiresAt, found := store.nonces[nonce]

	return found && store.now().Before(expiresAt)
}

func (store *Store) forgetExpired(moment time.Time) {
	for len(store.queue) > 0 && !moment.Before(store.queue[0].expiresAt) {
		delete(store.nonces, store.queue[0].value)
		store.queue = store.queue[1:]
	}
}
