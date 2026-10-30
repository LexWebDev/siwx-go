// Package noncetest holds the behaviour every siwx.NonceStore must have.
package noncetest

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"

	siwx "github.com/LexWebDev/siwx-go"
)

var eip4361Nonce = regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)

// Run checks a store created by build.
func Run(t *testing.T, build func(t *testing.T) siwx.NonceStore) {
	t.Helper()

	t.Run("issues a nonce that satisfies EIP-4361", func(t *testing.T) {
		nonce, err := build(t).Issue(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if !eip4361Nonce.MatchString(nonce) {
			t.Fatalf("nonce: %s", nonce)
		}
	})

	t.Run("issues unique nonces", func(t *testing.T) {
		store := build(t)
		first, _ := store.Issue(context.Background())
		second, _ := store.Issue(context.Background())

		if first == second {
			t.Fatalf("both are %s", first)
		}
	})

	t.Run("consumes a nonce exactly once", func(t *testing.T) {
		store := build(t)
		nonce, _ := store.Issue(context.Background())

		if err := store.Consume(context.Background(), nonce); err != nil {
			t.Fatal(err)
		}

		if err := store.Consume(context.Background(), nonce); !errors.Is(err, siwx.ErrInvalidNonce) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("rejects a nonce it never issued", func(t *testing.T) {
		if err := build(t).Consume(context.Background(), "neverIssued123"); !errors.Is(err, siwx.ErrInvalidNonce) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("reports a live nonce without consuming it", func(t *testing.T) {
		reportsLiveness(t, build(t))
	})

	t.Run("lets one of many concurrent requests consume a nonce", func(t *testing.T) {
		consumesOnce(t, build(t))
	})
}

func reportsLiveness(t *testing.T, store siwx.NonceStore) {
	t.Helper()

	nonce, _ := store.Issue(context.Background())

	for attempt := 0; attempt < 2; attempt++ {
		if live, err := store.Exists(context.Background(), nonce); err != nil || !live {
			t.Fatalf("live %v, err %v", live, err)
		}
	}

	if live, _ := store.Exists(context.Background(), "neverIssued123"); live {
		t.Fatal("unknown nonce reported live")
	}

	_ = store.Consume(context.Background(), nonce)

	if live, _ := store.Exists(context.Background(), nonce); live {
		t.Fatal("consumed nonce reported live")
	}
}

func consumesOnce(t *testing.T, store siwx.NonceStore) {
	t.Helper()

	for round := 0; round < 50; round++ {
		nonce, _ := store.Issue(context.Background())

		var (
			accepted atomic.Int32
			workers  sync.WaitGroup
		)
		start := make(chan struct{})

		for worker := 0; worker < 8; worker++ {
			workers.Add(1)

			go func() {
				defer workers.Done()
				<-start

				if store.Consume(context.Background(), nonce) == nil {
					accepted.Add(1)
				}
			}()
		}

		close(start)
		workers.Wait()

		if accepted.Load() != 1 {
			t.Fatalf("round %d accepted %d times", round, accepted.Load())
		}
	}
}
