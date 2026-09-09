package util

import "sync"

type SyncMutex struct{ sync.Mutex }
type SyncMu struct{ sync.Mutex }
type SyncWG struct{ sync.WaitGroup }
type SyncRWMutex struct{ sync.RWMutex }
type SyncOnce struct{ sync.Once }

type Mutex = SyncMutex
type WaitGroup = SyncWG
type RWMutex = SyncRWMutex
type Once = SyncOnce
