package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/tools/go/buildutil"
)

var importRegexp *regexp.Regexp

type StructTypeVisitor struct {
	structTypes []*ast.StructType
}

func (v *StructTypeVisitor) Visit(n ast.Node) (w ast.Visitor) {
	structType, ok := n.(*ast.StructType)
	if !ok {
		return v
	}
	for _, field := range structType.Fields.List {
		if field.Names != nil {
			continue
		}
		starExpr, isPtr := field.Type.(*ast.StarExpr)
		if !isPtr {
			continue
		}
		ident, isIdent := starExpr.X.(*ast.Ident)
		if !isIdent {
			continue
		}
		if ast.IsExported(ident.Name) {
			continue
		}
		v.structTypes = append(v.structTypes, structType)
	}
	return v
}

func searchStructs(astFile *ast.File) []*ast.StructType {
	var visitor StructTypeVisitor
	ast.Walk(&visitor, astFile)
	return visitor.structTypes
}

func processPackage(importPath string, err error) {
	if !importRegexp.MatchString(importPath) {
		return
	}
	if err != nil {
		if _, noGoErr := err.(*build.NoGoError); !noGoErr {
			fmt.Println(err)
		}
		return
	}
	pkg, err := build.Import(importPath, "", 0)
	if err != nil {
		if _, noGoErr := err.(*build.NoGoError); !noGoErr {
			fmt.Println(err)
		}
		return
	}

	fset := token.NewFileSet()
	var visitor StructTypeVisitor
	var pkgFiles []string
	pkgFiles = append(pkgFiles, pkg.GoFiles...)
	pkgFiles = append(pkgFiles, pkg.TestGoFiles...)
	pkgFiles = append(pkgFiles, pkg.XTestGoFiles...)
	pkgFiles = append(pkgFiles, pkg.CgoFiles...)
	if len(pkgFiles) > 0 {
		filename := pkgFiles[0]
		path := filepath.Join(pkg.Dir, filename)
		astFile, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			fmt.Println(err)
			fmt.Println("aborting package")
			return
		}
		ast.Walk(&visitor, astFile)
		for _, structType := range visitor.structTypes {
			fmt.Printf("\n%v:\n", fset.PositionFor(structType.Pos(), false))
			err := printer.Fprint(os.Stdout, fset, structType)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println()
		}
	}
}

func main() {
	importPattern := flag.String("importPattern", ".*", "only check packages with import path matching regex pattern")
	flag.Parse()
	var err error
	importRegexp, err = regexp.Compile(*importPattern)
	if err != nil {
		log.Fatal(err)
	}
	buildutil.ForEachPackage(&build.Default, processPackage)
}
