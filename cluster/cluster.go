package cluster

import (
	"fmt"
	agentproto "github.com/postverta/pv_agent/proto"
	"github.com/postverta/pv_backend/config"
	execproto "github.com/postverta/pv_exec/proto/exec"
	processproto "github.com/postverta/pv_exec/proto/process"
	worktreeproto "github.com/postverta/pv_exec/proto/worktree"
	gcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"log"
	"time"
)

func (ctx *Context) Refresh() {
	if ctx.Timer.Stop() {
		ctx.Timer.Reset(ctx.Cluster.ContextExpirationTime)
	}
}

func (ctx *Context) GetExecServiceClient() execproto.ExecServiceClient {
	ctx.Refresh()
	return execproto.NewExecServiceClient(ctx.GrpcConn)
}

func (ctx *Context) GetProcessServiceClient() processproto.ProcessServiceClient {
	ctx.Refresh()
	return processproto.NewProcessServiceClient(ctx.GrpcConn)
}

func (ctx *Context) GetWorktreeServiceClient() worktreeproto.WorktreeServiceClient {
	ctx.Refresh()
	return worktreeproto.NewWorktreeServiceClient(ctx.GrpcConn)
}

func (ctx *Context) GetAppEndpoint() string {
	ctx.Refresh()
	return ctx.AppEndpoint
}

func (ctx *Context) GetAppState() processproto.ProcessState {
	ctx.Refresh()
	return ctx.AppState
}

func (ctx *Context) GetAppStateChan() (stateChan chan processproto.ProcessState, chanId uint64) {
	ctx.Refresh()
	stateChan = make(chan processproto.ProcessState, 256)
	ctx.AppStateMutex.Lock()
	defer ctx.AppStateMutex.Unlock()

	chanId = ctx.NextChanId
	ctx.AppStateChan[chanId] = stateChan
	ctx.NextChanId++
	stateChan <- ctx.AppState
	return stateChan, chanId
}

func (ctx *Context) RemoveAppStateChan(chanId uint64) {
	ctx.AppStateMutex.Lock()
	defer ctx.AppStateMutex.Unlock()
	delete(ctx.AppStateChan, chanId)
}

func NewCluster(agents []string, contextExpirationTime time.Duration) (*Cluster, error) {
	c := &Cluster{
		Agents:                agents,
		ContextExpirationTime: contextExpirationTime,

		AgentServiceClient: make(map[string]agentproto.AgentServiceClient),
		AgentNumContexts:   make(map[string]uint),
		AppContext:         make(map[string]*Context),
		AppContextRefCount: make(map[string]uint),
	}

	for _, agent := range agents {
		conn, err := grpc.Dial(agent, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}

		client := agentproto.NewAgentServiceClient(conn)

		// For now, always clean up the containers when we start
		_, err = client.CloseAll(gcontext.Background(), &agentproto.CloseAllReq{})
		if err != nil {
			return nil, err
		}

		c.AgentServiceClient[agent] = client
	}

	return c, nil
}

func (c *Cluster) bestAgent() string {
	// Find the agent with the minimum number of contexts
	minContexts := -1
	bestAgent := ""
	for _, agent := range c.Agents {
		if minContexts == -1 || int(c.AgentNumContexts[agent]) < minContexts {
			minContexts = int(c.AgentNumContexts[agent])
			bestAgent = agent
		}
	}

	if minContexts > 10 {
		log.Println("[WARNING] Crowded server with %d containers", minContexts)
	}

	return bestAgent
}

// Get an existing context. Return null if none exists.
func (c *Cluster) GetExistingContext(appId string) (*Context, func()) {
	c.mutex.Lock()
	if context, found := c.AppContext[appId]; found {
		c.AppContextRefCount[appId]++
		c.mutex.Unlock()

		context.mutex.RLock()
		return context, func() {
			context.mutex.RUnlock()

			c.mutex.Lock()
			c.AppContextRefCount[appId]--
			c.mutex.Unlock()
		}
	} else {
		c.mutex.Unlock()
		return nil, nil
	}
}

