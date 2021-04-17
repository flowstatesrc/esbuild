package api

import (
	"github.com/evanw/esbuild/internal/graph"
	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/logger"
)

type queryExecution struct {
	call *js_ast.ECall
	queries queriesByWhitelistOrder
	isServer bool
}

type serverCall struct {
	parent *js_ast.Expr
	call *js_ast.ECall
	fsInstance *js_ast.EDot
}

type localFunction struct {
	part *js_ast.Part
	stmt *js_ast.Stmt
	fnStmt *js_ast.SFunction
	fnArrow *js_ast.EArrow
	fnExpr *js_ast.EFunction
	local *js_ast.SLocal
	decl *js_ast.Decl
}

type queryUsage struct {
	call *js_ast.ECall
	sourceIndex uint32
	isServer bool
}

type queryPart struct {
	ref js_ast.Ref
	parent *js_ast.Expr
	template *js_ast.ETemplate
	calls []queryUsage
	definedSource *logger.Source
	isFragment bool
}

type FlowStateAnalyzer struct {
	compiler *FlowStateCompiler
	file *graph.InputFile
	ast *js_ast.AST
	exports map[js_ast.Ref]js_ast.Ref
	exportedNamespaces map[js_ast.Ref]uint32
	queryParts []queryPart
	inlineTemplates map[*js_ast.ETemplate]js_ast.Ref
	// simple assignments with one identifier to another, e.g.: foo = bar
	aliases map[js_ast.Ref]js_ast.Ref
	// queryExecutions are possible queryExecutions, since we can't tell until
	// after we've visited all files if it's really a valid query execution or not.
	queryExecutions []queryExecution
	serverCalls []serverCall
	serverFunctions map[js_ast.Ref]*localFunction
	serverFunctionsByCtxVar map[js_ast.Ref]*localFunction
	queries []queryPart
	mergeImportRef js_ast.Ref
}

func NewFlowStateAnalyzer(compiler *FlowStateCompiler, file *graph.InputFile) *FlowStateAnalyzer {
	if js, ok := file.Repr.(*graph.JSRepr); ok {
		ast := &js.AST
		return &FlowStateAnalyzer{
			compiler: compiler,
			file:     file,
			ast: ast,
			exports: map[js_ast.Ref]js_ast.Ref{},
			exportedNamespaces: map[js_ast.Ref]uint32{},
			inlineTemplates: map[*js_ast.ETemplate]js_ast.Ref{},
			aliases: map[js_ast.Ref]js_ast.Ref{},
			serverFunctions: map[js_ast.Ref]*localFunction{},
			serverFunctionsByCtxVar: map[js_ast.Ref]*localFunction{},
			mergeImportRef: js_ast.InvalidRef,
		}
	}
	return nil
}

func (a *FlowStateAnalyzer) VisitStmt(stmt *js_ast.Stmt, part *js_ast.Part) StmtVisitor {
	// Visit the AST, discovering query executions and backend functions calls
	// We save these, and then afterwards we'll work backward from
	// them to find the backend function declarations, validator declarations,
	// and query declarations and construction.

	// Since we're looking for a function call expression, we only need concern
	// ourselves with statement types that can contain a function call.
	switch s := stmt.Data.(type) {
	case *js_ast.SFunction:
		if part != nil {
			a.recordServerFunction(part, stmt, s)
		}
	case *js_ast.SExportFrom:
		// We need to create a bridge from the exported Ref to the exported Ref in the file it references
		for _, item := range s.Items {
			expRef := item.Name.Ref
			imp := a.ast.ImportRecords[s.ImportRecordIndex]
			if imp.SourceIndex.IsValid() {
				targetExport := a.compiler.analyzers[imp.SourceIndex.GetIndex()].ast.NamedExports[item.OriginalName]
				a.exports[expRef] = targetExport.Ref
			}
		}
	case *js_ast.SExportStar:
		imp := a.ast.ImportRecords[s.ImportRecordIndex]
		if imp.SourceIndex.IsValid() {
			a.exportedNamespaces[s.NamespaceRef] = imp.SourceIndex.GetIndex()
		}
	}
	return a
}

