package cluster

import (
	agentproto "github.com/postverta/pv_agent/proto"
	processproto "github.com/postverta/pv_exec/proto/process"
	"google.golang.org/grpc"
	"sync"
	"time"
)

type Context struct {
	Cluster    *Cluster
	AppId      string
	WorktreeId string

	// Received from agent service
	Id           string
	GrpcEndpoint string
	AppEndpoint  string
	LspEndpoint  string
	GrpcConn     *grpc.ClientConn

	// Track app status
	// TODO: move this out of context
	AppState                processproto.ProcessState
	AppStateChan            map[uint64]chan processproto.ProcessState
	NextChanId              uint64
	AppStateMutex           sync.Mutex
	AppStateTrackerStopChan chan bool

	// Expiration timer
	Timer *time.Timer

	mutex sync.RWMutex
}

type Cluster struct {
	Agents []string

	ContextExpirationTime time.Duration

	AgentServiceClient map[string]agentproto.AgentServiceClient
	AgentNumContexts   map[string]uint
	AppContext         map[string]*Context
	AppContextRefCount map[string]uint

	mutex sync.Mutex
}
