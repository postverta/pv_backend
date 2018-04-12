package server

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/logmgr"
	"github.com/postverta/pv_backend/model"
	"github.com/postverta/pv_backend/util"
	processproto "github.com/postverta/pv_exec/proto/process"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	gcontext "golang.org/x/net/context"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func CheckOrigin(r *http.Request) bool {
	// FIXME ignore origin check for now
	return true
}

var (
	Upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     CheckOrigin,
	}
)

func HandleAppLogWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the connection to WebSocket
	vars := mux.Vars(r)
	appId := vars["id"]

	conn, err := Upgrader.Upgrade(w, r, http.Header{})
	if err != nil {
		log.Println("[ERROR] Cannot upgrade connection:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	kaConn := util.NewKeepAliveWsConn(conn, 5.0*time.Second, 5.0*time.Second)
	defer kaConn.Close()

	cid, c, err := logmgr.L().GetTailChan(appId, 500)
	if err != nil {
		log.Println("[ERROR] Cannot get log channel:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer logmgr.L().CloseChan(appId, cid)

	aggrDuration := 100 * time.Millisecond
	aggrTimer := time.NewTimer(aggrDuration)

	// Must call ReadMessage so that we process ping and pong messages
	readErrChan := make(chan error, 1)
	go func() {
		for {
			_, _, err := kaConn.ReadMessage()
			if err != nil {
				readErrChan <- err
				return
			}
		}
	}()

	buf := &bytes.Buffer{}
	for {
		select {
		case <-aggrTimer.C:
			if buf.Len() != 0 {
				err = kaConn.WriteMessage(websocket.TextMessage, buf.Bytes())
				if err != nil {
					return
				}
				buf = &bytes.Buffer{}
			}
			aggrTimer.Reset(aggrDuration)
		case line := <-c:
			if !utf8.Valid([]byte(line)) {
				break
			}
			buf.WriteString(line + "\n")
		case <-readErrChan:
			return
		case <-kaConn.InterruptedChan:
			return
		}
	}
}

func HandleAppStateWebSocket(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, http.Header{})
	if err != nil {
		log.Println("[ERROR] Cannot upgrade connection:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	kaConn := util.NewKeepAliveWsConn(conn, 5.0*time.Second, 5.0*time.Second)
	defer kaConn.Close()

	stateChan, chanId := context.GetAppStateChan()
	defer context.RemoveAppStateChan(chanId)

	// Must call ReadMessage so that we process ping and pong messages
	readErrChan := make(chan error, 1)
	go func() {
		for {
			_, _, err := kaConn.ReadMessage()
			if err != nil {
				readErrChan <- err
				return
			}
		}
	}()

	for {
		select {
		case ps := <-stateChan:
			err := kaConn.WriteMessage(websocket.TextMessage, []byte(ps.String()))
			if err != nil {
				return
			}
		case <-readErrChan:
			return
		case <-kaConn.InterruptedChan:
			return
		}
	}
}

func HandleAppLangServerWebSocket(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request) {
	// First try to sync types dependencies. Must do this for legacy
	// workspaces, as otherwise we won't find the root path.
	err := ContextSyncTypes(context)
	if err != nil {
		log.Println("[ERROR] Cannot sync types:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Enable the language server process
	req := &processproto.ConfigureProcessReq{
		ProcessName:   "lang-server",
		Enabled:       true,
		StartCmd:      []string{"/usr/local/bin/javascript-typescript-langserver", "-p", "2089"},
		RunPath:       "/langserver",
		ListeningPort: 2089,
		EnvVars:       make([]*processproto.KeyValuePair, 0),
	}

	_, err = context.GetProcessServiceClient().ConfigureProcess(gcontext.Background(), req)
	if err != nil {
		log.Println("[ERROR] Cannot enable language server process:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Wait for the language server to start
	startTime := time.Now()
	for {
		if time.Now().Sub(startTime) > 10.0*time.Second {
			log.Println("[ERROR] Time out waiting for language server to start")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		req := &processproto.GetProcessStateReq{
			ProcessName: "lang-server",
		}
		resp, err := context.GetProcessServiceClient().GetProcessState(gcontext.Background(), req)
		if err != nil {
			log.Println("[ERROR] Cannot get language server process state:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if resp.ProcessState == processproto.ProcessState_RUNNING {
			break
		}
		<-time.After(100 * time.Millisecond)
	}

	lspConn, err := net.Dial("tcp", context.LspEndpoint)
	if err != nil {
		log.Println("[ERROR] Cannot connect to the language server:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer lspConn.Close()

	// Time to set up the websocket connection and proxy the requests
	conn, err := Upgrader.Upgrade(w, r, http.Header{})
	if err != nil {
		log.Println("[ERROR] Cannot upgrade connection:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	kaConn := util.NewKeepAliveWsConn(conn, 5.0*time.Second, 5.0*time.Second)
	defer kaConn.Close()

	rwErrorChan := make(chan bool, 4)
	// Websocket -> LSP
	go func() {
		for {
			_, msg, err := kaConn.ReadMessage()
			if err != nil {
				rwErrorChan <- true
				return
			}

			_, err = fmt.Fprintf(lspConn, "Content-Length: %d\r\n\r\n", len(msg))
			if err != nil {
				rwErrorChan <- true
				return
			}

			_, err = lspConn.Write(msg)
			if err != nil {
				rwErrorChan <- true
				return
			}
		}
	}()

	// LSP -> Websocket
	go func() {
		reader := bufio.NewReader(lspConn)
		for {
			contentLength := -1
			for {
				header, err := reader.ReadString('\r')
				if err != nil {
					rwErrorChan <- true
					return
				}
				header = strings.TrimSuffix(header, "\r")
				_, err = reader.ReadByte() // pop the "\n"
				if err != nil {
					rwErrorChan <- true
					return
				}

				if len(header) == 0 {
					// This marks the end of the headers
					break
				}

				parts := strings.SplitN(header, ": ", 2)
				if len(parts) < 2 {
					log.Println("[ERROR] Bad message received from LSP")
					rwErrorChan <- true
					return
				}

				if parts[0] == "Content-Length" {
					contentLength, err = strconv.Atoi(parts[1])
					if err != nil {
						log.Println("[ERROR] Bad message Content-Length from LSP:", parts[1])
						rwErrorChan <- true
						return
					}
				}

				// Ignore the other headers
			}

			if contentLength == -1 {
				log.Println("[ERROR] Content-Length header not found from LSP")
				rwErrorChan <- true
				return
			}

			msg := make([]byte, contentLength)
			ptr := msg
			lengthToRead := contentLength
			for {
				n, err := reader.Read(ptr)
				if err != nil {
					rwErrorChan <- true
					return
				}
				lengthToRead -= n
				if lengthToRead == 0 {
					break
				}

				ptr = ptr[n:]
			}

			err = kaConn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				rwErrorChan <- true
				return
			}
		}
	}()

	select {
	case <-rwErrorChan:
		return
	case <-kaConn.InterruptedChan:
		return
	}
}
