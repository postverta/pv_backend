package server

import (
	"github.com/gorilla/mux"
	"net/http"
)

func NewInternalRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.Methods("OPTIONS").HandlerFunc(HandleOptions)

	for _, route := range internalRoutes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}

var internalRoutes = []Route{
	Route{
		"InternalAppLogPost",
		"POST",
		"/internal/app/{id}/log",
		HandleInternalAppLogPost,
	},

	/*
		Route{
			"InternalAppBackup",
			"GET",
			"/internal/app/{id}/backup",
			HandleInternalAppBackupGet,
		},
	*/
}
