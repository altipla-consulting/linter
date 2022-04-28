package customrules

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"github.com/mgechev/revive/lint"
	"libs.altipla.consulting/collections"
)

func isCgoExported(f *ast.FuncDecl) bool {
	if f.Recv != nil || f.Doc == nil {
		return false
	}

	cgoExport := regexp.MustCompile(fmt.Sprintf("(?m)^//export %s$", regexp.QuoteMeta(f.Name.Name)))
	for _, c := range f.Doc.List {
		if cgoExport.MatchString(c.Text) {
			return true
		}
	}
	return false
}

var allCapsRE = regexp.MustCompile(`^[A-Z0-9_]+$`)

// VarNamingRule lints given else constructs.
type VarNamingRule struct{}

// Apply applies the rule to given file.
func (r *VarNamingRule) Apply(file *lint.File, arguments lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	var whitelist []string
	var blacklist []string

	if len(arguments) >= 1 {
		whitelist = getList(arguments[0], "whitelist")
	}

	if len(arguments) >= 2 {
		blacklist = getList(arguments[1], "blacklist")
	}

	walker := lintNames{
		file:      file,
		fileAst:   file.AST,
		whitelist: whitelist,
		blacklist: blacklist,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}
	ast.Walk(&walker, file.AST)

	return failures
}

// Name returns the rule name.
func (r *VarNamingRule) Name() string {
	return "var-naming"
}

func checkList(fl *ast.FieldList, thing string, w *lintNames) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		for _, id := range f.Names {
			check(id, thing, w)
		}
	}
}

func check(id *ast.Ident, thing string, w *lintNames) {
	if id.Name == "_" {
		return
	}
	if collections.HasString(w.whitelist, id.Name) {
		return
	}

	// Handle two common styles from other languages that don't belong in Go.
	if len(id.Name) >= 5 && allCapsRE.MatchString(id.Name) && strings.Contains(id.Name, "_") {
		w.onFailure(lint.Failure{
			Failure:    "don't use ALL_CAPS in Go names; use CamelCase",
			Confidence: 0.8,
			Node:       id,
			Category:   "naming",
		})
		return
	}
	if len(id.Name) > 2 && id.Name[0] == 'k' && id.Name[1] >= 'A' && id.Name[1] <= 'Z' {
		should := string(id.Name[1]+'a'-'A') + id.Name[2:]
		w.onFailure(lint.Failure{
			Failure:    fmt.Sprintf("don't use leading k in Go names; %s %s should be %s", thing, id.Name, should),
			Confidence: 0.8,
			Node:       id,
			Category:   "naming",
		})
	}

	should := lint.Name(id.Name, w.whitelist, w.blacklist)
	if id.Name == should {
		return
	}

	if len(id.Name) > 2 && strings.Contains(id.Name[1:], "_") {
		w.onFailure(lint.Failure{
			Failure:    fmt.Sprintf("don't use underscores in Go names; %s %s should be %s", thing, id.Name, should),
			Confidence: 0.9,
			Node:       id,
			Category:   "naming",
		})
		return
	}
	w.onFailure(lint.Failure{
		Failure:    fmt.Sprintf("%s %s should be %s", thing, id.Name, should),
		Confidence: 0.8,
		Node:       id,
		Category:   "naming",
	})
}

type lintNames struct {
	file                   *lint.File
	fileAst                *ast.File
	lastGen                *ast.GenDecl
	genDeclMissingComments map[*ast.GenDecl]bool
	onFailure              func(lint.Failure)
	whitelist              []string
	blacklist              []string
}

func (w *lintNames) Visit(n ast.Node) ast.Visitor {
	switch v := n.(type) {
	case *ast.AssignStmt:
		if v.Tok == token.ASSIGN {
			return w
		}
		for _, exp := range v.Lhs {
			if id, ok := exp.(*ast.Ident); ok {
				check(id, "var", w)
			}
		}
	case *ast.FuncDecl:
		if w.file.IsTest() && (strings.HasPrefix(v.Name.Name, "Example") || strings.HasPrefix(v.Name.Name, "Test") || strings.HasPrefix(v.Name.Name, "Benchmark")) {
			return w
		}

		thing := "func"
		if v.Recv != nil {
			thing = "method"
		}

		// Exclude naming warnings for functions that are exported to C but
		// not exported in the Go API.
		// See https://github.com/golang/lint/issues/144.
		if ast.IsExported(v.Name.Name) || !isCgoExported(v) {
			check(v.Name, thing, w)
		}

		checkList(v.Type.Params, thing+" parameter", w)
		checkList(v.Type.Results, thing+" result", w)
	case *ast.GenDecl:
		if v.Tok == token.IMPORT {
			return w
		}
		var thing string
		switch v.Tok {
		case token.CONST:
			thing = "const"
		case token.TYPE:
			thing = "type"
		case token.VAR:
			thing = "var"
		}
		for _, spec := range v.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				check(s.Name, thing, w)
			case *ast.ValueSpec:
				for _, id := range s.Names {
					check(id, thing, w)
				}
			}
		}
	case *ast.InterfaceType:
		// Do not check interface method names.
		// They are often constrainted by the method names of concrete types.
		for _, x := range v.Methods.List {
			ft, ok := x.Type.(*ast.FuncType)
			if !ok { // might be an embedded interface name
				continue
			}
			checkList(ft.Params, "interface method parameter", w)
			checkList(ft.Results, "interface method result", w)
		}
	case *ast.RangeStmt:
		if v.Tok == token.ASSIGN {
			return w
		}
		if id, ok := v.Key.(*ast.Ident); ok {
			check(id, "range var", w)
		}
		if id, ok := v.Value.(*ast.Ident); ok {
			check(id, "range var", w)
		}
	case *ast.StructType:
		for _, f := range v.Fields.List {
			for _, id := range f.Names {
				check(id, "struct field", w)
			}
		}
	}
	return w
}

func getList(arg interface{}, argName string) []string {
	temp, ok := arg.([]interface{})
	if !ok {
		panic(fmt.Sprintf("Invalid argument to the var-naming rule. Expecting a %s of type slice with initialisms, got %T", argName, arg))
	}
	var list []string
	for _, v := range temp {
		if val, ok := v.(string); ok {
			list = append(list, val)
		} else {
			panic(fmt.Sprintf("Invalid %s values of the var-naming rule. Expecting slice of strings but got element of type %T", val, arg))
		}
	}
	return list
}
