package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/redisnonce"
)

var eip4361Nonce = regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)

func stores(t *testing.T) map[string]siwx.NonceStore {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return map[string]siwx.NonceStore{
		"memory": siwx.NewMemoryNonceStore(time.Minute, nil),
		"redis":  redisnonce.New(client, time.Minute),
	}
}

func TestIssuesANonceThatSatisfiesEip4361(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			nonce, err := store.Issue(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if !eip4361Nonce.MatchString(nonce) {
				t.Fatalf("nonce: %s", nonce)
			}
		})
	}
}

func TestIssuesUniqueNonces(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			first, _ := store.Issue(context.Background())
			second, _ := store.Issue(context.Background())

			if first == second {
				t.Fatalf("both are %s", first)
			}
		})
	}
}

func TestConsumesANonceExactlyOnce(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			nonce, _ := store.Issue(context.Background())

			if err := store.Consume(context.Background(), nonce); err != nil {
				t.Fatal(err)
			}

			if err := store.Consume(context.Background(), nonce); !errors.Is(err, siwx.ErrInvalidNonce) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestRejectsANonceItNeverIssued(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			if err := store.Consume(context.Background(), "neverIssued123"); !errors.Is(err, siwx.ErrInvalidNonce) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestReportsWhetherANonceIsLiveWithoutConsumingIt(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
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
		})
	}
}

func TestLetsOnlyOneOfManyConcurrentRequestsConsumeANonce(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
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
		})
	}
}

func TestMemoryStoreExpiresNonces(t *testing.T) {
	moment := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := siwx.NewMemoryNonceStore(time.Minute, func() time.Time { return moment })
	nonce, _ := store.Issue(context.Background())

	moment = moment.Add(61 * time.Second)

	if live, _ := store.Exists(context.Background(), nonce); live {
		t.Fatal("expired nonce reported live")
	}
	if err := store.Consume(context.Background(), nonce); !errors.Is(err, siwx.ErrInvalidNonce) {
		t.Fatalf("got %v", err)
	}
}

func TestMemoryStoreForgetsExpiredNonces(t *testing.T) {
	moment := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store := siwx.NewMemoryNonceStore(time.Minute, func() time.Time { return moment })

	for issued := 0; issued < 100; issued++ {
		_, _ = store.Issue(context.Background())
	}

	moment = moment.Add(61 * time.Second)
	_, _ = store.Issue(context.Background())

	if size := store.Len(); size != 1 {
		t.Fatalf("size: %d", size)
	}
}

func TestRedisStoreExpiresNonces(t *testing.T) {
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

func TestRedisStoreReportsAConnectionFailureAsSuch(t *testing.T) {
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

func TestNonceHandlerReturnsADifferentNonceEachCall(t *testing.T) {
	handler := siwx.NonceHandler(siwx.NewMemoryNonceStore(time.Minute, nil))
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

	siwx.NonceHandler(siwx.NewMemoryNonceStore(time.Minute, nil)).
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
