package api

import (
	"fmt"

	"github.com/evanw/esbuild/internal/js_ast"
)

type ExprVisitor interface {
	Visit(s *js_ast.Stmt, e *js_ast.Expr, decl *js_ast.Decl, parents []*js_ast.Expr, part *js_ast.Part) ExprVisitor
}

type StmtVisitor interface {
	VisitStmt(s *js_ast.Stmt, part *js_ast.Part) StmtVisitor
}

type noopExprVisitor struct {}

func (n *noopExprVisitor) Visit(s *js_ast.Stmt, e *js_ast.Expr, decl *js_ast.Decl, parents []*js_ast.Expr, part *js_ast.Part) ExprVisitor {
	return n
}

type noopStmtVisitor struct {}

func (n *noopStmtVisitor) VisitStmt(s *js_ast.Stmt, part *js_ast.Part) StmtVisitor {
	return n
}

func WalkAst(stmtVisitor StmtVisitor, visitor ExprVisitor, ast *js_ast.AST) bool {
	v:= FlowStateVisitor{stmtVisitor: stmtVisitor, visitor: visitor}
	v.init()
	for i := range ast.Parts {
		part := &ast.Parts[i]
		for j := range part.Stmts {
			v.visitStmt(&part.Stmts[j], part)
			if v.stmtVisitor == nil && v.visitor == nil {
				return true
			}
		}
	}
	return false
}

func WalkStmts(visitor StmtVisitor, stmts []*js_ast.Stmt, part *js_ast.Part) bool {
	v:= FlowStateVisitor{stmtVisitor: visitor}
	v.init()
	for _, stmt := range stmts {
		v.visitStmt(stmt, part)
		if v.stmtVisitor == nil {
			return true
		}
	}
	return false
}

func Walk(visitor ExprVisitor, stmts []*js_ast.Stmt, part *js_ast.Part) bool {
	v:= FlowStateVisitor{visitor: visitor}
	v.init()
	for _, stmt := range stmts {
		v.visitStmt(stmt, part)
		if v.visitor == nil {
			return true
		}
	}
	return false
}

type FlowStateVisitor struct {
	stmtVisitor StmtVisitor
	visitor ExprVisitor
}

func (v *FlowStateVisitor) init() {
	if v.visitor == nil {
		v.visitor = &noopExprVisitor{}
	}
	if v.stmtVisitor == nil {
		v.stmtVisitor = &noopStmtVisitor{}
	}
}

