package integration_tests

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/pkg/api"
)

func build(code map[string]string, modifyOpts func(opts *api.SQLJoyOptions), entryPoints ...string) api.BuildResult {
	if len(entryPoints) == 0 && len(code) == 1 {
		for key := range code {
			entryPoints = append(entryPoints, key)
		}
	}

	ep, _ := json.Marshal(entryPoints)
	optsJSON := []byte(fmt.Sprintf(`{
		"client": {"minify": false, "entryPoints": %s, "external": ["sqljoy"]},
		"server": {"minify": false},
		"logLevel": "info",
		"accountId": "account-id",
		"accountSecret": "keepitsecretkeepitsafe"
	}`, ep))

	opts, err := api.NewSQLJoyOptions(optsJSON, nil, "build")
	if err != nil {
		panic(err.Error())
	}

	opts.Include = nil
	opts.FS = fs.MockFS(code)
	if modifyOpts != nil {
		modifyOpts(opts)
	}

	return api.BuildFlowState(opts)
}

func getOutFile(result *api.BuildResult, suffix string) []byte {
	for _, file := range result.OutputFiles {
		if strings.HasSuffix(file.Path, suffix) {
			return file.Contents
		}
	}
	return nil
}

func getClientWhitelist(result *api.BuildResult) []map[string]interface{} {
	whitelist := getOutFile(result, "client-queries.json")
	if whitelist == nil {
		return nil
	}
	data := []map[string]interface{}{}
	err := json.Unmarshal(whitelist, &data)
	if err != nil {
		panic(err.Error())
	}
	return data
}

func getServerWhitelist(result *api.BuildResult) []map[string]interface{} {
	whitelist := getOutFile(result, "server-queries.json")
	if whitelist == nil {
		return nil
	}
	data := []map[string]interface{}{}
	err := json.Unmarshal(whitelist, &data)
	if err != nil {
		panic(err.Error())
	}
	return data
}
