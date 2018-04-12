package model

import (
	"github.com/dustinkirkland/golang-petname"
	"github.com/satori/go.uuid"
	"os"
	"sort"
)

func NewAppId() string {
	return uuid.NewV4().String()
}

func NewAppName() string {
	return petname.Generate(2, "-")
}

func (app *App) ToJsonMap() map[string]interface{} {
	return map[string]interface{}{
		"id":                app.Id,
		"user_id":           app.UserId,
		"name":              app.Name,
		"description":       app.Description,
		"icon":              app.Icon,
		"created_time":      app.CreatedTime,
		"accessed_time":     app.AccessedTime,
		"source_timestamp":  app.SourceTimestamp,
		"running_timestamp": app.RunningTimestamp,
	}
}

func appHostSuffix() string {
	if os.Getenv("PRODUCTION") != "" {
		return ".postverta.com"
	} else {
		return ".dev.postverta.io"
	}
}

func (app *App) GetSystemEnvVarMap() map[string]string {
	return map[string]string{
		"APP_ROOT": "/app",
		"APP_NAME": app.Name,
		"APP_HOST": app.Name + appHostSuffix(),
	}
}

func (api *Api) ToJsonMap() map[string]interface{} {
	return map[string]interface{}{
		"id":          api.Id,
		"name":        api.Name,
		"description": api.Description,
		"logo_url":    api.LogoUrl,
		"portal_url":  api.PortalUrl,
		"tags":        api.Tags,
		"snippet":     api.Snippet,
	}
}

type AppsByCreatedTime []*App

func (bct AppsByCreatedTime) Len() int      { return len(bct) }
func (bct AppsByCreatedTime) Swap(i, j int) { bct[i], bct[j] = bct[j], bct[i] }
func (bct AppsByCreatedTime) Less(i, j int) bool {
	return bct[i].CreatedTime.After(bct[j].CreatedTime)
}

func SortAppsByCreatedTime(apps []*App) {
	sort.Sort(AppsByCreatedTime(apps))
}

type AppsByAccessedTime []*App

func (bat AppsByAccessedTime) Len() int      { return len(bat) }
func (bat AppsByAccessedTime) Swap(i, j int) { bat[i], bat[j] = bat[j], bat[i] }
func (bat AppsByAccessedTime) Less(i, j int) bool {
	return bat[i].AccessedTime.After(bat[j].AccessedTime)
}

func SortAppsByAccessedTime(apps []*App) {
	sort.Sort(AppsByAccessedTime(apps))
}