func (v *FlowStateVisitor) visitStmt(stmt *js_ast.Stmt, part *js_ast.Part) {
	if stmt == nil || v.stmtVisitor == nil {
		return
	}

	log.Printf("visit %T\n", stmt.Data)
	v.stmtVisitor = v.stmtVisitor.VisitStmt(stmt, part)
	if v.stmtVisitor == nil {
		return
	}

	switch s := stmt.Data.(type) {
	case *js_ast.SBlock:
		for i := range s.Stmts {
			v.visitStmt(&s.Stmts[i], nil)
		}
	case *js_ast.SLazyExport:
		v.visitExprs(stmt, part, &s.Value)
	case *js_ast.SExpr:
		v.visitExprs(stmt, part, &s.Value)
	case *js_ast.SEnum:
		exprs := make([]*js_ast.Expr, len(s.Values))
		for i := range s.Values {
			p := &s.Values[len(s.Values)-1-i].Value
			exprs[i] = *p
		}
		v.visitExprs(stmt, part, exprs...)
	case *js_ast.SNamespace:
		for i := range s.Stmts {
			v.visitStmt(&s.Stmts[i], nil)
		}
	case *js_ast.SFunction:
		for i := range s.Fn.Body.Stmts {
			v.visitStmt(&s.Fn.Body.Stmts[i], nil)
		}
	case *js_ast.SClass:
		exprs := make([]*js_ast.Expr, 0, len(s.Class.Properties)*2)
		for _, prop := range s.Class.Properties {
			if prop.Value != nil {
				exprs = append(exprs, prop.Value)
			}
			if prop.Initializer != nil {
				exprs = append(exprs, prop.Initializer)
			}
		}
		v.visitExprs(stmt, part, exprs...)
	case *js_ast.SLabel:
		v.visitStmt(&s.Stmt, nil)
	case *js_ast.SIf:
		v.visitExprs(stmt, part, &s.Test)
		v.visitStmt(&s.Yes, nil)
		if s.No != nil {
			v.visitStmt(s.No, nil)
		}
	case *js_ast.SFor:
		v.visitStmt(s.Init, nil)
		v.visitExprs(stmt, part, s.Update, s.Test)
		v.visitStmt(&s.Body, nil)
	case *js_ast.SForIn:
		v.visitStmt(&s.Init, nil)
		v.visitExprs(stmt, part, &s.Value)
		v.visitStmt(&s.Body, nil)
	case *js_ast.SForOf:
		v.visitStmt(&s.Init, nil)
		v.visitExprs(stmt, part, &s.Value)
		v.visitStmt(&s.Body, nil)
	case *js_ast.SDoWhile:
		v.visitStmt(&s.Body, nil)
		v.visitExprs(stmt, part, &s.Test)
	case *js_ast.SWhile:
		v.visitExprs(stmt, part, &s.Test)
		v.visitStmt(&s.Body, nil)
	case *js_ast.SWith:
		v.visitExprs(stmt, part, &s.Value)
		v.visitStmt(&s.Body, nil)
	case *js_ast.STry:
		for i := range s.Body {
			v.visitStmt(&s.Body[i], nil)
		}
		if s.Catch != nil {
			for i := range s.Catch.Body {
				v.visitStmt(&s.Catch.Body[i], nil)
			}
		}
		if s.Finally != nil {
			for i := range s.Finally.Stmts {
				v.visitStmt(&s.Finally.Stmts[i], nil)
			}
		}
	case *js_ast.SSwitch:
		exprs := make([]*js_ast.Expr, len(s.Cases)+1)
		for i, kase := range s.Cases {
			exprs[len(s.Cases)-1-i] = kase.Value
			for i := range kase.Body {
				v.visitStmt(&kase.Body[i], nil)
			}
		}
		exprs[len(exprs	)-1] = &s.Test
		v.visitExprs(stmt, part, exprs...)
	case *js_ast.SReturn:
		v.visitExprs(stmt, part, s.Value)
	case *js_ast.SThrow:
		v.visitExprs(stmt, part, &s.Value)
	case *js_ast.SLocal:
		exprs := make([]*js_ast.Expr, len(s.Decls))
		for i := range s.Decls {
			p := &s.Decls[len(s.Decls)-1-i].Value
			exprs[i] = *p
		}
		v.visitExprs(stmt, part, exprs...)
	case *js_ast.SExportDefault:
		if s.Value.Stmt != nil {
			v.visitStmt(stmt, nil)
		}	else if s.Value.Expr != nil {
			v.visitExprs(stmt, part, s.Value.Expr)
		}
	case *js_ast.SComment, *js_ast.SDebugger, *js_ast.SDirective, *js_ast.SEmpty, *js_ast.STypeScript, *js_ast.SExportClause, *js_ast.SExportFrom, *js_ast.SExportStar, *js_ast.SExportEquals, *js_ast.SBreak, *js_ast.SContinue, *js_ast.SImport:
	default:
		panic(fmt.Sprintf("unknown statement type %T", stmt.Data))
	}
}

