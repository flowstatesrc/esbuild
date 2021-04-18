package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/evanw/esbuild/internal/logger"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/evanw/esbuild/pkg/cli"
)

func main() {
	args := os.Args[1:]

	cmd := "build"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	configFile := "fsconfig.json"
	if os.Getenv("DEBUG") != "" {
		configFile = "fsconfig.debug.json"
	}

	for i := range args {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--config="):
			configFile = arg[len("--config="):]
		default:
			fmt.Printf("unknown argument %q for %s: \n", arg, cmd)
			os.Exit(1)
		}
	}

	opts, err := cli.ParseFlowStateConfig(configFile, cmd)
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "build":
	case "deploy":
	case "watch":
		opts.Watch = true
	case "version":
		fmt.Println(esbuildVersion)
		return
	}

	if !opts.Watch {
		// Disable GC if we're just running a single build
		debug.SetGCPercent(-1)
	}

	run(opts, cmd)
}

func run(opts *api.SQLJoyOptions, cmd string) {
	start := time.Now()

	result := api.BuildFlowState(opts)
	if len(result.Errors) != 0 {
		os.Exit(1)
	}

	// Do not exit if we're in watch mode
	if opts.Watch {
		<-make(chan bool)
	}

	if !opts.NoSummary {
		api.PrintSummary(logger.OutputOptions{}, result.OutputFiles, start)
	}
}
