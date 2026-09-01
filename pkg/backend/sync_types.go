package backend

import "sync"

type syncMutex struct{ sync.Mutex }
type syncWG struct{ sync.WaitGroup }
type syncRWMutex struct{ sync.RWMutex }