func (a *FlowStateAnalyzer) Visit(stmt *js_ast.Stmt, expr *js_ast.Expr, decl *js_ast.Decl, parents []*js_ast.Expr, part *js_ast.Part) ExprVisitor {
	switch e := expr.Data.(type)  {
		case *js_ast.ECall:
			a.recordFlowStateCall(expr, e)
		case *js_ast.EArrow:
			if part != nil && decl != nil {
				a.recordServerFunctionVar(part, stmt, stmt.Data.(*js_ast.SLocal), decl, expr, e, nil)
			}
		case *js_ast.EFunction:
			if part != nil && decl != nil {
				a.recordServerFunctionVar(part, stmt, stmt.Data.(*js_ast.SLocal), decl, expr, nil, e)
			}
		case *js_ast.ETemplate:
			a.recordSQLTemplate(stmt, decl, parents, expr)
		case *js_ast.EIdentifier:
			if part != nil && stmtIsExport(stmt.Data) {
				a.recordExport(stmt.Data, decl, parents, e)
			} else if decl != nil && (len(parents) == 0 || isTargetOfIndexOrDot(parents, e)) {
				// This is an aliasing assignment from identifier e to decl.Binding
				// if e is the right hand side expression (len(parents) == 0) OR
				// if e is the target (root) of an index or dot expression that is the right hand side expression
				// e.g. foo = e[key] or foo = e.prop. See isTargetOfIndexOrDot for more details.
				a.recordAlias(decl, e.Ref)
			}
		case *js_ast.EImportIdentifier:
			if part != nil && stmtIsExport(stmt.Data) {
				a.recordExport(stmt.Data, decl, parents, e)
			} else if decl != nil && (len(parents) == 0 || isTargetOfIndexOrDot(parents, e)) {
				a.recordAlias(decl, e.Ref)
			}
		case *js_ast.EBinary:
			switch e.Op {
			case js_ast.BinOpAssign:
				left, _ := getRefForIdentifierOrPropertyAccess(a, &e.Left)
				right, _ := getRefForIdentifierOrPropertyAccess(a, &e.Right)
				if left != js_ast.InvalidRef && right != js_ast.InvalidRef {
					a.aliases[left] = right
				}
			}
	}
	return a
}

func (a *FlowStateAnalyzer) recordExport(stmt js_ast.S, decl *js_ast.Decl, parents []*js_ast.Expr, expr js_ast.E) {
	isLocal := false
	expRef := js_ast.InvalidRef
	switch s := stmt.(type) {
	case *js_ast.SExportDefault:
		expRef = s.DefaultName.Ref
	case *js_ast.SExportClause, *js_ast.SExportFrom:
	case *js_ast.SLocal:
		if decl != nil {
			if id, ok := decl.Binding.Data.(*js_ast.BIdentifier); ok {
				expRef = id.Ref
				isLocal = true
				break
			}
		}
	default:
		log.Printf("recordExport: %T stmt not handled yet\n", s)
	}

	if expRef == js_ast.InvalidRef {
		return
	}

	var identName string
	identifier := js_ast.InvalidRef
	switch e := expr.(type) {
	case *js_ast.EIdentifier:
		identName = a.ast.Symbols[e.Ref.InnerIndex].OriginalName
		identifier = e.Ref
	case *js_ast.EImportIdentifier:
		identName = a.ast.Symbols[e.Ref.InnerIndex].OriginalName
		identifier = e.Ref
	default:
		return
	}

	if isLocal {
		// We only map the identifier to the exported ref if we can trace it back to
		// the declaration through an unbroken chain of supported expressions.
		// These are currently object literals, ternary conditions, and binary && or || operations.

		start := len(parents) - 1
		for i := start; i >= 0; i-- {
			switch e := parents[i].Data.(type) {
			case *js_ast.EBinary:
				if e.Op == js_ast.BinOpLogicalAnd || e.Op == js_ast.BinOpLogicalOr {
					continue
				}
				return
			case *js_ast.EObject, *js_ast.EIf:
			default:
				return
			}
		}
	}

	log.Printf("map export %v -> %s", expRef, identName)
	a.exports[expRef] = identifier
}

func (a *FlowStateAnalyzer) recordAlias(decl *js_ast.Decl, ref js_ast.Ref) {
	if identifier, ok := decl.Binding.Data.(*js_ast.BIdentifier); ok {
		log.Printf("alias %s -> %s\n", a.ast.Symbols[identifier.Ref.InnerIndex].OriginalName, a.ast.Symbols[ref.InnerIndex].OriginalName)
		a.aliases[identifier.Ref] = ref
	}
}

