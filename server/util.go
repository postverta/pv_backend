package server

import (
	"encoding/json"
	"github.com/postverta/pv_backend/cluster"
	"github.com/postverta/pv_backend/model"
	execproto "github.com/postverta/pv_exec/proto/exec"
	processproto "github.com/postverta/pv_exec/proto/process"
	gcontext "golang.org/x/net/context"
	"strings"
)

func NeedUpdateSourceTimestamp(filePath string) bool {
	if strings.HasSuffix(filePath, ".md") {
		// md files don't count as source
		return false
	} else {
		return true
	}
}

func GetPackageJsonDescription(packageJson string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(packageJson))
	packageDict := make(map[string]interface{})
	err := dec.Decode(&packageDict)
	if err != nil {
		return "", err
	}

	description, ok := packageDict["description"].(string)
	if !ok {
		return "", nil
	} else {
		return description, nil
	}
}

func GetPackageJsonStartCmd(packageJson string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(packageJson))
	packageDict := make(map[string]interface{})
	err := dec.Decode(&packageDict)
	if err != nil {
		return "", err
	}

	scripts, ok := packageDict["scripts"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	start, ok := scripts["start"].(string)
	if !ok {
		return "", nil
	}

	return start, nil
}

func ContextReadFile(context *cluster.Context, filePath string) (content []byte, err error) {
	req := &execproto.ExecReq{
		TaskName: "file_read",
		KeyValues: []*execproto.ExecReq_KeyValuePair{
			&execproto.ExecReq_KeyValuePair{
				Key:   "FILEPATH",
				Value: filePath,
			},
		},
		WaitForCompletion: true,
	}
	resp, err := context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

func ContextEnableAppProcess(context *cluster.Context, app *model.App) error {
	req := &processproto.ConfigureProcessReq{
		ProcessName:   "app",
		Enabled:       true,
		StartCmd:      []string{"/scripts/log_run", app.StartCmd},
		RunPath:       "/app",
		ListeningPort: 8080,
		EnvVars:       make([]*processproto.KeyValuePair, 0),
	}

	for _, ev := range app.EnvVars {
		req.EnvVars = append(req.EnvVars, &processproto.KeyValuePair{
			Key:   ev.Key,
			Value: ev.Value,
		})
	}

	for k, v := range app.GetSystemEnvVarMap() {
		req.EnvVars = append(req.EnvVars, &processproto.KeyValuePair{
			Key:   k,
			Value: v,
		})
	}

	_, err := context.GetProcessServiceClient().ConfigureProcess(gcontext.Background(), req)
	return err
}

func ContextRestartAppProcess(context *cluster.Context, app *model.App) error {
	req := &processproto.RestartProcessReq{
		ProcessName: "app",
		StartCmd:    []string{"/scripts/log_run", app.StartCmd},
		EnvVars:     make([]*processproto.KeyValuePair, 0),
	}

	for _, ev := range app.EnvVars {
		req.EnvVars = append(req.EnvVars, &processproto.KeyValuePair{
			Key:   ev.Key,
			Value: ev.Value,
		})
	}

	for k, v := range app.GetSystemEnvVarMap() {
		req.EnvVars = append(req.EnvVars, &processproto.KeyValuePair{
			Key:   k,
			Value: v,
		})
	}

	_, err := context.GetProcessServiceClient().RestartProcess(gcontext.Background(), req)
	return err
}

func ContextSyncTypes(context *cluster.Context) (err error) {
	req := &execproto.ExecReq{
		TaskName:          "sync_types",
		KeyValues:         []*execproto.ExecReq_KeyValuePair{},
		WaitForCompletion: true,
	}
	_, err = context.GetExecServiceClient().Exec(gcontext.Background(), req)
	if err != nil {
		return err
	}

	return nil
}
