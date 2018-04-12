package logmgr

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

func NewLogMgr(baseLogDir string, idleDuration time.Duration) (*LogMgr, error) {
	// write a tmp file to test access
	f, err := os.Create(path.Join(baseLogDir, "test"))
	if err != nil {
		return nil, err
	}
	f.Close()

	err = os.Remove(path.Join(baseLogDir, "test"))
	if err != nil {
		return nil, err
	}

	return &LogMgr{
		BaseLogDir:   baseLogDir,
		IdleDuration: idleDuration,
		appContext:   make(map[string]*AppContext),
	}, nil
}

func (l *LogMgr) handleAppContext(ac *AppContext) {
	fn := path.Join(l.BaseLogDir, ac.id)
	var f *os.File = nil
	var err error

	for {
		select {
		case <-ac.idleTimer.C:
			if f != nil {
				f.Close()
			}
			ac.mutex.Lock()
			ac.idleTimer = nil
			ac.handlerRunning = false
			ac.mutex.Unlock()
			return
		case line := <-ac.inputChan:
			if !ac.idleTimer.Stop() {
				<-ac.idleTimer.C
			}

			if f == nil {
				f, err = os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Println("[ERROR] Cannot open log file, err:", err)
					f = nil
				}
			}

			if f != nil {
				_, err = f.WriteString(line + "\n")
				if err != nil {
					log.Println("[ERROR] Cannot write to log file, err:", err)

					// Close the file now, and we will try again later
					f.Close()
					f = nil
				}
			}

			ac.mutex.Lock()
			for _, c := range ac.outputChans {
				select {
				case c <- line:
				default:
					log.Println("[ERROR] blocking output channel with app", ac.id)
					continue
				}
			}
			ac.mutex.Unlock()
			ac.idleTimer.Reset(l.IdleDuration)
		}
	}
}

func (l *LogMgr) tailFile(appId string, tailLines int, outputChan chan string) error {
	fn := path.Join(l.BaseLogDir, appId)
	_, err := os.Stat(fn)
	if err != nil && os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("/usr/bin/tail", "-n",
		fmt.Sprintf("%d", tailLines), fn)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\n")
			outputChan <- line
		}

		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (l *LogMgr) getAppContext(appId string) *AppContext {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	ac, found := l.appContext[appId]
	if !found {
		ac = &AppContext{
			id:             appId,
			inputChan:      make(chan string, 1024),
			outputChans:    make(map[uint64]chan string),
			idleTimer:      nil,
			handlerRunning: false,
		}

		l.appContext[appId] = ac
	}

	return ac
}

func (l *LogMgr) maybeStartHandler(ac *AppContext) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	if ac.handlerRunning {
		return
	} else {
		ac.handlerRunning = true
		ac.idleTimer = time.NewTimer(l.IdleDuration)
		go l.handleAppContext(ac)
	}
}

func (l *LogMgr) WriteLine(appId string, line string) error {
	if strings.Contains(line, "\n") {
		return errors.New("Line cannot contain \\n")
	}

	ac := l.getAppContext(appId)
	ac.inputChan <- line

	l.maybeStartHandler(ac)
	return nil
}

func (l *LogMgr) GetTailChan(appId string, tailLines int) (chanId uint64, outputChan chan string, err error) {
	outputChan = make(chan string, tailLines*2)
	err = l.tailFile(appId, tailLines, outputChan)
	if err != nil {
		return 0, nil, err
	}

	// Note that there is a chance that some log message is inserted at this
	// point, and is written to file but not propagated to the websockets.
	// This is a race condition we can accept, because otherwise we might back
	// pressure the whole log collection process if one of the websockets is
	// stuck, which is not ideal.
	ac := l.getAppContext(appId)
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	chanId = ac.nextId
	ac.outputChans[chanId] = outputChan
	ac.nextId++

	return chanId, outputChan, nil
}

func (l *LogMgr) CloseChan(appId string, chanId uint64) {
	ac := l.getAppContext(appId)
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	delete(ac.outputChans, chanId)
}
