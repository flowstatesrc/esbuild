package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/evanw/esbuild/internal/cache"
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/graph"
	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/js_lexer"
	"github.com/evanw/esbuild/internal/logger"
)

var lateBoundParamStringLiteral = []uint16{'_', '_', 'P', 'A', 'R', 'A', 'M', '_'}

type moduleImports struct {
	module string
	imports []importedName
}

type importsByModule []moduleImports

func (a importsByModule) Len() int           { return len(a) }
func (a importsByModule) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a importsByModule) Less(i, j int) bool { return a[i].module < a[j].module }

func writeImports(sb *strings.Builder, functions map[string]importsByName) importsByModule {
	orderedFuncs := make(importsByModule, len(functions))
	i := 0
	for module, imports := range functions {
		orderedFuncs[i] = moduleImports{module: module, imports: imports}
		sort.Sort(imports) // to make it deterministic
		i++
	}
	sort.Sort(orderedFuncs) // to make it deterministic

	i = 0
	for _, mod := range orderedFuncs {
		sb.WriteString("import { ")
		for j := range mod.imports {
			imp := &mod.imports[j]
			if j != 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(imp.name)
			sb.WriteString(" as ")
			imp.alias = fmt.Sprintf("_%d", i)
			sb.WriteString(imp.alias)
			i++
		}
		sb.WriteString(` } from "`)
		sb.WriteString(mod.module)
		sb.WriteString("\";\n")
	}

	return orderedFuncs
}

func createFunctionMapEntryPoint(sb *strings.Builder, functions map[string]importsByName) string {
	// Create an entry point file where we import all the functions
	// Save them into an object where the keys are the hashes, and then export that
	// object as the name "functions".

	orderedFuncs := writeImports(sb, functions)

	sb.WriteString("\nexport const functions = {\n")
	i := 0
	for _, mod := range orderedFuncs {
		for _, imp := range mod.imports {
			if i != 0 {
				sb.WriteString(",\n")
			}
			sb.WriteString("\t\"")
			sb.WriteString(imp.hash)
			sb.WriteString(`": `)
			sb.WriteString(imp.alias)
			i++
		}
	}
	sb.WriteString("\n};\n")
	return sb.String()
}

type importedName struct {
	name string
	hash string
	alias string
}

type importsByName []importedName

func (a importsByName) Len() int           { return len(a) }
func (a importsByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a importsByName) Less(i, j int) bool { return a[i].name < a[j].name }

func newImport(module, name string) importedName {
	var digest [sha256.Size]byte
	h := sha256.New()
	h.Write([]byte(module))
	h.Write([]byte(name))
	sum := h.Sum(digest[:0])

	// Cut off the last 2 bytes of the hash so it's and even 30 bytes = 40 base64 chars
	// sha256 has much more entropy than we need.
	return importedName{
		name: name,
		hash: base64.RawURLEncoding.EncodeToString(sum[:30]),
	}
}

type undoReplaceExpr struct {
	expr *js_ast.Expr
	data js_ast.E
}

type undoReplaceStmt struct {
	stmt *js_ast.Stmt
	data js_ast.S
}

type FlowStateCompiler struct {
	opts            *FlowStateOptions
	logOptions      logger.OutputOptions
	log             logger.Log
	fs              fs.FS
	caches          *cache.CacheSet
	files           []graph.InputFile
	baseDir         string
	outDir          string
	wg              sync.WaitGroup
	analyzers       []*FlowStateAnalyzer
	undoReplaceExpr []undoReplaceExpr
	undoReplaceStmt []undoReplaceStmt
	undoRemoveParts []*js_ast.Part
	serverFile      string
	clientWhitelistFile   OutputFile
	serverWhitelistFile   OutputFile
	debug           bool
}

func NewFlowStateCompiler(opts *FlowStateOptions, logOptions logger.OutputOptions, log logger.Log, fs fs.FS, caches *cache.CacheSet) *FlowStateCompiler {
	return &FlowStateCompiler{
		opts: opts,
		logOptions: logOptions,
		log:         log,
		fs: fs,
		caches: caches,
		wg:          sync.WaitGroup{},
		debug: true,
	}
}

