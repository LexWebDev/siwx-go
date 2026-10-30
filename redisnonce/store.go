// Package redisnonce stores siwx nonces in Redis 6.2 or newer, shared between processes.
package redisnonce

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	siwx "github.com/LexWebDev/siwx-go"
)

const keyPrefix = "siwx:nonce:"

// Store is a siwx.NonceStore shared between processes.
type Store struct {
	client redis.Cmdable
	ttl    time.Duration
}

// New returns a store whose nonces live for ttl.
func New(client redis.Cmdable, ttl time.Duration) *Store {
	return &Store{client: client, ttl: ttl}
}

// Issue creates a nonce.
func (store *Store) Issue(ctx context.Context) (string, error) {
	nonce := rand.Text()

	if err := store.client.Set(ctx, keyPrefix+nonce, "1", store.ttl).Err(); err != nil {
		return "", err
	}

	return nonce, nil
}

// Exists reports whether a nonce is live, without consuming it.
func (store *Store) Exists(ctx context.Context, nonce string) (bool, error) {
	count, err := store.client.Exists(ctx, keyPrefix+nonce).Result()
	if err != nil {
		return false, err
	}

	return count == 1, nil
}

// Consume atomically removes a live nonce through GETDEL or returns siwx.ErrInvalidNonce.
func (store *Store) Consume(ctx context.Context, nonce string) error {
	err := store.client.GetDel(ctx, keyPrefix+nonce).Err()
	if errors.Is(err, redis.Nil) {
		return siwx.ErrInvalidNonce
	}

	return err
}
