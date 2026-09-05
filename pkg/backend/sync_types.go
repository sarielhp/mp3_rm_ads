package backend

import "sync"

type syncWG struct{ sync.WaitGroup }
type syncRWMutex struct{ sync.RWMutex }
