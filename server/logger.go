package server

import (
	"log"
	"net/http"
	"strings"
	"time"
)

func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s %s received", r.Method, r.RequestURI, name)

		if !strings.HasSuffix(r.RequestURI, "ws") {
			stopTimerChan := make(chan bool, 1)

			// Start a few timers to log slow requests
			go func() {
				slowTimer := time.NewTimer(time.Second * 10)
				runnawayTimer := time.NewTimer(time.Second * 30)
				for {
					select {
					case <-slowTimer.C:
						log.Printf("[WARNING] %s %s %s is slow", r.Method, r.RequestURI, name)
					case <-runnawayTimer.C:
						log.Printf("[ERROR] %s %s %s might run away", r.Method, r.RequestURI, name)
					case <-stopTimerChan:
						return
					}
				}
			}()

			inner.ServeHTTP(w, r)
			stopTimerChan <- true
		} else {
			inner.ServeHTTP(w, r)
		}

		log.Printf(
			"%s %s %s finished in %s",
			r.Method,
			r.RequestURI,
			name,
			time.Since(start),
		)
	})
}
