package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	execproto "github.com/postverta/pv_exec/proto/exec"
	"github.com/satori/go.uuid"
	gcontext "golang.org/x/net/context"
	"log"
	"net/http"
)

func getGithubRepoDescription(githubUser string, githubRepo string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s", githubUser, githubRepo))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Failed to get github repo, error code:%d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	repo := make(map[string]interface{})
	err = dec.Decode(&repo)
	if err != nil {
		return "", err
	}

	description, ok := repo["description"].(string)
	if !ok {
		// This can be normal, when the description is empty
		return "", nil
	}

	return description, nil
}

func getGithubCommit(githubUser string, githubRepo string, branch string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/branches/%s", githubUser, githubRepo, branch))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Failed to get github commit, error code:%d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	commit := make(map[string]interface{})
	err = dec.Decode(&commit)
	if err != nil {
		return "", err
	}

	commit, ok := commit["commit"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Github response doesn't contain a commit object")
	}

	sha, ok := commit["sha"].(string)
	if !ok {
		return "", fmt.Errorf("Github response doesn't contain a sha")
	}

	return sha, nil
}

func getGithubPackageJson(githubUser string, githubRepo string, commit string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/package.json?ref=%s", githubUser, githubRepo, commit))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Failed to get github package.json, error code:%d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	file := make(map[string]interface{})
	err = dec.Decode(&file)
	if err != nil {
		return "", err
	}

	content, ok := file["content"].(string)
	if !ok {
		return "", fmt.Errorf("Github response doesn't contain a content field")
	}

	encoding, ok := file["encoding"].(string)
	if !ok {
		return "", fmt.Errorf("Github response doesn't contain an encoding field")
	}

	if encoding != "base64" {
		return "", fmt.Errorf("Unknown encoding for github file:%s", encoding)
	}

	contentBytes, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", err
	}

	return string(contentBytes), nil
}

func HandleAppsImportPost(userId string, w http.ResponseWriter, r *http.Request) {
	SetCommonHeaders(w, true)

	type Input struct {
		GithubUser string
		GithubRepo string
		Branch     string
	}
	input := Input{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)

	if err != nil {
		log.Println("[WARNING] Cannot unmarshal input:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if input.GithubUser == "" || input.GithubRepo == "" || input.Branch == "" {
		log.Println("[WARNING] Must provide github repo information")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	commit, err := getGithubCommit(input.GithubUser, input.GithubRepo, input.Branch)
	if err != nil {
		log.Println("[WARNING] Cannot get github commit:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	packageJson, err := getGithubPackageJson(input.GithubUser, input.GithubRepo, commit)
	if err != nil {
		log.Println("[WARNING] Cannot get github package.json:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	githubDescription, err := getGithubRepoDescription(input.GithubUser, input.GithubRepo)
	if err != nil {
		log.Println("[WARNING] Cannot get github description:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	packageJsonDescription, err := GetPackageJsonDescription(packageJson)
	if err != nil {
		log.Println("[WARNING] Cannot get package.json description:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// We prefer github's description
	description := githubDescription
	if description == "" {
		description = packageJsonDescription
	}

	startCmd, err := GetPackageJsonStartCmd(packageJson)
	if err != nil {
		log.Println("[WARNING] Cannot get package.json start command:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: for now, we don't allow custom start command. Always use index.js
	// as a starting point
	startCmd = "node index.js"

	// Allocate a new worktree ID
	app := &model.App{
		WorktreeId:  uuid.NewV4().String(),
		Description: description,
		UserId:      userId,
		StartCmd:    startCmd,
		EnvVars:     []model.KeyValuePair{},
		ApiIds:      []string{},
	}

	app, err = model.C().NewApp(app)
	if err != nil {
		log.Println("[ERROR] Cannot create app in database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	context, closeFunc, err := cluster.C().GetContext(app.Id, "", app.WorktreeId)
	if err != nil {
		log.Println("[ERROR] Cannot get context for app", app.Id, "err:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer closeFunc()

	execReq := &execproto.ExecReq{
		TaskName: "github_import",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "GITHUB_USER",
				Value: input.GithubUser,
			},
			&execproto.ExecReq_KeyValuePair{
				Key:   "GITHUB_REPO",
				Value: input.GithubRepo,
			},
			&execproto.ExecReq_KeyValuePair{
				Key:   "GITHUB_BRANCH",
				Value: input.Branch,
			},
			&execproto.ExecReq_KeyValuePair{
				Key:   "GITHUB_COMMIT",
				Value: commit,
			},
		},
		WaitForCompletion: true,
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), execReq)
	if err != nil {
		log.Println("[ERROR] Cannot exec command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Also sync types
	err = ContextSyncTypes(context)
	if err != nil {
		log.Println("[ERROR] Cannot sync types:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.Marshal(app.ToJsonMap())
	w.Write(buf)
}
