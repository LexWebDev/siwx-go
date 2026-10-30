package memorynonce

import "time"

func NewWithClock(ttl time.Duration, clock func() time.Time) *Store {
	store := New(ttl)
	store.now = clock

	return store
}

func (store *Store) Len() int {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return len(store.nonces)
}
