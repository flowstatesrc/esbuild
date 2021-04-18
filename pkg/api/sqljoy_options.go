package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/evanw/esbuild/internal/fs"
)

type SQLJoyOptions struct {
	Client BuildOptions
	Server BuildOptions
	Include []string
	Exclude []string
	AccountId string
	AccountSecret string
	Watch bool
	NoSummary bool
	FS fs.FS
}

type OnloadOptionsCallback func(opts* BuildOptions, conf map[string]interface{}, server bool) error

func NewSQLJoyOptions(jsonOpts []byte, onLoadOptions OnloadOptionsCallback, cmd string) (*SQLJoyOptions, error) {
	opts := &SQLJoyOptions{}

	if jsonOpts != nil {
		err := opts.UnmarshalJSON(jsonOpts, onLoadOptions, cmd)
		if err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func (opts *SQLJoyOptions) UnmarshalJSON(jsonOpts []byte, onLoadOptions OnloadOptionsCallback, cmd string) error {
	data := struct {
		Client map[string]interface{} `json:"client"`
		Server map[string]interface{} `json:"server"`
		Watch bool `json:"watch"`
		Color      string `json:"color"`
		ErrorLimit int `json:"errorLimit"`
		LogLevel   string `json:"logLevel"`
		AccountId string `json:"accountId"`
		AccountSecret string `json:"accountSecret"`
		Env map[string]json.RawMessage `json:"environment"`
	}{}

	err := json.Unmarshal(jsonOpts, &data)
	if err != nil {
		return err
	}
	env := map[string]string{}
	if data.Env != nil {
		for k, v := range data.Env {
			env[k] = string(v)
		}
	}
	env["ENV_ACCOUNT_ID"] = `"` + data.AccountId + `"`

	// These defines are removed if not used, and should be kept flat (no objects or arrays)
	// Because esbuild will inline them and they can participate in constant folding
	// We can't change these defines between client and server builds because they re-use the
	// cached code after the defines have been injected. Because of this, ENV_SERVER is
	// handled explicitly by the compiler.
	opts.Client.Define = env
	opts.Server.Define = env

	opts.Watch = data.Watch
	opts.AccountSecret = data.AccountSecret

	var logLevel LogLevel
	switch data.LogLevel {
	case "":
	case "info":
		logLevel = LogLevelInfo
	case "warning":
		logLevel = LogLevelWarning
	case "error":
		logLevel = LogLevelError
	case "silent":
		logLevel = LogLevelSilent
	default:
		return fmt.Errorf("Invalid log level: %q (valid: info, warning, error, silent)", data.LogLevel)
	}

	// Setup client/server specific default options
	opts.Client.LogLevel = logLevel
	opts.Server.LogLevel = logLevel
	opts.Server.External = []string{"sqljoy-runtime"} // don't overwrite this, extend it

	builds := []struct {
		opts *BuildOptions
		conf map[string]interface{}
	}{
		{
			&opts.Client, data.Client,
		},
		{
			&opts.Server, data.Server,
		},
	}

	for i, build := range builds {
		isServer := i != 0
		err = unmarshalBuildOpts(build.opts, build.conf, isServer)
		if err != nil {
			return err
		}
		if onLoadOptions != nil {
			err = onLoadOptions(build.opts, build.conf, isServer)
			if err != nil {
				return err
			}
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	if len(opts.Include) == 0 {
		// Make sure the source directory is included, at the very least
		opts.Include = append(opts.Include, dir)
	}

	return nil
}

func unmarshalBuildOpts(opts *BuildOptions, conf map[string]interface{}, server bool) error {
	// Check disallowed esbuild options
	if conf["inject"] != nil || conf["define"] != nil {
		return errors.New("inject/define is not currently supported, use banner/footer or environment")
	}
	if conf["bundle"] != nil {
		return errors.New("bundle must be true (the default)")
	}
	if conf["platform"] != nil && conf["platform"] != "browser" {
		return errors.New("only browser is supported for platform (the default)")
	}

	// Setup the default build values (common to both client and server - see caller for target specificdefaults)
	opts.Bundle = true
	opts.Charset = CharsetUTF8
	opts.Loader = map[string]Loader{}
	opts.MinifySyntax = true
	opts.MinifyWhitespace = true
	opts.MinifyIdentifiers = true
	if server {
		opts.Outfile = "server.bundle.js"
	} else {
		opts.Outfile = "client.bundle.js"
	}
	opts.Write = true

	// Parse options from the conf map
	if conf["minify"] != nil {
		switch minify := conf["minify"].(type) {
		case bool:
			opts.MinifySyntax = minify
			opts.MinifyWhitespace = minify
			opts.MinifyIdentifiers = minify
		case map[string]interface{}:
			if val, ok := minify["minifySyntax"].(bool); ok {
				opts.MinifySyntax = val
			}
			if val, ok := minify["minifyWhitespace"].(bool); ok {
				opts.MinifyWhitespace = val
			}
			if val, ok := minify["minifyIdentifiers"].(bool); ok {
				opts.MinifyIdentifiers = val
			}
		default:
			return fmt.Errorf("invalid argument type %T for minify", minify)
		}
	}

	switch conf["treeShaking"] {
	case nil:
		fallthrough
	case "ignoreAnnotations":
		opts.TreeShaking = TreeShakingIgnoreAnnotations
	default:
		return fmt.Errorf("invalid argument %v for tree shaking", conf["tree-shaking"])
	}

	if conf["entryPoints"] != nil {
		if arr, ok := conf["entryPoints"].([]interface{}); ok {
			entryPoints, err := toStringSlice(arr)
			if err != nil {
				return fmt.Errorf("entryPoints: %v", err)
			}
			opts.EntryPoints = append(entryPoints, opts.EntryPoints...)
		} else {
			return fmt.Errorf("entryPoints must be an array, got %T", conf["entryPoints"])
		}
	} else if !server {
		return fmt.Errorf("entryPoints is required in the client section of the config")
	}

	if conf["external"] != nil {
		if arr, ok := conf["external"].([]interface{}); ok {
			external, err := toStringSlice(arr)
			if err != nil {
				return fmt.Errorf("external: %v", err)
			}
			opts.External = append(external, opts.External...)
		} else {
			return fmt.Errorf("external must be an array, got %T", conf["external"])
		}
	}

	switch conf["write"] {
	case nil:
	case true:
	case false:
		opts.Write = false
	default:
		return fmt.Errorf("invalid type %T for write", conf["write"])
	}

	switch conf["format"] {
	case "cjs":
		return errors.New("CommonJS output format is not supported")
	case "esm":
		return errors.New("ESM output format is not supported")
	default:
		opts.Format = FormatIIFE
	}

	// TODO setup default targets/engines for client and server

	return nil
}

func toStringSlice(arr []interface{}) ([]string, error) {
	result := make([]string, len(arr))
	var ok bool
	for i := range arr {
		result[i], ok = arr[i].(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got type %T", arr[i])
		}
	}
	return result, nil
}
