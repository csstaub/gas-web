// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package core holds the central scanning logic used by GAS
package core

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"reflect"
	"strings"
)

// The Context is populated with data parsed from the source code as it is scanned.
// It is passed through to all rule functions as they are called. Rules may use
// this data in conjunction withe the encoutered AST node.
type Context struct {
	FileSet  *token.FileSet
	Comments ast.CommentMap
	Info     *types.Info
	Pkg      *types.Package
	Root     *ast.File
	Config   map[string]interface{}
}

// The Rule interface used by all rules supported by GAS.
type Rule interface {
	Match(ast.Node, *Context) (*Issue, error)
}

// A RuleSet maps lists of rules to the type of AST node they should be run on.
// The anaylzer will only invoke rules contained in the list associated with the
// type of AST node it is currently visiting.
type RuleSet map[reflect.Type][]Rule

// Metrics used when reporting information about a scanning run.
type Metrics struct {
	NumFiles int `json:"files"`
	NumLines int `json:"lines"`
	NumNosec int `json:"nosec"`
	NumFound int `json:"found"`
}

// The Analyzer object is the main object of GAS. It has methods traverse an AST
// and invoke the correct checking rules as on each node as required.
type Analyzer struct {
	ignoreNosec bool
	ruleset     RuleSet
	context     Context
	logger      *log.Logger
	Issues      []Issue `json:"issues"`
	Stats       Metrics `json:"metrics"`
}

// NewAnalyzer builds a new anaylzer.
func NewAnalyzer(conf map[string]interface{}, logger *log.Logger) Analyzer {
	if logger == nil {
		logger = log.New(os.Stdout, "[gas]", 0)
	}
	a := Analyzer{
		ignoreNosec: conf["ignoreNosec"].(bool),
		ruleset:     make(RuleSet),
		Issues:      make([]Issue, 0),
		context:     Context{token.NewFileSet(), nil, nil, nil, nil, nil},
		logger:      logger,
	}

	// TODO(tkelsey): use the inc/exc lists

	return a
}

func (gas *Analyzer) process(filename string, source interface{}) error {
	mode := parser.ParseComments
	root, err := parser.ParseFile(gas.context.FileSet, filename, source, mode)
	if err == nil {
		gas.context.Comments = ast.NewCommentMap(gas.context.FileSet, root, root.Comments)
		gas.context.Root = root

		// here we get type info
		gas.context.Info = &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
			Scopes:     make(map[ast.Node]*types.Scope),
			Implicits:  make(map[ast.Node]types.Object),
		}

		conf := types.Config{Importer: importer.Default()}
		gas.context.Pkg, _ = conf.Check("pkg", gas.context.FileSet, []*ast.File{root}, gas.context.Info)
		if err != nil {
			gas.logger.Println("failed to check imports")
			return err
		}

		ast.Walk(gas, root)
		gas.Stats.NumFiles++
	}
	return err
}

// AddRule adds a rule into a rule set list mapped to the given AST node's type.
// The node is only needed for its type and is not otherwise used.
func (gas *Analyzer) AddRule(r Rule, n ast.Node) {
	t := reflect.TypeOf(n)
	if val, ok := gas.ruleset[t]; ok {
		gas.ruleset[t] = append(val, r)
	} else {
		gas.ruleset[t] = []Rule{r}
	}
}

// Process reads in a source file, convert it to an AST and traverse it.
// Rule methods added with AddRule will be invoked as necessary.
func (gas *Analyzer) Process(filename string) error {
	err := gas.process(filename, nil)
	fun := func(f *token.File) bool {
		gas.Stats.NumLines += f.LineCount()
		return true
	}
	gas.context.FileSet.Iterate(fun)
	return err
}

// ProcessSource will convert a source code string into an AST and traverse it.
// Rule methods added with AddRule will be invoked as necessary. The string is
// identified by the filename given but no file IO will be done.
func (gas *Analyzer) ProcessSource(filename string, source string) error {
	err := gas.process(filename, source)
	fun := func(f *token.File) bool {
		gas.Stats.NumLines += f.LineCount()
		return true
	}
	gas.context.FileSet.Iterate(fun)
	return err
}

// ignore a node (and sub-tree) if it is tagged with a "nosec" comment
func (gas *Analyzer) ignore(n ast.Node) bool {
	if groups, ok := gas.context.Comments[n]; ok && !gas.ignoreNosec {
		for _, group := range groups {
			if strings.Contains(group.Text(), "nosec") {
				gas.Stats.NumNosec++
				return true
			}
		}
	}
	return false
}

// Visit runs the GAS visitor logic over an AST created by parsing go code.
// Rule methods added with AddRule will be invoked as necessary.
func (gas *Analyzer) Visit(n ast.Node) ast.Visitor {
	if !gas.ignore(n) {
		if val, ok := gas.ruleset[reflect.TypeOf(n)]; ok {
			for _, rule := range val {
				ret, err := rule.Match(n, &gas.context)
				if err != nil {
					// will want to give more info than this ...
					gas.logger.Println("internal error running rule:", err)
				}
				if ret != nil {
					gas.Issues = append(gas.Issues, *ret)
					gas.Stats.NumFound++
				}
			}
		}
		return gas
	}
	return nil
}
