package main

import (
	"context"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/config"
	"github.com/postverta/pv_backend/logmgr"
	"github.com/postverta/pv_backend/model"
	sw "github.com/postverta/pv_backend/server"
	"github.com/joho/godotenv"
	"gopkg.in/segmentio/analytics-go.v3"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Printf("Server started")
	godotenv.Load()

	// Set up cluster connection
	err := cluster.InitGlobalCluster(config.ClusterEndpoints(), config.ClusterContextExpirationTime())
	if err != nil {
		log.Fatal("Cannot initialize cluster:", err)
	}

	// Set up database connection
	if os.Getenv("PRODUCTION") != "" {
		err = model.InitGlobalClient(func() (model.Client, error) {
			return model.NewMongodbClient("pv:pv@mongo/postverta")
		})
	} else {
		err = model.InitGlobalClient(func() (model.Client, error) {
			return model.NewDummyClient(), nil
		})
	}
	if err != nil {
		log.Fatal("Cannot initialize database client:", err)
	}

	// Set up log storage
	err = logmgr.InitGlobalLogMgr(config.LogDirectory(), config.LogIdleDuration())
	if err != nil {
		log.Fatal("Cannot initialize log storage:", err)
	}

	router := sw.NewRouter()
	internalRouter := sw.NewInternalRouter()

	// Segment client
	analyticsClient := analytics.New(config.SegmentWriteKey())

	// This is only used to forward http to https for the proxy server
	var proxyHttpsForwardServer *http.Server
	var proxyServer *http.Server
	if os.Getenv("PRODUCTION") != "" {
		proxyServer = &http.Server{
			Addr:    ":443",
			Handler: sw.NewReverseProxyHandler(analyticsClient),
		}
		proxyHttpsForwardServer = &http.Server{
			Addr:    ":80",
			Handler: sw.NewHttpsRedirectHandler(),
		}
	} else {
		proxyServer = &http.Server{
			Addr:    ":80",
			Handler: sw.NewReverseProxyHandler(analyticsClient),
		}
	}

	apiServer := &http.Server{Addr: ":9090", Handler: router}
	internalServer := &http.Server{Addr: ":9091", Handler: internalRouter}

	certFile := "/etc/postverta.cer"
	keyFile := "/etc/postverta.key"

	// Serve reverse proxy
	go func() {
		log.Printf("Start serving reverse proxy")
		var err error
		if os.Getenv("PRODUCTION") != "" {
			err = proxyServer.ListenAndServeTLS(certFile, keyFile)
		} else {
			err = proxyServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Serve http->https redirection server
	if proxyHttpsForwardServer != nil {
		go func() {
			log.Printf("Start serving http->https redirection")
			err := proxyHttpsForwardServer.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				log.Fatal(err)
			}
		}()
	}

	// Server API server
	go func() {
		log.Printf("Start serving API")
		var err error
		if os.Getenv("PRODUCTION") != "" {
			err = apiServer.ListenAndServeTLS(certFile, keyFile)
		} else {
			err = apiServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Server internal API server (no HTTPS)
	go func() {
		log.Printf("Start serving internal API")
		err := internalServer.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down proxy server")
	proxyServer.Shutdown(context.Background())

	if proxyHttpsForwardServer != nil {
		log.Println("Shutting down http->https redirection server")
		proxyHttpsForwardServer.Shutdown(context.Background())
	}

	log.Println("Shutting down API server")
	apiServer.Shutdown(context.Background())

	log.Println("Shutting down internal API server")
	internalServer.Shutdown(context.Background())

	log.Println("Servers shut down gracefully")
}
