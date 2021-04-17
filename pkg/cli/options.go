package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

func onLoadOptions(opts* api.BuildOptions, conf map[string]interface{}, server bool) error {
	targets, _ := conf["target"].(string)
	if targets != "" {
		var err error
		opts.Target, opts.Engines, err = parseTargets(strings.Split(targets, ","))
		if err != nil {
			return err
		}
	}

	return nil
}

func ParseFlowStateConfig(configFile, cmd string) (*api.FlowStateOptions, error)  {
	optsJSON, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %q: %v", configFile, err)
	}

	configDir := path.Dir(configFile)
	if configDir != "" {
		err = os.Chdir(configDir)
		if err != nil {
			return nil, fmt.Errorf("could not set current working dir: %v", err)
		}
	}

	opts, err := api.NewFlowStateOptions(optsJSON, onLoadOptions, cmd)
	//api.Config = opts (TODO: why don't we do this)
	return opts, err
}
