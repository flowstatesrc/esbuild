package api

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"

	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/logger"
)

type queryType uint8

const (
	queryTypeFragment queryType = iota
	queryTypeSelect
	queryTypeUpdate
	queryTypeDelete
	queryTypeInsert
	queryTypeOther
)

func (ty queryType) MarshalJSON() (s []byte, err error) {
	switch ty {
	case queryTypeSelect:
		s = []byte(`"select"`)
	case queryTypeUpdate:
		s = []byte(`"update"`)
	case queryTypeDelete:
		s = []byte(`"delete"`)
	case queryTypeInsert:
		s = []byte(`"insert"`)
	case queryTypeOther:
		s = []byte(`"other"`)
	case queryTypeFragment:
	default:
		return nil, errors.New("invalid query type")
	}
	return
}

func (ty queryType) String() string {
	var s string
	switch ty {
	case queryTypeSelect:
		s = "select"
	case queryTypeUpdate:
		s = "update"
	case queryTypeDelete:
		s = "delete"
	case queryTypeInsert:
		s = "insert"
	case queryTypeOther:
		s = "other"
	case queryTypeFragment:
	default:
		panic("invalid query type")
	}
	return s;
}

type queryVarType uint8

const (
	queryVarTypeVar queryVarType = iota
	queryVarTypeParam
	queryVarTypeServer
	queryVarTypeFragment
)

var reQueryType = regexp.MustCompile(`(?ms)(?:\s*(?:--.*?$|/*.*?\*/))*\s*(\w+)`)
var rePercentParams = regexp.MustCompile(`%\{([a-zA-Z0-9_.-]+?)\}`)

var serverVars = []string{"SESSION.","ENV."}

func getQueryType(query string) queryType {
	submatches := reQueryType.FindStringSubmatch(query)
	ty := queryTypeOther
	if len(submatches) == 0 {
		return ty
	}
	switch strings.ToLower(submatches[1]) {
	case "select":
		ty = queryTypeSelect
	case "update":
		ty = queryTypeUpdate
	case "insert":
		ty = queryTypeInsert
	case "delete":
		ty = queryTypeDelete
	}
	return ty
}

type sourceLocation struct {
	Line      uint32     `json:"line"`
	File  string     `json:"fileName"`
}

type query struct {
	queryPart `json:"-"`
	Hash      string     `json:"id"`
	QueryText string     `json:"query"`
	Type      queryType  `json:"type,omitempty"`
	IsPublic  bool       `json:"isPublic,omitempty"`
	isFragment bool
	// ServerReferences and ClientReferences are only tracked if isFragment=false
	ServerReferences uint16 `json:"serverReferences,omitempty"`
	ClientReferences uint16 `json:"clientReferences,omitempty"`
	inlinedClientCount uint16
	inlinedServerCount uint16
	DefinedAt        sourceLocation `json:"definedAt"`
	Usages           []sourceLocation `json:"usages,omitempty"`
	Params           []string   `json:"params,omitempty"` // to assist with reading whitelist file only
	Fragments        [][]*query  `json:"fragments,omitempty"`
	parts            []string
	vars             []queryVar
	binHash          [sha256.Size]byte
}

// queriesByWhitelistOrder sort queries by (isPublic, type, fileName, line)
type queriesByWhitelistOrder []*query

func (a queriesByWhitelistOrder) Len() int           { return len(a) }
func (a queriesByWhitelistOrder) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a queriesByWhitelistOrder) Less(i, j int) bool {
	if a[i].IsPublic == a[j].IsPublic {
		if a[i].Type == a[j].Type {
			if a[i].DefinedAt.File == a[j].DefinedAt.File {
				return a[i].DefinedAt.Line < a[j].DefinedAt.Line
			}
			return a[i].DefinedAt.File < a[j].DefinedAt.File
		}
		return a[i].Type < a[j].Type
	} else {
		return a[i].IsPublic
	}
}

type queryVar struct {
	ref       js_ast.Ref
	ty        queryVarType
	name      string // optional
	fragments []*query
	expr      *js_ast.Expr
}

