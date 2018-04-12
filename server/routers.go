package server

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.Methods("OPTIONS").HandlerFunc(HandleOptions)

	for _, route := range routes {
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

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World!")
}

func HandleOptions(w http.ResponseWriter, r *http.Request) {
	SetCommonOptionsHeaders(w, "GET, POST, DELETE", false)
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/",
		Index,
	},

	Route{
		"AppsGet",
		"GET",
		"/apps",
		CheckAuth(HandleAppsGet, false),
	},

	Route{
		"AppsImportPost",
		"POST",
		"/apps/import",
		CheckAuth(HandleAppsImportPost, true),
	},

	Route{
		"AppsUploadPost",
		"POST",
		"/apps/upload",
		CheckAuth(HandleAppsUploadPost, true),
	},

	Route{
		"AppGet",
		"GET",
		"/app/{id}",
		CheckAuth(CheckApp(HandleAppGet, true, false), true),
	},

	Route{
		"AppNamePost",
		"POST",
		"/app/{id}/name",
		CheckAuth(CheckApp(HandleAppNamePost, false, false), true),
	},

	Route{
		"AppDescriptionPost",
		"POST",
		"/app/{id}/description",
		CheckAuth(CheckApp(HandleAppDescriptionPost, false, false), true),
	},

	Route{
		"AppIconPost",
		"POST",
		"/app/{id}/icon",
		CheckAuth(CheckApp(HandleAppIconPost, false, false), true),
	},

	Route{
		"AppDelete",
		"DELETE",
		"/app/{id}",
		CheckAuth(CheckApp(HandleAppDelete, false, false), true),
	},

	Route{
		"NameGet",
		"GET",
		"/name/{name}",
		HandleNameGet,
	},

	Route{
		"AppAdoptPost",
		"POST",
		"/app/{id}/adopt",
		CheckAuth(CheckApp(HandleAppAdoptPost, false, false), false),
	},

	Route{
		"AppAlivePost",
		"POST",
		"/app/{id}/alive",
		CheckAuth(CheckApp(CheckAppContext(HandleAppAlivePost), true, false), true),
	},

	Route{
		"AppAccessGet",
		"GET",
		"/app/{id}/access",
		CheckAuth(CheckApp(HandleAppAccessGet, true, false), true),
	},

	Route{
		"AppForkPost",
		"POST",
		"/app/{id}/fork",
		CheckAuth(CheckApp(HandleAppForkPost, true, false), true),
	},

	Route{
		"AppFilesGet",
		"GET",
		"/app/{id}/files",
		CheckAuth(CheckApp(CheckAppContext(HandleAppFilesGet), true, false), true),
	},

	Route{
		"AppFileGet",
		"GET",
		"/app/{id}/file/{path}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppFileGet), true, false), true),
	},

	Route{
		"AppFilePost",
		"POST",
		"/app/{id}/file/{path}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppFilePost), false, false), true),
	},

	Route{
		"AppFileMovePost",
		"POST",
		"/app/{id}/file/{path}/move",
		CheckAuth(CheckApp(CheckAppContext(HandleAppFileMovePost), false, false), true),
	},

	Route{
		"AppFileCopyPost",
		"POST",
		"/app/{id}/file/{path}/copy",
		CheckAuth(CheckApp(CheckAppContext(HandleAppFileCopyPost), false, false), true),
	},

	Route{
		"AppFileDelete",
		"DELETE",
		"/app/{id}/file/{path}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppFileDelete), false, false), true),
	},

	Route{
		"AppExportGet",
		"GET",
		"/app/{id}/export",
		CheckAuth(CheckApp(CheckAppContext(HandleAppExportGet), false, false), true),
	},

	Route{
		"AppUpdatePost",
		"POST",
		"/app/{id}/update",
		CheckAuth(CheckApp(CheckAppContext(HandleAppUpdatePost), false, false), true),
	},

	Route{
		"AppEnablePost",
		"POST",
		"/app/{id}/enable",
		CheckAuth(CheckApp(CheckAppContext(HandleAppEnablePost), false, false), true),
	},

	Route{
		"AppPackagesGet",
		"GET",
		"/app/{id}/packages",
		CheckAuth(CheckApp(CheckAppContext(HandleAppPackagesGet), true, false), true),
	},

	Route{
		"AppPackagePost",
		"POST",
		"/app/{id}/package/{name:.*}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppPackagePost), false, false), true),
	},

	Route{
		"AppPackageDelete",
		"DELETE",
		"/app/{id}/package/{name:.*}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppPackageDelete), false, false), true),
	},

	Route{
		"AppApisGet",
		"GET",
		"/app/{id}/apis",
		CheckAuth(CheckApp(CheckAppContext(HandleAppApisGet), true, false), true),
	},

	Route{
		"AppApiPost",
		"POST",
		"/app/{id}/api/{api_id}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppApiPost), false, false), true),
	},

	Route{
		"AppApiDelete",
		"DELETE",
		"/app/{id}/api/{api_id}",
		CheckAuth(CheckApp(CheckAppContext(HandleAppApiDelete), false, false), true),
	},

	Route{
		"AppEnvVarsGet",
		"GET",
		"/app/{id}/env_vars",
		CheckAuth(CheckApp(HandleAppEnvVarsGet, false, false), true),
	},

	Route{
		"AppEnvVarPost",
		"POST",
		"/app/{id}/env_var/{name}",
		CheckAuth(CheckApp(HandleAppEnvVarPost, false, false), true),
	},

	Route{
		"AppEnvVarDelete",
		"DELETE",
		"/app/{id}/env_var/{name}",
		CheckAuth(CheckApp(HandleAppEnvVarDelete, false, false), true),
	},

	Route{
		"GalleryAppsGet",
		"GET",
		"/gallery/apps",
		HandleGalleryAppsGet,
	},

	Route{
		"UserGet",
		"GET",
		"/user/{id}",
		HandleUserGet,
	},

	Route{
		"AppNameGet",
		"GET",
		"/appname/{name}",
		HandleAppNameGet,
	},

	Route{
		"AppLogWebSocket",
		"GET",
		"/app/{id}/log/ws",
		HandleAppLogWebSocket,
	},

	Route{
		"AppStateWebSocket",
		"GET",
		"/app/{id}/state/ws",
		CheckAuth(CheckApp(CheckAppContext(HandleAppStateWebSocket), false, true), true),
	},

	Route{
		"AppLangServerWebSocket",
		"GET",
		"/app/{id}/langserver/ws",
		CheckAuth(CheckApp(CheckAppContext(HandleAppLangServerWebSocket), false, true), true),
	},
}
