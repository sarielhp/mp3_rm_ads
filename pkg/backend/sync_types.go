package backend

import "sync"

type syncRWMutex struct{ sync.RWMutex }
type syncWaitGroup struct{ sync.WaitGroup }
