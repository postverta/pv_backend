package server

import (
	"encoding/json"
	"github.com/postverta/pv_backend/model"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type ByEnvVarKey []interface{}

func (bevk ByEnvVarKey) Len() int      { return len(bevk) }
func (bevk ByEnvVarKey) Swap(i, j int) { bevk[i], bevk[j] = bevk[j], bevk[i] }
func (bevk ByEnvVarKey) Less(i, j int) bool {
	evi := bevk[i].(map[string]interface{})
	evj := bevk[j].(map[string]interface{})
	keyi := strings.ToUpper(evi["key"].(string))
	keyj := strings.ToUpper(evj["key"].(string))
	return strings.Compare(keyi, keyj) < 0
}

func HandleAppEnvVarsGet(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	apis, err := model.C().GetApisByIds(app.ApiIds)
	if err != nil {
		log.Println("[ERROR] Cannot get APIs from database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	systemEnvVarMap := app.GetSystemEnvVarMap()

	keyValueMap := make(map[string]string)
	for _, kv := range app.EnvVars {
		if _, found := systemEnvVarMap[kv.Key]; found {
			// This is only for safety. To make sure we don't have duplicate
			// entries in our response.
			continue
		}
		keyValueMap[kv.Key] = kv.Value
	}

	results := make(map[string]interface{})
	for _, api := range apis {
		if len(api.RequiredEnvVarKeys) == 0 && len(api.OptionalEnvVarKeys) == 0 {
			continue
		}

		keyValues := make([]interface{}, len(api.RequiredEnvVarKeys)+len(api.OptionalEnvVarKeys))
		results[api.Name] = keyValues
		i := 0
		for _, key := range api.RequiredEnvVarKeys {
			value := keyValueMap[key]
			keyValues[i] = map[string]interface{}{
				"key":      key,
				"value":    value,
				"required": true,
			}
			delete(keyValueMap, key)
			i++
		}
		for _, key := range api.OptionalEnvVarKeys {
			value := keyValueMap[key]
			keyValues[i] = map[string]interface{}{
				"key":      key,
				"value":    value,
				"required": false,
			}
			delete(keyValueMap, key)
			i++
		}
		sort.Sort(ByEnvVarKey(keyValues))
	}

	keyValues := make([]interface{}, len(keyValueMap))
	results["_default"] = keyValues
	i := 0
	for k, v := range keyValueMap {
		keyValues[i] = map[string]interface{}{
			"key":      k,
			"value":    v,
			"required": false,
		}
		i++
	}
	sort.Sort(ByEnvVarKey(keyValues))

	keyValues = make([]interface{}, len(systemEnvVarMap))
	results["_system"] = keyValues
	i = 0
	for k, v := range systemEnvVarMap {
		keyValues[i] = map[string]interface{}{
			"key":      k,
			"value":    v,
			"required": false,
		}
		i++
	}
	sort.Sort(ByEnvVarKey(keyValues))

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(results)
	w.Write(buf)
}

func HandleAppEnvVarPost(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	key := vars["name"]

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("[WARNING] Cannot read body:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if matched, _ := regexp.Match(`^[a-zA-Z_]+[a-zA-Z0-9_]*$`, []byte(key)); !matched {
		log.Println("[WARNING] Bad environment variable key")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, found := app.GetSystemEnvVarMap()[key]; found {
		log.Println("[WARNING] Trying to update system env var")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	value := string(content)
	found := false
	for i, kv := range app.EnvVars {
		if kv.Key == key {
			app.EnvVars[i].Value = value
			found = true
			break
		}
	}

	if !found {
		app.EnvVars = append(app.EnvVars, model.KeyValuePair{
			Key:   key,
			Value: value,
		})
	}

	err = model.C().UpdateApp(app, []string{"EnvVars"})
	if err != nil {
		log.Println("[ERROR] Cannot save app to database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = model.C().UpdateAppSourceTimestamp(app)
	if err != nil {
		log.Println("[ERROR] Cannot update source timestamp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}

func HandleAppEnvVarDelete(userId string, app *model.App, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)
	vars := mux.Vars(r)
	key := vars["name"]

	if matched, _ := regexp.Match(`^[a-zA-Z_]+[a-zA-Z0-9_]*$`, []byte(key)); !matched {
		log.Println("[WARNING] Bad environment variable key")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, found := app.GetSystemEnvVarMap()[key]; found {
		log.Println("[WARNING] Trying to delete system env var")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: we shouldn't allow deleting API env vars either.

	found := false
	idx := 0
	for i, kv := range app.EnvVars {
		if kv.Key == key {
			idx = i
			found = true
			break
		}
	}

	if !found {
		// Nothing to delete
		w.WriteHeader(http.StatusOK)
		return
	}

	copy(app.EnvVars[idx:], app.EnvVars[idx+1:])
	app.EnvVars = app.EnvVars[:len(app.EnvVars)-1]

	err := model.C().UpdateApp(app, []string{"EnvVars"})
	if err != nil {
		log.Println("[ERROR] Cannot save app to database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = model.C().UpdateAppSourceTimestamp(app)
	if err != nil {
		log.Println("[ERROR] Cannot update source timestamp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}