func (c *FlowStateCompiler) CompileClient(outDir, baseDir string, files []graph.InputFile) {
	c.outDir = outDir
	c.baseDir = baseDir
	c.files = files
	analyzers := make([]*FlowStateAnalyzer, len(files))
	for i := range files {
		file := &files[i]
		path := file.Source.KeyPath.Text
		if file.Source.KeyPath.Namespace != "file" && path == "<runtime>" {
			continue // ignore runtime module
		}
		for _, dir := range c.opts.Exclude {
			if strings.HasPrefix(path, dir) {
				continue
			}
		}
		if len(c.opts.Include) != 0 {
			for _, dir := range c.opts.Include {
				if strings.HasPrefix(path, dir) {
					analyzers[i] = NewFlowStateAnalyzer(c, file)
					break
				}
			}
		} else {
			analyzers[i] = NewFlowStateAnalyzer(c, file)
		}
	}

	c.analyzers = analyzers
	c.wg.Add(len(analyzers))
	for _, analyzer := range analyzers {
		if analyzer == nil {
			c.wg.Done()
		} else {
			go c.visitFile(analyzer)
		}
	}

	c.wg.Wait()

	// Now we've identified all of the queries and server calls. We need to:
	//
	// 	- replace query templates with compiled SQL objects that use hashes
	//  - create query whitelist JSON files mapping hashes above to the queries and to validators
	//  - replace server calls with hashes
	//  - map validators to generated functions indexed by query hashes
	//  - create <server> and <validators> virtual modules as entry points for the server bundle

	c.generateOutputs()
}

func (c *FlowStateCompiler) CompileServer() BuildResult {
	for _, undo := range c.undoReplaceExpr {
		undo.expr.Data = undo.data
	}
	for _, undo := range c.undoReplaceStmt {
		undo.stmt.Data = undo.data
	}

	// Undo analyzer.addImports and server function part.ForceRemove
	for _, analyzer := range c.analyzers {
		if analyzer == nil {
			continue
		}
		for _, f := range analyzer.serverFunctions {
			if f.part != nil {
				// TODO unmark this as ForceRemove
			}
		}
	}

	c.opts.Server.Stdin = &StdinOptions{
		Contents: c.serverFile,
		ResolveDir: c.baseDir,
	}

	c.opts.Server.OnBundleCompile = func(options *config.Options, _ logger.Log, _ fs.FS, files []graph.InputFile, entryPoints []graph.EntryPoint) {
		if len(entryPoints) == 0 {
			panic("no entry point defined")
		}
		log.Println("creating server bundle")
	}

	value := rebuildImpl(c.fs, c.opts.Server, c.caches, nil, c.logOptions, c.log, true)
	return value.result
}

func (c *FlowStateCompiler) visitFile(analyzer *FlowStateAnalyzer) {
	log.Printf("scan: %s\n", path.Base(analyzer.file.Source.KeyPath.Text))
	WalkAst(analyzer, analyzer, analyzer.ast)
	c.wg.Done()
}

