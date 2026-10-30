package memorynonce_test

import (
	"context"
	"errors"
	"testing"
	"time"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/internal/noncetest"
	"github.com/LexWebDev/siwx-go/memorynonce"
)

func TestStore(t *testing.T) {
	noncetest.Run(t, func(*testing.T) siwx.NonceStore {
		return memorynonce.New(time.Minute)
	})
}

func TestStoreExpiresNonces(t *testing.T) {
	moment := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := memorynonce.NewWithClock(time.Minute, func() time.Time { return moment })
	nonce, _ := store.Issue(context.Background())

	moment = moment.Add(61 * time.Second)

	if live, _ := store.Exists(context.Background(), nonce); live {
		t.Fatal("expired nonce reported live")
	}
	if err := store.Consume(context.Background(), nonce); !errors.Is(err, siwx.ErrInvalidNonce) {
		t.Fatalf("got %v", err)
	}
}

func TestStoreForgetsExpiredNonces(t *testing.T) {
	moment := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := memorynonce.NewWithClock(time.Minute, func() time.Time { return moment })

	for issued := 0; issued < 100; issued++ {
		_, _ = store.Issue(context.Background())
	}

	moment = moment.Add(61 * time.Second)
	_, _ = store.Issue(context.Background())

	if size := store.Len(); size != 1 {
		t.Fatalf("size: %d", size)
	}
}

func TestStoreKeepsLiveNoncesWhileForgettingExpiredOnes(t *testing.T) {
	moment := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := memorynonce.NewWithClock(time.Minute, func() time.Time { return moment })
	early, _ := store.Issue(context.Background())

	moment = moment.Add(40 * time.Second)
	late, _ := store.Issue(context.Background())

	moment = moment.Add(30 * time.Second)
	_, _ = store.Issue(context.Background())

	if live, _ := store.Exists(context.Background(), early); live {
		t.Fatal("expired nonce still live")
	}
	if live, _ := store.Exists(context.Background(), late); !live {
		t.Fatal("live nonce forgotten")
	}
	if size := store.Len(); size != 2 {
		t.Fatalf("size: %d", size)
	}
}

func TestStoreForgetsAConsumedNonceWithoutDisturbingExpiry(t *testing.T) {
	moment := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := memorynonce.NewWithClock(time.Minute, func() time.Time { return moment })
	consumed, _ := store.Issue(context.Background())
	kept, _ := store.Issue(context.Background())

	_ = store.Consume(context.Background(), consumed)
	moment = moment.Add(61 * time.Second)
	_, _ = store.Issue(context.Background())

	if live, _ := store.Exists(context.Background(), kept); live {
		t.Fatal("expired nonce still live")
	}
	if size := store.Len(); size != 1 {
		t.Fatalf("size: %d", size)
	}
}

func BenchmarkIssueWithManyLiveNonces(b *testing.B) {
	store := memorynonce.New(time.Hour)

	for issued := 0; issued < 200000; issued++ {
		_, _ = store.Issue(context.Background())
	}

	b.ResetTimer()

	for b.Loop() {
		_, _ = store.Issue(context.Background())
	}
}
