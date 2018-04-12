package model

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"reflect"
	"time"
)

type MongodbClient struct {
	UserClient
	session *mgo.Session
}

func NewMongodbClient(url string) (Client, error) {
	session, err := mgo.Dial(url)
	if err != nil {
		return nil, err
	}
	session.SetSafe(&mgo.Safe{})

	// Ensure a couple of indices
	coll := session.DB("").C("Apps")
	err = coll.EnsureIndex(mgo.Index{
		Key:        []string{"name"},
		Unique:     true,
		Background: false,
	})
	if err != nil {
		return nil, err
	}
	err = coll.EnsureIndex(mgo.Index{
		Key:        []string{"user_id"},
		Unique:     false,
		Background: false,
	})
	if err != nil {
		return nil, err
	}
	err = coll.EnsureIndex(mgo.Index{
		Key:        []string{"worktree_id"},
		Unique:     true,
		Background: false,
	})
	if err != nil {
		return nil, err
	}
	err = coll.EnsureIndex(mgo.Index{
		Key:        []string{"created_time"},
		Background: false,
	})
	if err != nil {
		return nil, err
	}
	err = coll.EnsureIndex(mgo.Index{
		Key:        []string{"last_backup_time"},
		Background: false,
	})
	if err != nil {
		return nil, err
	}

	return &MongodbClient{
		session: session,
	}, nil
}

func (app *App) toBsonMap(fields []string) bson.M {
	appValue := reflect.ValueOf(app).Elem()
	appType := appValue.Type()
	m := bson.M{}
	for _, field := range fields {
		vt, found := appType.FieldByName(field)
		if !found {
			log.Panic("Unexpected field", field)
		}
		v := appValue.FieldByName(field)
		tag := vt.Tag.Get("bson")
		if tag == "" {
			m[field] = v.Interface()
		} else {
			m[tag] = v.Interface()
		}
	}

	return m
}

func (mc *MongodbClient) NewApp(app *App) (*App, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	currentTime := time.Now()

	app.Id = NewAppId()
	app.CreatedTime = currentTime
	app.AccessedTime = currentTime

	for {
		app.Name = NewAppName()
		err := c.Insert(app)
		if err != nil && !mgo.IsDup(err) {
			return nil, err
		} else if err == nil {
			break
		}
	}

	return app, nil
}

func (mc *MongodbClient) UpdateApp(app *App, fields []string) error {
	updateMap := app.toBsonMap(fields)
	change := bson.M{"$set": updateMap}

	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	return c.UpdateId(app.Id, change)
}

func (mc *MongodbClient) UpdateAppName(app *App, newName string) error {
	change := bson.M{"$set": bson.M{"name": newName}}

	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	err := c.UpdateId(app.Id, change)

	if err == nil {
		app.Name = newName
	}

	if mgo.IsDup(err) {
		return ErrorDuplicateAttribute("name")
	} else {
		return err
	}
}

func (mc *MongodbClient) UpdateAppDescription(app *App, newDescription string) error {
	change := bson.M{"$set": bson.M{"description": newDescription}}

	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	err := c.UpdateId(app.Id, change)
	if err == nil {
		app.Description = newDescription
	}

	return err
}

func (mc *MongodbClient) UpdateAppSourceTimestamp(app *App) error {
	change := bson.M{"$currentDate": bson.M{"source_timestamp": bson.M{"$type": "timestamp"}}}
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	err := c.UpdateId(app.Id, change)
	if err != nil {
		return err
	}
	q := c.FindId(app.Id)
	return q.One(app)
}

func (mc *MongodbClient) UpdateAppRunningTimestamp(app *App) error {
	change := bson.M{"$currentDate": bson.M{"running_timestamp": bson.M{"$type": "timestamp"}}}
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	err := c.UpdateId(app.Id, change)
	if err != nil {
		return err
	}
	q := c.FindId(app.Id)
	return q.One(app)
}

func (mc *MongodbClient) GetApp(id string) (*App, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	q := c.FindId(id)
	app := &App{}
	err := q.One(app)
	if err != nil {
		if err == mgo.ErrNotFound {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		return app, nil
	}
}

func (mc *MongodbClient) GetAppByName(name string) (*App, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	app := &App{
		Name: name,
	}
	q := c.Find(app.toBsonMap([]string{"Name"}))
	err := q.One(app)
	if err != nil {
		if err == mgo.ErrNotFound {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		return app, nil
	}
}

func (mc *MongodbClient) GetAppsByUserId(userId string) ([]*App, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	app := &App{
		UserId: userId,
	}
	q := c.Find(app.toBsonMap([]string{"UserId"}))
	apps := []*App{}
	err := q.All(&apps)
	if err != nil {
		return nil, err
	} else {
		return apps, nil
	}
}

func (mc *MongodbClient) GetAppByWorktreeId(worktreeId string) (*App, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	app := &App{
		WorktreeId: worktreeId,
	}
	q := c.Find(app.toBsonMap([]string{"WorktreeId"}))
	err := q.One(app)
	if err != nil {
		if err == mgo.ErrNotFound {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		return app, nil
	}
}

func (mc *MongodbClient) DeleteApp(id string) error {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	return c.RemoveId(id)
}

func (mc *MongodbClient) GetGalleryApps(limit int) ([]*App, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apps")
	q := c.Find(
		bson.M{
			"gallery": true,
		},
	).Sort("-created_time").Limit(limit)

	apps := []*App{}
	err := q.All(&apps)
	if err != nil {
		return nil, err
	} else {
		return apps, nil
	}
}

func (mc *MongodbClient) GetApis() ([]*Api, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apis")
	q := c.Find(nil)
	apis := []*Api{}
	err := q.All(&apis)
	if err != nil {
		return nil, err
	} else {
		return apis, nil
	}
}

func (mc *MongodbClient) GetApi(id string) (*Api, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apis")
	q := c.FindId(id)
	api := &Api{}
	err := q.One(api)
	if err != nil {
		if err == mgo.ErrNotFound {
			return nil, nil
		} else {
			return nil, err
		}
	} else {
		return api, nil
	}
}

func (mc *MongodbClient) GetApisByIds(ids []string) ([]*Api, error) {
	session := mc.session.Copy()
	defer session.Close()
	c := session.DB("").C("Apis")
	q := c.Find(&bson.M{"_id": &bson.M{"$in": ids}})
	apis := []*Api{}
	err := q.All(&apis)
	if err != nil {
		return nil, err
	} else {
		return apis, nil
	}
}