func (c *FlowStateCompiler) findImportForFunctionIdentifier(analyzer *FlowStateAnalyzer, fIdent *js_ast.Expr, calls map[string]importsByName) (*js_ast.Symbol, importedName) {
	ast := analyzer.ast
	ref, _ := getRefForIdentifierOrPropertyAccess(nil, fIdent)
	symbol := &ast.Symbols[ref.InnerIndex]
	imported, isImport := ast.NamedImports[ref]
	var imp importedName
	if isImport {
		module := ast.ImportRecords[imported.ImportRecordIndex].Path.Text
		imp = newImport(module, imported.Alias)
		calls[module] = append(calls[module], imp)
	} else {
		// Must be a local function
		if f := analyzer.serverFunctions[ref]; f != nil {
			if f.fnStmt != nil {
				if !f.fnStmt.IsExport {
					c.log.AddError(&analyzer.file.Source, fIdent.Loc, fmt.Sprintf("function %s must be exported", symbol.OriginalName))
				}
			}	else if f.local != nil {
				if !f.local.IsExport {
					c.log.AddError(&analyzer.file.Source, fIdent.Loc, fmt.Sprintf("function %s must be exported", symbol.OriginalName))
				}
			} else {
				panic("serverFunction instance missing required field")
			}
			// visitor.file.Source.KeyPath.Text
			relPath, ok := c.fs.Rel(c.baseDir, analyzer.file.Source.KeyPath.Text)
			if ok {
				relPath = "./" + relPath
			} else {
				relPath = analyzer.file.Source.KeyPath.Text
			}
			imp = newImport(relPath, symbol.OriginalName)
			calls[relPath] = append(calls[relPath], imp)
		} else {
			c.log.AddError(&analyzer.file.Source, fIdent.Loc, fmt.Sprintf("server call %s must refer to a top level exportable function", symbol.OriginalName))
			return nil, importedName{}
		}
	}
	return symbol, imp
}

func (c *FlowStateCompiler) generateServerFile(validators map[string]importsByName, validatorsByQuery map[string][]string) {
	var validatorsReady sync.WaitGroup
	validatorsReady.Add(1)
	var sb *strings.Builder
	go func() {
		sb = c.writeValidators(validators, validatorsByQuery)
		validatorsReady.Done()
	}()

	calls := map[string]importsByName{}

	for _, visitor := range c.analyzers {
		if visitor == nil {
			continue
		}

		for _, serverCall := range visitor.serverCalls {
			symbol, imp := c.findImportForFunctionIdentifier(visitor, &serverCall.call.Target, calls)
			if symbol == nil {
				continue
			}

			// Transform the call from foo(xxx.beginTx(), ...args) to xxx.serverCall("hash", ...args)
			newTarget := *serverCall.fsInstance
			newTarget.Name = serverCallMethodName
			newCall := &js_ast.ECall{
				Target:                 js_ast.Expr{
					Data: &newTarget,
				},
				Args:                   append([]js_ast.Expr{
					// Replace the target function with a string literal containing the base64 encoded hash of the (module, name)
					{Data: &js_ast.EString{
						Value:          js_lexer.StringToUTF16(imp.hash),
					}}},
					serverCall.call.Args[1:]...),
			}

			log.Printf("replacing server call %s, alias %s\n", imp.name, imp.alias)
			c.replaceExpr(serverCall.parent, serverCall.call, newCall, true)

			// Decrease the symbol use count by one
			if symbol.UseCountEstimate != 0 {
				symbol.UseCountEstimate--
				if symbol.UseCountEstimate == 0 {
					// Remove this function from the client bundle (we restore it when building the server bundle later)
					ref, prop := getRefForIdentifierOrPropertyAccess(visitor, &serverCall.call.Target)
					ref = c.findOriginalRef(visitor, ref, prop)
					if ref != js_ast.InvalidRef {
						analyzer := c.analyzers[ref.SourceIndex]
						if analyzer != nil {
							if f := analyzer.serverFunctions[ref]; f != nil {
								if f.part != nil {
									// TODO set ForceRemove to ensure this part gets removed by tree-shaking
									//f.part.ForceRemove = true
								}
							}
						}
					}
				}
			}
		}
	}

	validatorsReady.Wait()
	c.serverFile = createFunctionMapEntryPoint(sb, calls)
}

