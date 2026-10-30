package siwx

var (
	PersonalMessageHash = personalMessageHash
	RecoverAddress      = recoverAddress
)

func (store *MemoryNonceStore) Len() int {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return len(store.nonces)
}
