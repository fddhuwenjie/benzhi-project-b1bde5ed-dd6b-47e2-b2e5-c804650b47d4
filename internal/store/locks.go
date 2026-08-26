package store

import "sync"

type keyedLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (k *keyedLocks) get(key string) *sync.Mutex {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.locks == nil {
		k.locks = map[string]*sync.Mutex{}
	}
	if k.locks[key] == nil {
		k.locks[key] = &sync.Mutex{}
	}
	return k.locks[key]
}
