package backend

import "sync"

type syncRWMutex struct{ sync.RWMutex }