// Get the existing context of an app. If none exists, start a new one with the
// given worktree Ids
func (c *Cluster) GetContext(appId string, sourceWorktreeId string, worktreeId string) (*Context, func(), error) {
	c.mutex.Lock()
	if context, found := c.AppContext[appId]; found {
		if context.WorktreeId != worktreeId {
			// This should never happen
			c.mutex.Unlock()
			return nil, nil, fmt.Errorf("Inconsistent worktree IDs for the same app")
		}

		c.AppContextRefCount[appId]++
		c.mutex.Unlock()

		context.mutex.RLock()
		return context, func() {
			context.mutex.RUnlock()

			c.mutex.Lock()
			c.AppContextRefCount[appId]--
			c.mutex.Unlock()
		}, nil
	}

	// Select an agent to open the context
	agent := c.bestAgent()
	client := c.AgentServiceClient[agent]
	c.AgentNumContexts[agent]++

	openReq := &agentproto.OpenContextReq{
		Image: config.ClusterBaseImage(),
		StorageConfig: &agentproto.StorageConfig{
			AccountName: config.AzureAccountName(),
			AccountKey:  config.AzureAccountKey(),
			Container:   "worktree",
		},
		WorktreeId:       worktreeId,
		SourceWorktreeId: sourceWorktreeId,
		MountPoint:       "/app",
		AutosaveInterval: config.WorktreeAutosaveInterval(),
		// TODO: in the future, have a number of "port slots", and allocate a port from the collection when a new process is started.
		Ports: []uint32{8080, 2089},
		Env: []string{
			"PV_APP_ROOT=/app",
			fmt.Sprintf("PV_APP_ID=%s", appId),
			fmt.Sprintf("PV_INTERNAL_API_ENDPOINT=%s", config.InternalApiEndPoint()),
		},
		ExecConfigRoots: []string{
			"/etc/task/common",
			"/etc/task/javascript",
		},
	}

	startTime := time.Now()
	openResp, err := client.OpenContext(gcontext.Background(), openReq)
	log.Printf("[INFO] OpenContext takes %fs\n", time.Since(startTime).Seconds())
	if err != nil {
		c.mutex.Unlock()
		return nil, nil, err
	}

	// This call is non-blocking. Connecting happens in background.
	grpcConn, err := grpc.Dial(openResp.GrpcEndpoint,
		grpc.WithBackoffMaxDelay(time.Millisecond*10),
		grpc.WithInsecure())
	if err != nil {
		c.mutex.Unlock()
		return nil, nil, err
	}

	var appEndpoint string
	var lspEndpoint string
	for _, portEndpoint := range openResp.PortEndpoints {
		if portEndpoint.Port == 8080 {
			appEndpoint = portEndpoint.Endpoint
		} else if portEndpoint.Port == 2089 {
			lspEndpoint = portEndpoint.Endpoint
		} else {
			log.Println("[ERROR] Unknown port mapping")
		}
	}
	context := &Context{
		Id:                      openResp.ContextId,
		Cluster:                 c,
		AppId:                   appId,
		WorktreeId:              worktreeId,
		GrpcEndpoint:            openResp.GrpcEndpoint,
		AppEndpoint:             appEndpoint,
		LspEndpoint:             lspEndpoint,
		GrpcConn:                grpcConn,
		Timer:                   time.NewTimer(c.ContextExpirationTime),
		AppState:                processproto.ProcessState_NOT_RUNNING,
		AppStateChan:            make(map[uint64]chan processproto.ProcessState),
		AppStateTrackerStopChan: make(chan bool, 1),
	}

	// kick off the app state tracker routine
	go func() {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				break
			case <-context.AppStateTrackerStopChan:
				return
			}
			processServiceClient := processproto.NewProcessServiceClient(context.GrpcConn)
			req := &processproto.GetProcessStateReq{
				ProcessName: "app",
			}
			resp, err := processServiceClient.GetProcessState(gcontext.Background(), req)
			if err != nil {
				// ignore the error
			} else {
				context.AppStateMutex.Lock()
				if context.AppState != resp.ProcessState {
					context.AppState = resp.ProcessState
					for _, stateChan := range context.AppStateChan {
						// Always detect whether stateChan is full and
						// the write would be blocking. Otherwise we
						// can deadlock the entire system!
						select {
						case stateChan <- context.AppState:
						default:
							log.Println("AppState channel is full")
						}
					}
				}
				context.AppStateMutex.Unlock()
			}
		}
	}()

	c.AppContext[appId] = context
	c.AppContextRefCount[appId]++
	c.mutex.Unlock()

	go func() {
		for {
			// Wait for the context to expire.
			<-context.Timer.C

			c.mutex.Lock()
			if c.AppContextRefCount[context.AppId] > 0 {
				// The context is still being used. Reset the timer.
				context.Timer.Reset(c.ContextExpirationTime)
				c.mutex.Unlock()
			} else {
				break
			}
		}

		defer c.mutex.Unlock()

		// We should never block at acquiring this lock, because new
		// request for this context will be blocked. To be safe, acquire
		// this lock to ensure no more instance of the context is
		// allocated.
		context.mutex.Lock()

		// Stop the tracker routine
		context.AppStateTrackerStopChan <- true

		err = context.GrpcConn.Close()
		if err != nil {
			log.Println("Cannot close GRPC connection, err:", err)
			// Ignore this error
		}

		closeReq := &agentproto.CloseContextReq{
			ContextId: context.Id,
		}
		_, err = client.CloseContext(gcontext.Background(), closeReq)
		if err != nil {
			if grpc.Code(err) == codes.InvalidArgument {
				// Some leftover state, which is ok
			} else {
				// Something really bad has happened...
				log.Println("[ERROR] Cannot close context", context, "err:", err)

				// For now, we intentionally quit without releasing
				// the context lock. This prevents any future access
				// to the context or the disk. Otherwise, corruption
				// can occur!!
				return
			}
		}

		delete(c.AppContext, context.AppId)
		c.AgentNumContexts[agent]--

		context.mutex.Unlock()
	}()

	context.mutex.RLock()
	return context, func() {
		context.mutex.RUnlock()

		c.mutex.Lock()
		c.AppContextRefCount[appId]--
		c.mutex.Unlock()
	}, nil
}