func (v *FlowStateVisitor) visitExprs(stmt *js_ast.Stmt, part *js_ast.Part, exprs ...*js_ast.Expr) {
	if len(exprs) == 0 || v.visitor == nil {
		return
	}

	var decls []js_ast.Decl
	var decl *js_ast.Decl
	if local, ok := stmt.Data.(*js_ast.SLocal); ok {
		decls = local.Decls
	}

	// Since we're looking for a function call expression, we only need concern
	// ourselves with expression types that can contain a function call.
	tail := len(exprs) - 1
	// We append to parents, so it can outgrow this, we just use the stack buffer
	// to reduce the amount of allocations needed in the common case.
	var stackBuf [32]*js_ast.Expr
	parents := stackBuf[:0]
	var stackBuf2 [32]uint32
	popParents := stackBuf2[:0]

	for tail >= 0 {
		if tail < len(decls) {
			// The decls were added in reverse order to exprs
			decl = &decls[0]
			decls = decls[1:]
		}
		expr := exprs[tail]
		exprs = exprs[:tail]
		if expr == nil {
			tail--
			continue
		}

		log.Printf("visit %T\n", expr.Data)
		v.visitor = v.visitor.Visit(stmt, expr, decl, parents, part)
		if v.visitor == nil {
			return
		}

		switch e := expr.Data.(type) {
		case *js_ast.EArray:
			for i := range e.Items {
				exprs = append(exprs, &e.Items[len(e.Items)-1-i])
			}
		case *js_ast.EUnary:
			exprs = append(exprs, &e.Value)
		case *js_ast.EBinary:
			exprs = append(exprs, &e.Left)
			exprs = append(exprs, &e.Right)
		case *js_ast.ENew:
			exprs = append(exprs, &e.Target)
			for i := range e.Args {
				exprs = append(exprs, &e.Args[len(e.Args)-1-i])
			}
		case *js_ast.ECall:
			exprs = append(exprs, &e.Target)
			for i := range e.Args {
				exprs = append(exprs, &e.Args[len(e.Args)-1-i])
			}
		case *js_ast.EDot:
			exprs = append(exprs, &e.Target)
		case *js_ast.EIndex:
			exprs = append(exprs, &e.Target)
			exprs = append(exprs, &e.Index)
		case *js_ast.EArrow:
			for i := range e.Body.Stmts {
				v.visitStmt(&e.Body.Stmts[len(e.Body.Stmts)-1-i], nil)
			}
			for i := range e.Args {
				p := &e.Args[len(e.Args)-1-i].Default
				exprs = append(exprs, *p)
			}
		case *js_ast.EFunction:
			for i := range e.Fn.Body.Stmts {
				v.visitStmt(&e.Fn.Body.Stmts[len(e.Fn.Body.Stmts)-1-i], nil)
			}
			for i := range e.Fn.Args {
				p := &e.Fn.Args[len(e.Fn.Args)-1-i].Default
				exprs = append(exprs, *p)
			}
		case *js_ast.EClass:
			for i := range e.Class.Properties {
				prop := e.Class.Properties[len(e.Class.Properties)-1-i]
				if prop.Initializer != nil {
					exprs = append(exprs, prop.Initializer)
				}
				if prop.Value != nil {
					exprs = append(exprs, prop.Value)
				}
			}
		case *js_ast.EJSXElement:
			for i := range e.Children {
				exprs = append(exprs, &e.Children[len(e.Children)-1-i])
			}
			for _, prop := range e.Properties {
				if prop.Initializer != nil {
					exprs = append(exprs, prop.Initializer)
				}
				if prop.Value != nil {
					exprs = append(exprs, prop.Value)
				}
			}
			if e.Tag != nil {
				exprs = append(exprs, e.Tag)
			}
		case *js_ast.EObject:
			for i := range e.Properties {
				prop := e.Properties[len(e.Properties)-1-i]
				if prop.Initializer != nil {
					exprs = append(exprs, prop.Initializer)
				}
				if prop.Value != nil {
					exprs = append(exprs, prop.Value)
				}
			}
		case *js_ast.ESpread:
			exprs = append(exprs, &e.Value)
		case *js_ast.ETemplate:
			for i := range e.Parts {
				exprs = append(exprs, &e.Parts[len(e.Parts)-1-i].Value)
			}
			if e.Tag != nil {
				exprs = append(exprs, e.Tag)
			}
		case *js_ast.EAwait:
			exprs = append(exprs, &e.Value)
		case *js_ast.EYield:
			if e.Value != nil{
				exprs = append(exprs, e.Value)
			}
		case *js_ast.EIf:
			exprs = append(exprs, &e.No)
			exprs = append(exprs, &e.Yes)
			exprs = append(exprs, &e.Test)
		case *js_ast.EImport:
			exprs = append(exprs, &e.Expr)
		case *js_ast.EIdentifier, *js_ast.EImportIdentifier, *js_ast.EBoolean, *js_ast.ESuper, *js_ast.ENull, *js_ast.EUndefined, *js_ast.EThis, *js_ast.ENewTarget, *js_ast.EImportMeta, *js_ast.EPrivateIdentifier, *js_ast.EMissing, *js_ast.ENumber, *js_ast.EBigInt, *js_ast.EString, *js_ast.ERegExp, *js_ast.ERequire, *js_ast.ERequireResolve:
		default:
			panic(fmt.Sprintf("unknown expression type %T", e))
		}

		newTail := len(exprs) - 1
		if newTail < tail {
			// we didn't add any child expressions to the exprs stack, check if we can pop the top parent
			// the first child is at childrenStartIndex, so if newTail is below that, we've visited all
			// the children and can pop this parent.
			i := len(popParents)-1
			if i >= 0 && newTail < int(popParents[i]) {
				log.Printf("done with tree at %T", parents[i].Data)
				parents = parents[:i] // pop parent
				popParents = popParents[:i]
			}
		} else {
			// expr is the parent of newTail - tail children
			parents = append(parents, expr)
			popParents = append(popParents, uint32(tail))
		}

		tail = newTail
	}
}