func (c *FlowStateCompiler) writeValidators(validators map[string]importsByName, validatorsByQuery map[string][]string) *strings.Builder {
	sb := &strings.Builder{}

	orderedFuncs := writeImports(sb, validators)
	aliasesByHash := map[string]string{}

	// Order the queries by hash so it will be deterministic
	orderedQueries := make([]string, len(validatorsByQuery))
	for queryHash := range validatorsByQuery {
		orderedQueries = append(orderedQueries, queryHash)
	}
	sort.Strings(orderedQueries)

	for _, mod := range orderedFuncs {
		for _, imp := range mod.imports {
			aliasesByHash[imp.hash] = imp.alias
		}
	}

	i := 0
	sb.WriteString("\nexport const validators = {\n")
	for _, queryHash := range orderedQueries {
		validatorHashes := validatorsByQuery[queryHash]
		if len(validatorHashes) == 0 {
			continue
		}
		if i != 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString("\t\"")
		sb.WriteString(queryHash)
		sb.WriteString("\": (e, s) => {\n")
		for _, validatorHash := range validatorHashes {
			sb.WriteString("\t\t")
			sb.WriteString(aliasesByHash[validatorHash])
			sb.WriteString("(e, s);\n")
		}
		sb.WriteString("\t}")
		i++
	}
	sb.WriteString("\n};\n")

	return sb
}

func (c *FlowStateCompiler) generateOutputs() {
	validators := map[string]importsByName{}
	// query hash -> [validator hashes]
	validatorsByQuery := map[string][]string{}


	allQueries := map[js_ast.Ref]queriesByWhitelistOrder{}

	var keys []js_ast.Ref
	for _, visitor := range c.analyzers {
		if visitor == nil {
			continue
		}

		for _, qp := range visitor.queries {
			keys = append(keys, qp.ref)
			q, err := newQuery(qp)
			if err != nil {
				c.log.AddError(qp.definedSource, qp.parent.Loc, err.Error())
				continue
			}
			allQueries[qp.ref] = append(allQueries[qp.ref], q)
		}
	}

	for sourceIndex, analyzer := range c.analyzers {
		if analyzer == nil {
			continue
		}

		// Find the queries for each queryExecute call, and gather the usages
		for i := range analyzer.queryExecutions {
			queryExec := &analyzer.queryExecutions[i]
			queryExec.queries = c.findQueryForExpr(analyzer, allQueries, &queryExec.call.Args[0])

			if len(queryExec.queries) == 0 {
				// Not a supported query execution expression, issue an error
				c.log.AddError(&analyzer.file.Source, queryExec.call.Args[0].Loc, "could not identify query for first argument to executeQuery")
				continue
			}

			for _, q := range queryExec.queries {
				if q.isFragment {
					c.log.AddError(&analyzer.file.Source, queryExec.call.Args[0].Loc, "cannot use a query part (created with sql.p``) as a query: use sql`${part}` instead")
					continue
				}
				q.calls = append(q.calls, queryUsage{sourceIndex: uint32(sourceIndex), isServer: queryExec.isServer, call: queryExec.call})
			}
		}
	}

	for _, analyzer := range c.analyzers {
		if analyzer == nil {
			continue
		}
		// Compile the queries and replace them (if client build) in the code with query objects
		for i := range analyzer.queryExecutions {
			queryExec := &analyzer.queryExecutions[i]
			for _, q := range queryExec.queries {
				// We've identified one or more queries that are associated with this executeQuery call
				// 1) Replace the query templates in the code with query objects
				// 2) Lookup the validator functions and record them

				if !q.compile(c, analyzer, allQueries) {
					continue
				}

				c.replaceQuery(analyzer, q)
				q.Type = getQueryType(q.QueryText)

				// Match the validator identifiers up to the defined server functions
				if len(queryExec.call.Args) > 2 {
					validatorArgs := queryExec.call.Args[2:]
					for j := range validatorArgs {
						validator := &validatorArgs[j]
						symbol, imp := c.findImportForFunctionIdentifier(analyzer, validator, validators)
						if symbol == nil {
							continue
						}
						validatorsByQuery[q.Hash] = append(validatorsByQuery[q.Hash], imp.hash)
					}
				}
			}
		}
	}

	// Add all client queries to the clientWhitelist
	// Add all server queries to the serverWhitelist
	// Include unreachable queries because we can't determine reachability 100% accurately.
	// Replace all queries with query objects

	clientWhitelist := make(queriesByWhitelistOrder, 0, len(allQueries))
	serverWhitelist := make(queriesByWhitelistOrder, 0, len(allQueries))
	for _, queries := range allQueries {
		for _, q := range queries {
			if q.isFragment {
				// If the fragment was completely inlined, replace it with undefined
				if q.inlinedClientCount == q.ClientReferences {
					c.replaceExpr(q.parent, q.template, &js_ast.EUndefined{}, q.inlinedServerCount != q.ServerReferences)
				} else if q.inlinedServerCount == q.ServerReferences {
					// Used on the client, but not the server
					c.undoReplaceExpr = append(c.undoReplaceExpr, undoReplaceExpr{q.parent, &js_ast.EUndefined{}})
				}
				continue
			}

			if q.ClientReferences != 0 {
				clientWhitelist = append(clientWhitelist, q)
			} else if q.ServerReferences != 0 {
				serverWhitelist = append(serverWhitelist, q)
			}
		}
	}

	if c.log.HasErrors() {
		return
	}

	c.wg.Add(2)

	go func(wl queriesByWhitelistOrder) {
		c.outputWhitelist(&c.clientWhitelistFile, "client-queries.json", wl)
		c.wg.Done()
	}(clientWhitelist)

	go func(wl queriesByWhitelistOrder) {
		c.outputWhitelist(&c.serverWhitelistFile, "server-queries.json", wl)
		c.wg.Done()
	}(serverWhitelist)

	c.generateServerFile(validators, validatorsByQuery)
	c.wg.Wait()
}

