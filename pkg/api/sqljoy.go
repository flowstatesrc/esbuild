package api

import (
	log_ "log"
	"os"
	"path"

	"github.com/evanw/esbuild/internal/cache"
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/graph"
	"github.com/evanw/esbuild/internal/logger"
)

var f, _ = os.Open("/dev/null");
var log = log_.New(f /*os.Stderr*/, "", log_.Lshortfile)

func BuildFlowState(opts *SQLJoyOptions) BuildResult {
	log.Println("configuring build")

	// Lifted from api_impl.go buildImpl
	logOptions := logger.OutputOptions{
		IncludeSource: true,
		MessageLimit:  opts.Client.LogLimit,
		Color:         validateColor(opts.Client.Color),
		LogLevel:      validateLogLevel(opts.Client.LogLevel),
	}
	loggerInstance := logger.NewStderrLog(logOptions)

	var f fs.FS
	var err error
	if opts.FS != nil {
		f = opts.FS
	} else {
		f, err = fs.RealFS(fs.RealFSOptions{
			AbsWorkingDir: opts.Client.AbsWorkingDir,
			WantWatchData: opts.Watch,
		})
		if err != nil {
			loggerInstance.AddError(nil, logger.Loc{}, err.Error())
			return BuildResult{Errors: convertMessagesToPublic(logger.Error, loggerInstance.Done())}
		}
	}

	log.Println("loading plugins")
	oldAbsWorkingDir := opts.Client.AbsWorkingDir
	plugins := loadPlugins(&opts.Client, f, loggerInstance)
	if opts.Client.AbsWorkingDir != oldAbsWorkingDir {
		panic("Mutating \"AbsWorkingDir\" is not allowed")
	}
	caches := cache.MakeCacheSet()

	buildClient := func() BuildResult {
		log.Println("building client bundle")
		value := rebuildImpl(f, opts.Client, caches, plugins, logOptions, loggerInstance, true)
		return value.result
	}

	compiler := NewFlowStateCompiler(opts, logOptions, loggerInstance, f, caches)
	output := make([]OutputFile, 0, 3)

	opts.Client.OnBundleCompile = func(options *config.Options, _ logger.Log, _ fs.FS, files []graph.InputFile, entryPoints []graph.EntryPoint) {
		if len(entryPoints) == 0 {
			panic("no entry point defined")
		}
		log.Println("creating whitelist and client bundle")

		baseDir := path.Dir(files[entryPoints[0].SourceIndex].Source.KeyPath.Text)
		outDir := options.AbsOutputDir
		if outDir == "" {
			outDir = path.Dir(options.AbsOutputFile)
			if outDir == "" {
				outDir = baseDir
			}
		}
		compiler.CompileClient(outDir, baseDir, files)

		if len(compiler.clientWhitelistFile.Contents) > 2 {
			output = append(output, compiler.clientWhitelistFile)
		}
		if len(compiler.serverWhitelistFile.Contents) > 2 {
			output = append(output, compiler.serverWhitelistFile)
		}
	}

	clientResult := buildClient()
	if len(clientResult.Errors) != 0 {
		return clientResult
	}

	serverResult := compiler.CompileServer()

	output = append(output, clientResult.OutputFiles...)
	output = append(output, serverResult.OutputFiles...)
	return BuildResult{
		Errors: serverResult.Errors,
		OutputFiles: output,
	}
}
