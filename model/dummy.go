package model

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Dummy implementation of the client, for development only.
type DummyClient struct {
	UserClient
	Mutex            sync.RWMutex
	IdToAppMap       map[string]*App
	NameToAppMap     map[string]*App
	WorktreeToAppMap map[string]*App

	IdToApiMap map[string]*Api
}

func NewDummyClient() Client {
	dc := &DummyClient{
		IdToAppMap:       make(map[string]*App),
		NameToAppMap:     make(map[string]*App),
		WorktreeToAppMap: make(map[string]*App),

		IdToApiMap: make(map[string]*Api),
	}

	if f, err := os.Open("fake_apis.json"); err == nil {
		dec := json.NewDecoder(f)
		err = dec.Decode(&dc.IdToApiMap)
		if err != nil {
			log.Println("Cannot decode fake_apis.json")
		}
	}

	return dc
}

func (mc *DummyClient) NewApp(app *App) (*App, error) {
	currentTime := time.Now()
	mc.Mutex.RLock()
	name := ""
	for {
		name = NewAppName()
		if app, _ := mc.GetAppByName(name); app == nil {
			break
		}
	}
	app.Id = NewAppId()
	app.Name = name
	app.CreatedTime = currentTime
	app.AccessedTime = currentTime

	// For testing, we put all apps into gallery
	app.Gallery = true

	mc.Mutex.RUnlock()

	mc.Mutex.Lock()
	mc.IdToAppMap[app.Id] = app
	mc.NameToAppMap[app.Name] = app
	mc.WorktreeToAppMap[app.WorktreeId] = app
	mc.Mutex.Unlock()

	return app, nil
}

func (mc *DummyClient) UpdateApp(app *App, fields []string) error {
	// We cannot handle updating name and worktreeId. They must
	// go through the special functions
	for _, field := range fields {
		if field == "Name" || field == "WorktreeId" {
			return fmt.Errorf("Don't directly update name or worktreeId")
		}
	}

	return nil
}

func (mc *DummyClient) UpdateAppName(app *App, newName string) error {
	mc.Mutex.Lock()
	defer mc.Mutex.Unlock()
	if oldApp, found := mc.NameToAppMap[newName]; found && oldApp.Id != app.Id {
		return ErrorDuplicateAttribute("name")
	}
	delete(mc.NameToAppMap, app.Name)
	mc.NameToAppMap[newName] = app
	app.Name = newName
	return nil
}

func (mc *DummyClient) UpdateAppDescription(app *App, newDescription string) error {
	app.Description = newDescription
	return nil
}

func (mc *DummyClient) UpdateAppSourceTimestamp(app *App) error {
	app.SourceTimestamp = time.Now().UnixNano()
	return nil
}

func (mc *DummyClient) UpdateAppRunningTimestamp(app *App) error {
	app.RunningTimestamp = time.Now().UnixNano()
	return nil
}

func (mc *DummyClient) GetApp(id string) (*App, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	return mc.IdToAppMap[id], nil
}

func (mc *DummyClient) GetAppByName(name string) (*App, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	return mc.NameToAppMap[name], nil
}

func (mc *DummyClient) GetAppsByUserId(userId string) ([]*App, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	apps := make([]*App, 0)
	for _, app := range mc.IdToAppMap {
		if app.UserId == userId {
			apps = append(apps, app)
		}
	}

	return apps, nil
}

func (mc *DummyClient) GetAppByWorktreeId(worktreeId string) (*App, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	return mc.WorktreeToAppMap[worktreeId], nil
}

func (mc *DummyClient) DeleteApp(id string) error {
	mc.Mutex.Lock()
	defer mc.Mutex.Unlock()

	app, found := mc.IdToAppMap[id]
	if !found {
		return nil
	}
	delete(mc.IdToAppMap, id)
	delete(mc.NameToAppMap, app.Name)
	return nil
}

func (mc *DummyClient) GetGalleryApps(limit int) ([]*App, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	apps := make([]*App, 0)
	for _, app := range mc.IdToAppMap {
		if app.Gallery {
			apps = append(apps, app)
		}
	}

	SortAppsByCreatedTime(apps)
	if len(apps) > limit {
		apps = apps[:limit]
	}

	return apps, nil
}

func (mc *DummyClient) GetApis() ([]*Api, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	apis := make([]*Api, 0)
	for _, api := range mc.IdToApiMap {
		apis = append(apis, api)
	}

	return apis, nil
}

func (mc *DummyClient) GetApi(id string) (*Api, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	return mc.IdToApiMap[id], nil
}

func (mc *DummyClient) GetApisByIds(ids []string) ([]*Api, error) {
	mc.Mutex.RLock()
	defer mc.Mutex.RUnlock()
	results := []*Api{}
	for _, id := range ids {
		results = append(results, mc.IdToApiMap[id])
	}
	return results, nil
}