func (c *FlowStateCompiler) outputWhitelist(whitelistFile *OutputFile, fileName string, whitelistQueries queriesByWhitelistOrder) {
	sort.Sort(whitelistQueries)
	whitelistFile.Path = path.Join(c.outDir, fileName)

	contents, err := json.MarshalIndent(whitelistQueries, "", "\t")
	if err != nil {
		c.log.AddError(nil, logger.Loc{}, fmt.Sprintf("json.Marshal %s: %v", fileName, err.Error()))
	}

	if len(contents) != 0 {
		whitelistFile.Contents = contents
		err := c.fs.WriteFile(whitelistFile.Path, whitelistFile.Contents, 0644)
		if err != nil {
			c.log.AddError(nil, logger.Loc{}, fmt.Sprintf("write query whitelist %s: %v", fileName, err))
		}
	}
}

// replace replaces the query template literal with a compiled query object literal
// Replacing the queries is fairly straightforward
// At each template literal, we create the query object and bind the params
//
// let query = sql`SELECT foo, bar FROM baz WHERE foo > ${n + 1}`
//
// is replaced with:
//
// let query = {
//   "query": "b06bc30a53ac5d3feb624e536c86394ccb9ac5fc8da9bd239ef48724138e9fc1",
//	 "text": "SELECT foo, bar FROM baz WHERE foo > $0", // debug only
//   "type": "select",
//   "params": {
//     "$0": n + 1
//   }
// };
//
// For more complex examples see the unit tests in TODO.
func (c *FlowStateCompiler) replaceQuery(visitor *FlowStateAnalyzer, q *query) {
	if (q.QueryText != "") {
		return // already replaced query
	}

	loc := q.parent.Loc
	newProp := func(key string, value *js_ast.Expr) js_ast.Property {
		return js_ast.Property{
			Key:   js_ast.Expr{Data: &js_ast.EString{Value: js_lexer.StringToUTF16(key)}, Loc: loc},
			Value: value,
		}
	}

	digest := js_lexer.StringToUTF16(q.Hash)
	props := []js_ast.Property{
		newProp("query", &js_ast.Expr{Data: &js_ast.EString{Value: digest}, Loc: loc}),
	}

	i := 1
	q.IsPublic = true
	params := &js_ast.EObject{Properties: make([]js_ast.Property, 0, len(q.vars)), IsSingleLine: true}
	var queryFragments [][]*query
	var fragments []js_ast.Expr
	text := make([]string, 0, len(q.parts)*2-1)
	for j, v := range q.vars {
		text = append(text, q.parts[j])

		expr := v.expr
		name := v.name
		switch v.ty {
		case queryVarTypeVar:
			text = append(text, fmt.Sprintf("$%d", i))
		case queryVarTypeParam:
			// late-bound parameter (at executeQuery call)
			expr = &js_ast.Expr{Data: &js_ast.EString{Value: lateBoundParamStringLiteral}, Loc: loc}
			text = append(text, fmt.Sprintf("$%d", i))
		case queryVarTypeFragment:
			fragments = append(fragments, *expr)
			queryFragments = append(queryFragments, v.fragments)
			var n string
			if name == "" {
				n =  fmt.Sprintf("${fragment%d}", len(fragments))
			} else {
				n = fmt.Sprintf("${%s}", v.name)
			}
			text = append(text, n)
			continue
		case queryVarTypeServer:
			q.IsPublic = false
			text = append(text, fmt.Sprintf("${%s}", v.name))
			continue
		}
		if name == "" {
			name = text[len(text)-1]
		}
		params.Properties = append(params.Properties, newProp(name, expr))
		i++
	}

	if len(q.parts) > len(q.vars) {
		text = append(text, q.parts[len(q.parts)-1])
	}
	q.QueryText = strings.Join(text, "")
	if c.debug || q.ServerReferences != 0 {
		props = append(props, newProp("text", &js_ast.Expr{Data: &js_ast.EString{Value: js_lexer.StringToUTF16(q.QueryText)}, Loc: loc}))
	}
	props = append(props, newProp("params", &js_ast.Expr{Data: params, Loc: loc}))

	queryObj := &js_ast.EObject{
		Properties:   props,
		IsSingleLine: true,
	}

	q.Fragments = queryFragments
	var newExpr js_ast.E = queryObj
	if len(fragments) == 0 {
		c.replaceExpr(q.parent, q.template, queryObj, false)
	} else {
		// Wrap as sql.merge(queryObj, fragments...)
		sql := *q.template.Tag
		if q.isFragment {
			sql = q.template.Tag.Data.(*js_ast.EDot).Target
		}
		call := &js_ast.ECall{
			Target: js_ast.Expr{Data: &js_ast.EDot{Target: sql, Name: "merge"}, Loc: loc},
			Args:   append([]js_ast.Expr{{Data: queryObj, Loc: loc}}, fragments...),
		}
		c.replaceExpr(q.parent, q.template, call, false)
		newExpr = call
	}

	if q.ClientReferences == 0 {
		if q.ServerReferences != 0 {
			// This query is only used by the server, remove it in the client build
			log.Printf("replaced server-only query starting with %q with undefined in client build\n", q.template.HeadRaw)
			c.replaceExpr(q.parent, newExpr, &js_ast.EUndefined{}, true)
		} else {
			if !q.isInlined() {
				text := "query is unused"
				if q.isFragment {
					text = "fragment is unused"
				}
				c.log.AddWarning(q.definedSource, q.parent.Loc, text)
			}
		}
	} else if q.ServerReferences == 0 {
		// This query is used on the client, but not the server.
		// We want to remove it for the server build, but leave it as-is for the client build
		// We can do this by adding an "undo" entry that removes it when run before the server build.
		c.undoReplaceExpr = append(c.undoReplaceExpr, undoReplaceExpr{q.parent, &js_ast.EUndefined{}})
	}

	for _, fragments := range queryFragments {
		private := len(fragments) != 0 // assume all of these fragments are private
		for _, fragment := range fragments {
			if fragment.IsPublic {
				private = false // if one fragment is public, the whole group is
			}
			c.replaceQuery(c.analyzers[fragment.ref.SourceIndex], fragment)
		}
		// If any group of fragments is private, the query is private
		if q.IsPublic && private {
			q.IsPublic = false
		}
	}
}

