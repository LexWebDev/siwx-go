package redisnonce_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/internal/noncetest"
	"github.com/LexWebDev/siwx-go/redisnonce"
)

func TestStore(t *testing.T) {
	noncetest.Run(t, func(t *testing.T) siwx.NonceStore {
		server := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: server.Addr()})
		t.Cleanup(func() { _ = client.Close() })

		return redisnonce.New(client, time.Minute)
	})
}

func TestStoreExpiresNonces(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store := redisnonce.New(client, time.Minute)
	nonce, _ := store.Issue(context.Background())

	server.FastForward(61 * time.Second)

	if err := store.Consume(context.Background(), nonce); !errors.Is(err, siwx.ErrInvalidNonce) {
		t.Fatalf("got %v", err)
	}
}

func TestStoreReportsAConnectionFailureAsSuch(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })

	store := redisnonce.New(client, time.Minute)
	server.Close()

	err := store.Consume(context.Background(), "anyNonce1234")
	if err == nil || errors.Is(err, siwx.ErrInvalidNonce) {
		t.Fatalf("got %v", err)
	}
}
