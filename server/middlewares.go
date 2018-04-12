package server

import (
	"fmt"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/config"
	"github.com/postverta/pv_backend/model"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strings"
)

type HttpHandlerWithUserId func(userId string, w http.ResponseWriter, r *http.Request)

type HttpHandlerWithUserIdAndApp func(userId string, app *model.App, w http.ResponseWriter, r *http.Request)

type HttpHandlerWithContext func(userId string, app *model.App, context *cluster.Context, w http.ResponseWriter, r *http.Request)

func verifyAccessToken(accessToken string) (valid bool, userId string) {
	secret := config.Auth0Secret()
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		log.Println("Cannot parse jwt token, err:", err)
		return false, ""
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		valid = true
		userId = claims["sub"].(string)
	} else {
		valid = false
		userId = ""
	}
	return
}

func CheckAuth(inner HttpHandlerWithUserId, authOptional bool) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		valid := false
		userId := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			authToken := strings.TrimPrefix(authHeader, "Bearer ")
			valid, userId = verifyAccessToken(authToken)
			if !valid {
				SetCommonHeaders(w, true)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else if !authOptional {
			SetCommonHeaders(w, true)
			w.WriteHeader(http.StatusUnauthorized)
		}

		inner(userId, w, r)
	})
}

func CheckApp(inner HttpHandlerWithUserIdAndApp, publicAccess bool, ignoreOwnership bool) HttpHandlerWithUserId {
	return HttpHandlerWithUserId(func(userId string, w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		appId := vars["id"]
		// Serialize all requests to the same app

		app, err := model.C().GetApp(appId)
		if err != nil {
			log.Println("[ERROR] Cannot get app in database:", err)
			SetCommonHeaders(w, true)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if app == nil {
			log.Println("[WARNING] Cannot find app")
			SetCommonHeaders(w, true)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if app.UserId != userId && app.UserId != "" && !ignoreOwnership &&
			(app.Private || !publicAccess) {
			log.Println("[WARNING] User doesn't own the app")
			SetCommonHeaders(w, true)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		inner(userId, app, w, r)
	})
}

func CheckAppContext(inner HttpHandlerWithContext) HttpHandlerWithUserIdAndApp {
	return HttpHandlerWithUserIdAndApp(func(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
		context, closeFunc, err := cluster.C().GetContext(app.Id, app.WorktreeId, app.WorktreeId)
		if err != nil {
			log.Println("[ERROR] Cannot get context for app", app.Id, "err:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer closeFunc()

		inner(userId, app, context, w, r)
	})
}