func newQuery(qp queryPart) (*query, error) {
	parts := make([]string, 0, len(qp.template.Parts)+1)
	vars := make([]queryVar, 0, len(qp.template.Parts))

	appendPart := func(s string) {
		i := 0
		matches := rePercentParams.FindAllStringIndex(s, -1)
		for _, match := range matches {
			start := match[0]
			end := match[1]
			head := s[i:start]
			parts = append(parts, head)
			varName := s[start+2:end-1]
			ty := queryVarTypeParam
			for _, serverVar := range serverVars {
				if strings.HasPrefix(varName, serverVar) {
					ty = queryVarTypeServer
					break
				}
			}
			vars = append(vars, queryVar{ref: js_ast.InvalidRef, name: varName, ty: ty})
			i = end
		}
		s = s[i:]
		parts = append(parts, s)
	}

	appendPart(qp.template.HeadRaw)
	for i := range qp.template.Parts {
		part := &qp.template.Parts[i]
		ref, _ := getRefForIdentifierOrPropertyAccess(nil, &part.Value)
		vars = append(vars, queryVar{ref: ref, expr: &part.Value})
		tail := part.TailRaw
		// Record any :label: following the template expression and don't include it in parts.
		if len(tail) != 0 && tail[0] == ':' {
			if end := strings.IndexByte(tail[1:], ':'); end > 0 {
				end += 1 // we started from [1:]
				vars[len(vars)-1].name = tail[1:end]
				tail = tail[end+1:]
			}
		}
		appendPart(tail)
	}

	if len(parts) != len(vars) + 1 {
		panic("number of template parts doesn't match template vars")
	}

	loc := qp.parent.Loc
	location := logger.LocationOrNil(qp.definedSource, logger.Range{Loc: loc})
	var definedAt sourceLocation
	if location != nil {
		definedAt.Line = uint32(location.Line)
		definedAt.File = location.File
	}

	return &query{
		queryPart: qp,
		parts:     parts,
		vars:      vars,
		IsPublic:  true,
		isFragment: qp.isFragment,
		DefinedAt: definedAt,
	}, nil
}

const errQueryAsQueryPart = "cannot use a query (created with sql``) as a query part: use sql.p`` instead"

func (q *query) compile(c *FlowStateCompiler, analyzer *FlowStateAnalyzer, allQueries map[js_ast.Ref]queriesByWhitelistOrder) bool {
	if q.Hash != "" {
		return true // already compiled
	}
	c.addQueryUsageLocations(q)

	// The hash of the query should be the hash of the query parts
	// with the name of the server params and the hashes of any query fragments, if any.
	// Regular vars and params are excluded as they're just placeholders
	// and their content doesn't change the query.

	h := sha256.New()
Outer:
	for i := 0; i < len(q.vars); i++ {
		v := &q.vars[i]
		switch v.ty {
		case queryVarTypeVar:
			// This could be a fragment, see if we can trace it back to a query
			v.fragments = c.findQueryForExpr(nil, allQueries, v.expr)
			inline := len(v.fragments) == 1
			for j := range v.fragments {
				v.ty = queryVarTypeFragment
				fragment := v.fragments[j]
				if !fragment.isFragment {
					c.log.AddError(q.definedSource, q.parent.Loc, errQueryAsQueryPart)
					return false
				}
				fragment.ServerReferences += q.ServerReferences
				fragment.ClientReferences += q.ClientReferences
				if inline {
					fragment.inlinedServerCount += q.ServerReferences
					fragment.inlinedClientCount += q.ClientReferences
					q.insert(i, fragment)
					i--
					continue Outer // we replaced this index with the fragment parts/vars, so reprocess this
				}
				if !fragment.compile(c, analyzer, allQueries) {
					return false
				}
				h.Write(fragment.binHash[:])
			}
		case queryVarTypeServer:
			q.IsPublic = false
			h.Write([]byte(v.name))
		}
		h.Write([]byte(q.parts[i]))
	}
	h.Write([]byte(q.parts[len(q.parts)-1]))
	sum := h.Sum(q.binHash[:0])

	q.Hash = base64.RawURLEncoding.EncodeToString(sum[:30])
	return true
}

func (q *query) insert(index int, fragment *query) {
	// Insert vars from fragment, replacing q.vars[index] with fragment.vars
	// Note if fragment.vars is empty, we're removing it and reducing len(vars) by 1.
	// Replacing a var means the parts before and after the var should be combined.
	// Additionally we're adding N vars, and N+1 parts, which would mean we go from M-1 vars and M parts
	// to N+M-1 vars and N+1+M parts, which now means there's two more parts than vara, breaking the invariant.
	// So we also have to append N.parts[0] to M.parts[index].
	q.parts[index] += fragment.parts[0] + q.parts[index+1]

	tVars := q.vars[index+1:]
	tParts := q.parts[index+2:]

	if len(fragment.vars) != 0 {
		q.vars = append(q.vars[:index], fragment.vars...)
	} else {
		// Concatenate parts[index] with parts[index+1]
		q.vars = q.vars[:index]
	}
	q.vars = append(q.vars, tVars...)
	q.parts = append(q.parts[:index+1], fragment.parts[1:]...)
	q.parts = append(q.parts, tParts...)

	if !fragment.IsPublic {
		q.IsPublic = false
	}

	if len(q.parts) != len(q.vars) + 1 {
		panic("number of template parts doesn't match template vars")
	}
}

func (q *query) isReachable() bool {
	return q.ClientReferences != 0 || q.ServerReferences != 0
}

func (q *query) isInlined() bool {
	return q.inlinedClientCount != 0 || q.inlinedServerCount != 0
}

type queriesByType []query

func (a queriesByType) Len() int           { return len(a) }
func (a queriesByType) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a queriesByType) Less(i, j int) bool {
	return a[i].Type < a[j].Type || (a[i].Type == a[j].Type && a[i].Hash < a[j].Hash)
}