func (c *FlowStateCompiler) findOriginalRef(analyzer *FlowStateAnalyzer, ref js_ast.Ref, prop string) js_ast.Ref {
	log.Printf("findOriginalRef(%d, %v, %q)\n", analyzer.file.Source.Index, ref, prop)
	for ref != js_ast.InvalidRef && analyzer != nil {
		queryImport, isImported := analyzer.ast.NamedImports[ref]
		if isImported {
			impRecord := analyzer.ast.ImportRecords[queryImport.ImportRecordIndex]
			if !impRecord.SourceIndex.IsValid() {
				break
			}
			exporter := c.analyzers[impRecord.SourceIndex.GetIndex()]
			if exporter == nil {
				break
			}

			if exp, ok := exporter.ast.NamedExports[queryImport.Alias]; ok {
				ref = exp.Ref
			} else if exp, ok := exporter.ast.NamedExports[prop]; ok {
				ref = exp.Ref
				prop = ""
			} else if len(exporter.ast.ExportStarImportRecords) != 0 {
				for _, i := range exporter.ast.ExportStarImportRecords {
					idx := exporter.ast.ImportRecords[i].SourceIndex
					if !idx.IsValid() {
						continue
					}
					other := c.analyzers[idx.GetIndex()]
					if other == nil {
						continue
					}
					if exp, ok := other.ast.NamedExports[queryImport.Alias]; ok {
						ref = exp.Ref
						break
					}
				}
			} else {
				log.Printf("can't find export for %s in %d", queryImport.Alias, exporter.file.Source.Index)
				break
			}
			analyzer = exporter
		} else {
			log.Printf("checking aliases and exports in %d\n", ref.SourceIndex)
			analyzer := c.analyzers[ref.SourceIndex]
			if analyzer == nil {
				break
			}
			if expRef, ok := c.analyzers[ref.SourceIndex].exports[ref]; ok {
				ref = expRef
			} else if aliasedRef, ok := c.analyzers[ref.SourceIndex].aliases[ref]; ok {
				ref = aliasedRef
				analyzer = c.analyzers[ref.SourceIndex]
			} else if prop != "" {
				log.Printf("looking for exported namespace %v in %d\n", ref, ref.SourceIndex)
				if ns, ok := c.analyzers[ref.SourceIndex].exportedNamespaces[ref]; ok {
					analyzer = c.analyzers[ns]
					if analyzer == nil {
						break
					}
					if exp, ok := analyzer.ast.NamedExports[prop]; ok {
						ref = exp.Ref
						prop = ""
						continue
					}
				}
				break
			} else {
				break // not supported
			}
		}
	}
	return ref
}

