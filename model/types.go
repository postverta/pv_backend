package model

import (
	"fmt"
	"time"
)

type ErrorDuplicateAttribute string

func (ead ErrorDuplicateAttribute) Error() string {
	return fmt.Sprintf("Attribute %s cannot be duplicate", string(ead))
}

func IsDuplicateAttributeError(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(ErrorDuplicateAttribute)
	return ok
}

type KeyValuePair struct {
	Key   string `bson:"key"`
	Value string `bson:"value"`
}

type App struct {
	// Metadata
	Id           string    `bson:"_id"`
	Description  string    `bson:"description"`
	Icon         string    `bson:"icon"`
	UserId       string    `bson:"user_id"`
	Name         string    `bson:"name"`
	WorktreeId   string    `bson:"worktree_id"`
	CreatedTime  time.Time `bson:"created_time"`
	AccessedTime time.Time `bson:"accessed_time"`
	Private      bool      `bson:"private"`

	StartCmd string         `bson:"start_cmd"`
	EnvVars  []KeyValuePair `bson:"env_vars"`

	Gallery bool `bson:"gallery"`

	// APIs enabled for this app
	ApiIds []string `bson:"api_ids"`

	SourceTimestamp  int64 `bson:"source_timestamp"`
	RunningTimestamp int64 `bson:"running_timestamp"`
}

type User struct {
	Id       string `json:"user_id"`
	Name     string `json:"name"`
	Nickname string `json:"nickname"`
	Picture  string `json:"picture"`
}

type Api struct {
	Id                 string   `bson:"_id"`
	Name               string   `bson:"name"`
	LogoUrl            string   `bson:"logo_url"`
	Description        string   `bson:"description"`
	PortalUrl          string   `bson:"portal_url"`
	Tags               []string `bson:"tags"`
	Packages           []string `bson:"packages"`
	RequiredEnvVarKeys []string `bson:"required_env_var_keys"`
	OptionalEnvVarKeys []string `bson:"optional_env_var_keys"`
	Snippet            string   `bson:"snippet"`
}

type Client interface {
	// App functions
	NewApp(app *App) (*App, error)
	UpdateApp(app *App, fields []string) error
	UpdateAppName(app *App, newName string) error
	UpdateAppDescription(app *App, newDescription string) error
	UpdateAppSourceTimestamp(app *App) error
	UpdateAppRunningTimestamp(app *App) error

	GetApp(id string) (*App, error)
	GetAppByName(name string) (*App, error)
	GetAppsByUserId(userId string) ([]*App, error)
	GetAppByWorktreeId(worktreeId string) (*App, error)
	GetGalleryApps(limit int) ([]*App, error)

	DeleteApp(id string) error

	// User functions
	GetUser(id string) (*User, error)

	// Api functions
	GetApis() ([]*Api, error)
	GetApi(id string) (*Api, error)
	GetApisByIds(ids []string) ([]*Api, error)
}