func (a *FlowStateAnalyzer) recordServerFunction(part *js_ast.Part, stmt *js_ast.Stmt, f *js_ast.SFunction) {
	if len(f.Fn.Args) == 0 {
		// A server function requires a context argument
		return
	}
	fun := &localFunction{part: part, stmt: stmt, fnStmt: f}
	a.serverFunctions[f.Fn.Name.Ref] = fun
	if ctx, ok := f.Fn.Args[0].Binding.Data.(*js_ast.BIdentifier); ok {
		a.serverFunctionsByCtxVar[ctx.Ref] = fun
	}
}

func (a *FlowStateAnalyzer) recordServerFunctionVar(part *js_ast.Part, stmt *js_ast.Stmt, local *js_ast.SLocal, decl *js_ast.Decl, expr *js_ast.Expr, arrowFn *js_ast.EArrow, fnExpr *js_ast.EFunction) {
	if !local.IsExport {
		return
	}
	if identifier, ok := decl.Binding.Data.(*js_ast.BIdentifier); ok {
		var args []js_ast.Arg
		if arrowFn != nil {
			args = arrowFn.Args
		} else {
			args = fnExpr.Fn.Args
		}

		if len(args) == 0 {
			// A server function requires a context argument
			return
		}

		fun := &localFunction{
			part: part,
			stmt: stmt,
			fnArrow: arrowFn,
			fnExpr: fnExpr,
			local: local,
			decl: decl,
		}

		if ctx, ok := args[0].Binding.Data.(*js_ast.BIdentifier); ok {
			a.serverFunctionsByCtxVar[ctx.Ref] = fun
		}
		a.serverFunctions[identifier.Ref] = fun
	}
}

const sqlTemplateTag = "sql"
const templatePart = "p"

func (a *FlowStateAnalyzer) recordSQLTemplate(stmt *js_ast.Stmt, decl *js_ast.Decl, parents []*js_ast.Expr, expr *js_ast.Expr) bool {
	// The sql template must be
	//  - assigned to an identifier
	//  - assigned to an object property
	//  - assigned to an object literal property
	//  - a branch of a ternary expression assigned to one of the above
	//  - part of a logical expression involving || and && operators assigned to one of the above
	//
	// If it's not one of these supported cases, we treat it as an error.
	template := expr.Data.(*js_ast.ETemplate)

	if template.Tag == nil {
		return false
	}

	isFragment := false
	ref := js_ast.InvalidRef
	switch target := template.Tag.Data.(type) {
	case *js_ast.EIdentifier:
		ref = target.Ref
	case *js_ast.EImportIdentifier:
		ref = target.Ref
	case *js_ast.EDot:
		if ident, ok := target.Target.Data.(*js_ast.EIdentifier); target.Name == templatePart && ok {
			ref = ident.Ref
			isFragment = true
		}
	}

	if ref == js_ast.InvalidRef {
		return false
	}

	symbol := a.ast.Symbols[ref.InnerIndex]
	if symbol.OriginalName != sqlTemplateTag {
		return false
	}

	log.Printf("found sql template starting with %s\n", template.HeadRaw)

	// Find the base expression, the root expression containing the query literal
	var assign interface{} = stmt.Data
	ref = js_ast.InvalidRef
	start := len(parents) - 1
CheckParents:
	for i := start; i >= 0; i-- {
		// This template expression is not directly assigned to an identifier.
		// The only cases where this is currently supported is if it's part of ternary
		// expression or a logical expression, or an object literal that is itself
		// part of an assignment. We might have these kinds of expressions nested
		// in each other multiple levels deep, so work backwards as far as possible
		// and then we'll try to link the result with the above.

		switch e := parents[i].Data.(type) {
		case *js_ast.EBinary:
			// This is an assignment expression, record the assignment target
			if e.Op == js_ast.BinOpAssign {
				assign = e.Left.Data
				break CheckParents
			}
		case *js_ast.ECall:
			var iface interface{} = template
			// If the template is inlined directly into a executeQuery expression, record it here
			if i == start && e.Args[0].Data == iface {
				// Invent a reference and add it to inlineTemplates map using the template pointer as the key
				highestInner := uint32(0x00ffffff)
				for _, r := range a.inlineTemplates {
					if r.InnerIndex > highestInner {
						highestInner = r.InnerIndex
					}
				}
				highestInner++
				ref = js_ast.Ref{SourceIndex: a.file.Source.Index, InnerIndex: highestInner}
				a.inlineTemplates[template] = ref
				log.Printf("template is inline in call expr, tagging with ref %v\n", ref)
				break CheckParents
			}
		}
	}

	if ref == js_ast.InvalidRef {
		// Now look for the base expression in the assignment statement to get the identifier for it
		// ref is now the identifier reference for the (expression) containing the query literal
		switch s := assign.(type) {
		case *js_ast.SLocal:
			if b, ok := decl.Binding.Data.(*js_ast.BIdentifier); ok {
				ref = b.Ref
			}
		case *js_ast.EIdentifier, *js_ast.EIndex, *js_ast.EDot:
			// This was the left (target) part of a binary assignment expression
			ref, _ = getRefForIdentifierOrPropertyAccess(a, &js_ast.Expr{Data: s.(js_ast.E)})
			// TODO what do we do with prop in the case of a foo["prop"] or foo.prop expression?
		}
	}

	if ref == js_ast.InvalidRef {
		return false
	}

	log.Printf("add query %q with ref %v\n", template.HeadRaw, ref)
	a.queries = append(a.queries, queryPart{
		ref:           ref,
		parent:        expr,
		template:      template,
		definedSource: &a.file.Source,
		isFragment: isFragment,
	})

	return true
}