func (c *FlowStateCompiler) addQueryUsageLocations(q *query) bool {
	for _, call := range q.calls {
		if call.isServer {
			q.ServerReferences += 1
		} else {
			q.ClientReferences += 1
		}
		location := logger.LocationOrNil(&c.files[call.sourceIndex].Source, logger.Range{Loc: call.call.Target.Loc})
		if location != nil {
			insert := true
			for _, usage := range q.Usages {
				if usage.File == location.File && usage.Line == uint32(location.Line) {
					insert = false
					break
				}
			}
			if insert {
				q.Usages = append(q.Usages, sourceLocation{
					Line: uint32(location.Line),
					File: location.File,
				})
			}
		}
	}

	return len(q.Usages) != 0
}

func (c *FlowStateCompiler) findQueryForExpr(analyzer *FlowStateAnalyzer, allQueries map[js_ast.Ref]queriesByWhitelistOrder, expr *js_ast.Expr) queriesByWhitelistOrder {
	if expr == nil {
		return nil
	}
	queryRef, prop := getRefForIdentifierOrPropertyAccess(analyzer, expr)
	if queryRef == js_ast.InvalidRef {
		// If expression is a ternary condition or logical expression, call findQueryForExpr for each subexpression and combine the results
		var q1, q2 []*query
		switch e := expr.Data.(type) {
		//case *js_ast.ECall:
		//	e.Target
		case *js_ast.EIf:
			q1 = c.findQueryForExpr(analyzer, allQueries, &e.Yes)
			q2 = c.findQueryForExpr(analyzer, allQueries, &e.No)
		case *js_ast.EBinary:
			switch e.Op {
			case js_ast.BinOpLogicalOr, js_ast.BinOpLogicalAnd:
				q1 = c.findQueryForExpr(analyzer, allQueries, &e.Left)
				q2 = c.findQueryForExpr(analyzer, allQueries, &e.Right)
			}
		}
		if len(q2) == 0 {
			return q1
		} else if len(q1) == 0 {
			return q2
		} else {
			// Mergesort q1 and q2 into a single array. This is overkill to optimize this, but it was fun!
			a := make(queriesByWhitelistOrder, len(q1)+len(q2))
			i := 0
			j := 0
			for k := range a {
				if i < len(q1) {
					if j < len(q2) {
						// Add both elements to the array, compare them using a.Less
						// And if they're in the wrong order, Swap them with.
						// But only advance the source array for the element we keep, and only advance
						// the length of a by 1.
						a[k] = q1[i]
						a[k+1] = q2[j]
						if a.Less(k+1, k) {
							a.Swap(k+1, k)
							j++
						} else {
							i++
						}
					} else {
						a[k] = q1[i]
						i++
					}
				} else {
					a[k] = q2[j]
					j++
				}
			}
			return a
		}
	}
	if analyzer == nil {
		analyzer = c.analyzers[queryRef.SourceIndex]
	}
	queryRef = c.findOriginalRef(analyzer, queryRef, prop)
	a := allQueries[queryRef]
	log.Printf("%d queries found for original ref %v\n", len(a), queryRef)
	sort.Sort(a)
	return a
}

