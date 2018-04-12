package server

import (
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	processproto "github.com/postverta/pv_exec/proto/process"
	"github.com/yhat/wsutil"
	"gopkg.in/segmentio/analytics-go.v3"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

type ReverseProxyHandler struct {
	WebSocketReverseProxy *wsutil.ReverseProxy
	ReverseProxy          *httputil.ReverseProxy

	// For event tracking
	analyticsClient analytics.Client
}

func proxyDirector(r *http.Request) {
	r.URL.Host = r.Header.Get("POSTVERTA_APP_ENDPOINT")
	r.Header.Del("POSTVERTA_APP_ENDPOINT")
}

func (h *ReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	parts := strings.Split(host, ".")
	proxyStartTime := time.Now()
	if len(parts) < 2 || parts[len(parts)-2] != "postverta" {
		log.Println("Not an app host name:", host)
		http.NotFound(w, r)
		return
	}

	name := parts[0]
	app, err := model.C().GetAppByName(name)
	if err != nil {
		log.Println("[ERROR] Cannot get app in database:", err)
		http.NotFound(w, r)
		return
	}

	if app == nil {
		log.Println("[WARNING] Cannot find app name:", name)
		http.NotFound(w, r)
		return
	}

	context, closeFunc, err := cluster.C().GetContext(app.Id, app.WorktreeId, app.WorktreeId)
	if err != nil {
		log.Println("[ERROR] Cannot get context for app", app.Id, "err:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer closeFunc()

	// To be sure, we always try to enable the process first
	err = ContextEnableAppProcess(context, app)
	if err != nil {
		log.Println("[ERROR] Cannot enable app:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Wait until we have reached the running state, or timeout
	startTime := time.Now()
	timeoutDuration := 10 * time.Second
	for {
		if context.AppState == processproto.ProcessState_RUNNING {
			// This is ugly: we pass the app endpoint to
			// the director through a new header in request.
			r.Header.Add("POSTVERTA_APP_ENDPOINT", context.GetAppEndpoint())

			// Log the event
			if h.analyticsClient != nil {
				properties := analytics.NewProperties().
					Set("address", r.RemoteAddr).
					Set("app_id", app.Id).
					Set("app_name", app.Name).
					Set("method", r.Method).
					Set("url", r.URL.String()).
					Set("delay", time.Since(proxyStartTime))

				h.analyticsClient.Enqueue(analytics.Track{
					UserId:     "PROXY_USER",
					Event:      "proxy - hit",
					Properties: properties,
				})
			}

			// call reverse proxies
			if wsutil.IsWebSocketRequest(r) {
				r.URL.Scheme = "ws"
				h.WebSocketReverseProxy.ServeHTTP(w, r)
			} else {
				r.URL.Scheme = "http"
				h.ReverseProxy.ServeHTTP(w, r)
			}
			return
		} else if context.AppState == processproto.ProcessState_FINISHED {
			log.Println("[ERROR] Process has stopped")
			// TODO: some special error message?
			http.NotFound(w, r)
			return
		}

		if time.Now().Sub(startTime) > timeoutDuration {
			log.Println("[ERROR] Timeout waiting for the app to become running")
			// TODO: some special error message?
			http.NotFound(w, r)
			return
		}

		<-time.After(10 * time.Millisecond)
	}
}

func NewReverseProxyHandler(analyticsClient analytics.Client) *ReverseProxyHandler {
	return &ReverseProxyHandler{
		WebSocketReverseProxy: &wsutil.ReverseProxy{
			Director: proxyDirector,
		},
		ReverseProxy: &httputil.ReverseProxy{
			Director: proxyDirector,
		},
		analyticsClient: analyticsClient,
	}
}
