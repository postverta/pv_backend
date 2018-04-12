package server

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/postverta/pv_backend/config"
	"github.com/postverta/pv_backend/model"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"regexp"
)

func HandleAppsGet(userId string, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	apps, err := model.C().GetAppsByUserId(userId)
	if err != nil {
		log.Println("[ERROR] Cannot get apps in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	model.SortAppsByAccessedTime(apps)

	output := make([]map[string]interface{}, 0)
	for _, app := range apps {
		output = append(output, app.ToJsonMap())
	}

	buf, _ := json.Marshal(output)
	w.WriteHeader(http.StatusOK)
	w.Write(buf)
}

func HandleAppGet(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	buf, _ := json.Marshal(app.ToJsonMap())
	w.WriteHeader(http.StatusOK)
	w.Write(buf)
}

func HandleAppNamePost(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	type Input struct {
		NewName string `json:"new_name"`
	}
	input := Input{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)

	if err != nil {
		log.Println("[ERROR] Cannot unmarshal input:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Must follow host name requirement (RFC1123)
	if matched, _ := regexp.Match(`^[a-zA-Z0-9]$|^[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]$`, []byte(input.NewName)); !matched {
		log.Println("[WARNING] Bad app name:", input.NewName)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = model.C().UpdateAppName(app, input.NewName)
	if model.IsDuplicateAttributeError(err) {
		// This is a common thing, no need to log anything
		w.WriteHeader(http.StatusConflict)
		return
	} else if err != nil {
		log.Println("[ERROR] Cannot update app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppDescriptionPost(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	type Input struct {
		NewDescription string `json:"new_description"`
	}
	input := Input{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)

	if err != nil {
		log.Println("[ERROR] Cannot unmarshal input:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(input.NewDescription) == 0 {
		log.Println("[WARNING] Description is empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = model.C().UpdateAppDescription(app, input.NewDescription)
	if err != nil {
		log.Println("[ERROR] Cannot update app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppIconPost(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	// We use Cloudinary to manage image files
	// Maybe move this to a shared library some time
	url := config.CloudinaryUploadUrl()
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Because write can block, put it in a separate goroutine
	go func() {
		defer pw.Close()
		defer writer.Close()
		part, err := writer.CreateFormFile("file", "test")
		if err != nil {
			log.Println("[ERROR] Cannot create form field:", err)
			return
		}
		_, err = io.Copy(part, r.Body)
		if err != nil {
			log.Println("[ERROR] Cannot copy file data:", err)
			return
		}
		err = writer.WriteField("upload_preset", config.CloudinaryUploadPreset())
		if err != nil {
			log.Println("[ERROR] Cannot set form field:", err)
			return
		}
	}()

	resp, err := http.Post(url, writer.FormDataContentType(), pr)
	if err != nil {
		log.Println("[ERROR] Cloudinary upload failed:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if resp.StatusCode != 200 {
		log.Println("[ERROR] Cloudinary upload failed, status:", resp.Status)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	result := make(map[string]interface{})
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&result)
	if err != nil {
		log.Println("[ERROR] Cannot parse Cloudinary response:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, found := result["public_id"]; !found {
		log.Println("[ERROR] bad Cloudinary response, no public_id field:", result)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if publicId, ok := result["public_id"].(string); !ok {
		log.Println("[ERROR] bad Cloudinary response, public_id is not string:", result)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		app.Icon = fmt.Sprintf("%s/%s.png", config.CloudinaryDownloadUrl(), publicId)
		err = model.C().UpdateApp(app, []string{"Icon"})
		if err != nil {
			log.Println("[ERROR] Cannot update database:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		buf, _ := json.Marshal(app.ToJsonMap())
		w.Write(buf)
	}
}

func HandleAppDelete(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	// First delete the app in database. This will fail all future requests
	// with regard to this app.
	err := model.C().DeleteApp(app.Id)
	if err != nil {
		log.Println("[ERROR] Cannot delete app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// We don't delete the worktree image immediately. An async sweeping
	// task will clear all dangling images eventually.
	w.WriteHeader(http.StatusOK)
}

func HandleAppAdoptPost(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	if app.UserId == "" {
		app.UserId = userId
		err := model.C().UpdateApp(app, []string{"UserId"})
		if err != nil {
			log.Println("[ERROR] Cannot update app in database:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppAccessGet(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	accessMode := ""
	if app.UserId != userId && app.UserId != "" {
		// Not my application, allow viewer mode
		accessMode = "viewer"
	} else if app.UserId == userId {
		// My own application, or I'm an anonymous user checking out another anonymous app
		accessMode = "owner"
	} else {
		// Similar to above, but I can potentially adopt the anonymous application
		accessMode = "adopter"
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(map[string]interface{}{"mode": accessMode})
	w.Write(buf)
}

func HandleAppNameGet(w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	app, err := model.C().GetAppByName(vars["name"])
	if err != nil {
		log.Println("[ERROR] Cannot load app from database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if app == nil {
		log.Println("[ERROR] Cannot find app with the name", vars["name"])
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}
