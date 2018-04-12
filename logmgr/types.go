package logmgr

import (
	"sync"
	"time"
)

type AppContext struct {
	id             string
	inputChan      chan string
	outputChans    map[uint64]chan string
	idleTimer      *time.Timer
	handlerRunning bool

	nextId uint64
	mutex  sync.Mutex
}

type LogMgr struct {
	BaseLogDir   string
	IdleDuration time.Duration

	appContext map[string]*AppContext
	mutex      sync.Mutex
}
