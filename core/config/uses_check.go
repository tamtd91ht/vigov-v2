package config

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// CheckUses compares a main's declaration with what its package actually reads. It returns one
// line per problem, empty when the two agree:
//
//   - a Config accessor read in dir whose group uses does not declare — in a pod that is a panic
//     at startup (Config.require); here it is a red test before anything is deployed;
//   - a declared group nothing in dir reads — in staging and prod Load would refuse to start
//     without variables this service never uses.
//
// Every main calls it from a test (`config.CheckUses(".", configUses)`), which is what makes an
// omission in the declaration red rather than a production incident.
//
// HOW IT KNOWS WHICH GROUP AN ACCESSOR NEEDS: it does not keep a table. It parses the non-test Go
// files of dir, collects every selector naming a zero-argument method of Config, and CALLS that
// method on a Config that declares nothing. The accessor's own require() panics with the groups it
// accepts — so the guard in accessors.go is the single source of truth, and a second table that
// could drift from it does not exist.
//
// WHAT IT CANNOT SEE: a Config handed to another package, which then reads it there. No package
// takes a whole Config today (core/storage and core/malwarescan take their own group's struct);
// one that starts to is still caught at runtime by require, at the first startup in any
// environment.
func CheckUses(dir string, uses Usage) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config.CheckUses: %w", err)
	}
	fset := token.NewFileSet()
	var problems []string
	read := map[Group]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("config.CheckUses: %w", err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			groups, guarded := accessorGroups(sel.Sel.Name)
			if !guarded {
				return true
			}
			for _, g := range groups {
				read[g] = true
			}
			if !uses.hasAny(groups) {
				problems = append(problems, fmt.Sprintf("%s: reads Config.%s() but config.Uses declares none of %s",
					fset.Position(sel.Pos()), sel.Sel.Name, joinGroups(groups)))
			}
			return true
		})
	}
	for _, g := range uses.Groups() {
		if !read[g] {
			problems = append(problems, fmt.Sprintf("config.Uses declares %s but nothing in %s reads it — "+
				"in staging/prod the service would refuse to start without variables it never uses", g, dir))
		}
	}
	sort.Strings(problems)
	return problems, nil
}

// accessorGroups reports the groups a zero-argument Config method requires, by calling it on a
// Config that declares nothing. guarded is false for a name that is not such a method, and for an
// unguarded method (CanhBao, Redacted, the renderers).
func accessorGroups(name string) (groups []Group, guarded bool) {
	m := reflect.ValueOf(Config{service: "config.CheckUses"}).MethodByName(name)
	if !m.IsValid() || m.Type().NumIn() != 0 {
		return nil, false
	}
	defer func() {
		if p := recover(); p != nil {
			u, ok := p.(undeclaredRead)
			if !ok {
				panic(p)
			}
			groups, guarded = u.groups, true
		}
	}()
	m.Call(nil)
	return nil, false
}