func (c *FlowStateCompiler) replaceExpr(expr *js_ast.Expr, old, new js_ast.E, undo bool) bool {
	if expr.Data != old {
		return false
	}

	expr.Data = new
	if undo {
		c.undoReplaceExpr = append(c.undoReplaceExpr, undoReplaceExpr{expr, old})
	}
	return true
}

func (c *FlowStateCompiler) replaceStmt(stmt *js_ast.Stmt, old, new js_ast.S, undo bool) bool {
	if stmt.Data != old {
		return false
	}

	stmt.Data = new
	if undo {
		c.undoReplaceStmt = append(c.undoReplaceStmt, undoReplaceStmt{stmt, old})
	}
	return true
}

func getRefForIdentifierOrPropertyAccess(visitor *FlowStateAnalyzer, expr *js_ast.Expr) (ref js_ast.Ref, prop string) {
	ref = js_ast.InvalidRef
	switch f := expr.Data.(type) {
	case *js_ast.EIdentifier:
		ref = f.Ref
	case *js_ast.EImportIdentifier:
		ref = f.Ref
	case *js_ast.EDot:
		// A property access as the target is allowed only if it's a property access on an import namespace
		//   import * as ns from "foo"
		//   ns.someFunc(...)
		prop = f.Name
		switch t := f.Target.Data.(type) {
		case *js_ast.EIdentifier:
			ref = t.Ref
		case *js_ast.EImportIdentifier:
			ref = t.Ref
		}
	case *js_ast.EIndex:
		switch k := f.Index.Data.(type) {
		case *js_ast.EString:
			prop = js_lexer.UTF16ToString(k.Value)
		}
		switch t := f.Target.Data.(type) {
		case *js_ast.EIdentifier:
			ref = t.Ref
		case *js_ast.EImportIdentifier:
			ref = t.Ref
		}
	case *js_ast.ETemplate:
		if visitor != nil {
			if inlineRef, ok := visitor.inlineTemplates[f]; ok {
				ref = inlineRef
			}
		}
	}
	return
}