const queryExecuteMethodName = "executeQuery"
const beginTransactionMethodName = "beginTx"
const serverCallMethodName = "serverCall"

func (a *FlowStateAnalyzer) recordFlowStateCall(parent *js_ast.Expr, call *js_ast.ECall) bool {
	// Both types of calls require at least 1 argument
	if len(call.Args) == 0 {
		return false
	}

	ast := a.ast

	isPossibleServerCall := func(ref js_ast.Ref) bool {
		symbol := ast.Symbols[ref.InnerIndex]
		if symbol.OriginalName == queryExecuteMethodName {
			a.compiler.log.AddError(&a.file.Source, call.Target.Loc, "executeQuery must be invoked as a method")
			return false
		}
		return true
	}

	switch target := call.Target.Data.(type) {
	case *js_ast.EIdentifier:
		if isPossibleServerCall(target.Ref) {
			return a.recordServerCall(parent, call)
		}
	case *js_ast.EImportIdentifier:
		if isPossibleServerCall(target.Ref) {
			return a.recordServerCall(parent, call)
		}
	case *js_ast.EDot:
		// Calling a property access expression - could be either a server or query call
		isServer := false
		if target.Name == queryExecuteMethodName {
			// This is a possible query execution, we'll verify them after we've visited all files

			// If the left side of the target refers to the first argument of a function
			// that has been recorded as a possible server call, then this is a server query execution.
			switch lhs := target.Target.Data.(type) {
			case *js_ast.EIdentifier:
				f := a.serverFunctionsByCtxVar[lhs.Ref]
				isServer = f != nil
				if isServer {
					log.Println("found server queryExecute")
				} else {
					log.Println("found client queryExecute")
				}
			}
			a.queryExecutions = append(a.queryExecutions, queryExecution{call: call, isServer: isServer})
			return true
		} else {
			return a.recordServerCall(parent, call)
		}
	}
	return false
}

func (a *FlowStateAnalyzer) recordServerCall(parent *js_ast.Expr, call *js_ast.ECall) bool {
	switch ctx := call.Args[0].Data.(type) {
	case *js_ast.ECall:
		// beginTx doesn't currently accept arguments
		if len(ctx.Args) != 0 {
			return false
		}
		if dot, ok := ctx.Target.Data.(*js_ast.EDot); ok {
			if dot.Name != beginTransactionMethodName {
				return false
			}

			// We have a function call where the first argument is xxx.beginTx(). That's a server call.
			log.Println("record server call")
			a.serverCalls = append(a.serverCalls, serverCall{parent: parent, call: call, fsInstance: dot})
		}
	}
	return false
}

func stmtIsExport(stmt js_ast.S) bool {
	switch s := stmt.(type) {
	case *js_ast.SLocal:
		return s.IsExport
	case *js_ast.SExportDefault, *js_ast.SExportClause, *js_ast.SExportEquals, *js_ast.SExportFrom, *js_ast.SExportStar, *js_ast.SLazyExport:
		return true
	default:
		return false
	}
}

func isTargetOfIndexOrDot(parents []*js_ast.Expr, identifier js_ast.E) bool {
	if len(parents) != 1 {
		return false
	}
	switch parent := parents[0].Data.(type) {
	case *js_ast.EDot:
		return parent.Target.Data == identifier
	case *js_ast.EIndex:
		return parent.Target.Data == identifier
	}
	return false
}