package main

import (
	"flag"
	"io"
	"os"
	"regexp"
	// go internal
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	// go tools
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/packages"
)

type Package struct {
	Path      string
	Fset      *token.FileSet
	Files     []*ast.File
	TypesInfo *types.Info
}

type Context struct {
	Packages map[string]*Package
	Package  *Package
	Indent   int
	Writer   io.Writer
}

func NewContext(w io.Writer) *Context {
	return &Context{
		Packages: make(map[string]*Package),
		Package:  nil,
		Indent:   -1,
		Writer:   w,
	}
}

func (c *Context) IncIndent() *Context {
	return &Context{
		Packages: c.Packages,
		Package:  c.Package,
		Indent:   c.Indent + 1,
		Writer:   c.Writer,
	}
}

func (c *Context) DecIndent() *Context {
	return &Context{
		Packages: c.Packages,
		Package:  c.Package,
		Indent:   c.Indent - 1,
		Writer:   c.Writer,
	}
}

func (c *Context) StartStruct(p *Package) *Context {
	return &Context{
		Packages: c.Packages,
		Package:  p,
		Indent:   c.Indent + 1,
		Writer:   c.Writer,
	}
}

func (c *Context) WriteLn(line string) {
	for i := 0; i < c.Indent; i++ {
		c.Writer.Write([]byte{' ', ' '})
	}
	c.Writer.Write([]byte(line))
	c.Writer.Write([]byte{'\n'})
}

func Field_GetType(p *Package, f *ast.Field) types.Type {
	var unwrap func(t ast.Expr) *ast.Ident

	// `p.TypesInfo.Defs` maps `*ast.Ident` to `types.Type`. Sometimes `f.type`
	// is encapsulated by an "intermediate" type such as "star expression" or
	// "selector expression". Fortunately, these can be unwrapped to reveal
	// the underlying `*ast.Ident`.
	unwrap = func(t ast.Expr) *ast.Ident {
		switch x := t.(type) {
		case *ast.SelectorExpr:
			return unwrap(x.Sel)
		case *ast.StarExpr:
			return unwrap(x.X)
		case *ast.Ident:
			return x
		}
		return nil
	}

	if len(f.Names) > 0 {
		return p.TypesInfo.Defs[f.Names[0]].Type()
	} else {
		return p.TypesInfo.Defs[unwrap(f.Type)].Type()
	}
}

func Type_IsArray(t types.Type) bool {
	// In most circumstances the "intermediate" types matter. However,
	// `toml` automatically constructs the intermediates when parsing.
	// For instance, if a struct contains a field of type `*string`,
	// `toml` can initialize it from a plain-old "string". We will
	// ignore the intermediates when appropriate.
	switch x := t.(type) {
	case *types.Pointer:
		return Type_IsArray(x.Elem())
	case *types.Slice:
		return true
	}
	return false
}

func Type_IsStruct(t types.Type) bool {
	x := Type_GetBasal(t)

	if _, ok := x.(*types.Struct); ok {
		return true
	}
	if _, ok := x.Underlying().(*types.Struct); ok {
		return true
	}
	return false
}

func Type_GetBasal(t types.Type) types.Type {
	// In most circumstances the "intermediate" types matter. However,
	// `toml` automatically constructs the intermediates when parsing.
	// For instance, if a struct contains a field of type `*string`,
	// `toml` can initialize it from a plain-old "string". We will
	// ignore the intermediates when appropriate.
	switch x := t.(type) {
	case *types.Pointer:
		return Type_GetBasal(x.Elem())
	case *types.Slice:
		return Type_GetBasal(x.Elem())
	}
	return t
}

func Type_GetNamed(t types.Type) *types.Named {
	x, ok := Type_GetBasal(t).(*types.Named)
	if !ok {
		return nil
	}
	return x
}

func Type_LoadPackage(context *Context, t *types.Named) *Package {
	path := t.Obj().Pkg().Path()

	if p, ok := context.Packages[path]; ok {
		return p
	}

	config := loader.Config{
		ParserMode: parser.ParseComments,
	}
	config.Import(path)
	prog, err := config.Load()
	if err != nil {
		panic(err)
	}

	for _, pkgInfo := range prog.InitialPackages() {
		context.Packages[path] = &Package{
			Fset:      prog.Fset,
			Files:     pkgInfo.Files,
			Path:      pkgInfo.Pkg.Path(),
			TypesInfo: &pkgInfo.Info,
		}
	}

	p, _ := context.Packages[path]
	return p
}

func WriteFieldStruct(context *Context, f *ast.Field) {
	t := Field_GetType(context.Package, f)

	n := Type_GetNamed(t)
	if n == nil {
		return
	}

	x := Type_LoadPackage(context, n)
	if x == nil {
		return
	}

	ProcessStruct(context.StartStruct(x), n.Obj().Name())
}

var regex_comment = regexp.MustCompile("^\\/\\/(.*)")
var regex_flag_unc = regexp.MustCompile("^\\s*!td:unc\\s?(.*)")
var regex_flag_skip = regexp.MustCompile("^\\s*!td:skip\\s*$")
var regex_flag_follow = regexp.MustCompile("^\\s*!td:follow\\s*$")

func WriteComment(c *Context, g *ast.CommentGroup) {
	has_output := false
	for _, l := range g.List {
		// The current approach supports single-line comments. There is
		// ambiguity regarding line prefixes and whitespace when parsing
		// block comments. Since single-line comments are consistent, it
		// avoids this ambiguity.
		m := regex_comment.FindStringSubmatch(l.Text)
		if m == nil {
			continue
		}

		// don't write flag
		if f := regex_flag_skip.FindStringSubmatch(m[1]); f != nil {
			continue
		}
		// don't write flag
		if f := regex_flag_follow.FindStringSubmatch(m[1]); f != nil {
			continue
		}

		// Do not emit a "#" when the line is intended to be uncommented.
		// This is useful for supplying default values.
		ug := regex_flag_unc.FindStringSubmatch(m[1])
		if ug != nil {
			has_output = true
			c.WriteLn(ug[1])
		} else {
			for i, _ := range m[1] {
				if i+1 >= len(m[1]) ||
					m[1][i+0] != '/' ||
					m[1][i+1] != '/' {
					break
				}
				// Rewrite the line such that additional "//" is converted
				// into "#". If any other character (including whitespace)
				// is encountered, stop rewriting the string. This makes
				// it possible to have section headings.
				m[1] = m[1][:i] + "#" + m[1][i+2:]
			}
			has_output = true
			c.WriteLn("#" + m[1])
		}
	}

	if has_output {
		c.WriteLn("")
	}
}

const (
	TD_NONE = 0
	// The default behavior is to write the documentation for all
	// exported fields. Sometimes this behavior is undesirable.
	// Enabling this flag with "!td:skip" skips writing
	// documentation for the current field.
	TD_SKIP = 1
	// Structures are not automatically followed. This prevents generating
	// documentation for types from external packages. This "opt-in"
	// behavior also prevents infinite recursion.
	TD_FOLLOW = 2
)

func ParseCommentFlags(g *ast.CommentGroup) int {
	flags := TD_NONE
	for _, l := range g.List {
		// The current approach supports single-line comments. There is
		// ambiguity regarding line prefixes and whitespace when parsing
		// block comments. Since single-line comments are consistent, it
		// avoids this ambiguity.
		m := regex_comment.FindStringSubmatch(l.Text)
		if m == nil {
			continue
		}

		if f := regex_flag_skip.FindStringSubmatch(m[1]); f != nil {
			flags = flags | TD_SKIP
		}

		if f := regex_flag_follow.FindStringSubmatch(m[1]); f != nil {
			flags = flags | TD_FOLLOW
		}
	}

	return flags
}

const (
	TD_STRUCT       = 0 // generates "[NAME]"
	TD_STRUCT_ARRAY = 1 // generates "[[NAME]]"
)

func WriteStructHeader(c *Context, name string, t int) {
	switch t {
	case TD_STRUCT:
		c.WriteLn("[" + name + "]")
	case TD_STRUCT_ARRAY:
		c.WriteLn("[[" + name + "]]")
	}
}

func ProcessField(context *Context, f *ast.Field) {
	t := Field_GetType(context.Package, f)

	flags := TD_NONE
	if f.Doc != nil {
		flags = ParseCommentFlags(f.Doc)
	}

	if flags&TD_SKIP > 0 {
		return
	}

	if f.Doc != nil {
		WriteComment(context, f.Doc)
	}

	if len(f.Names) > 0 {
		if ast.IsExported(f.Names[0].Name) {
			// The struct field is not "anonymous". However, the field type
			// is an "anonymous" struct. :-) These are rendered in the same
			// fashion as inlined non-"anonymous" structs. However, these
			// are parsed in a different manner.
			if s, ok := f.Type.(*ast.StructType); ok {
				WriteStructHeader(context, f.Names[0].Name, TD_STRUCT)
				for _, f := range s.Fields.List {
					ProcessField(context.IncIndent(), f)
				}
			} else {
				if Type_IsStruct(t) && (flags&TD_FOLLOW != 0) {
					if Type_IsArray(t) {
						WriteStructHeader(context, f.Names[0].Name, TD_STRUCT_ARRAY)
						WriteFieldStruct(context, f)
					} else {
						WriteStructHeader(context, f.Names[0].Name, TD_STRUCT)
						WriteFieldStruct(context, f)
					}
				}
			}
		}
	} else {
		// `toml` requires that "anonymous" fields be structs. Otherwise,
		// the field is ignored completely. `toml` requires all fields to
		// be exported. "anonymous" fields are not exported. However, a
		// struct may contain an exported "named" field.
		if Type_IsStruct(t) && (flags&TD_FOLLOW != 0) {
			WriteFieldStruct(context.DecIndent(), f)
		}
	}

}

func ProcessStruct(context *Context, s_name string) {
	for _, f := range context.Package.Files {
		ast.Inspect(f, func(node ast.Node) bool {
			decl, ok := node.(*ast.GenDecl)
			if !ok {
				return true
			}

			if len(decl.Specs) < 1 {
				return false
			}

			spec, ok := decl.Specs[0].(*ast.TypeSpec)
			if !ok {
				return false
			}

			s, ok := spec.Type.(*ast.StructType)
			if !ok {
				return false
			}

			if spec.Name.Name != s_name {
				return false
			}

			if decl.Doc != nil {
				WriteComment(context, decl.Doc)
			}
			for _, f := range s.Fields.List {
				ProcessField(context, f)
			}

			return true
		})
	}
}

func main() {
	var ft = flag.String("t", "", "(Required) Specifies the target structure.")
	var fo = flag.String("o", "",
		`This specifies the output path for the generated toml document.
It is recommended that the location is in the same directory or
a subdirectory of the current package. This option is
required.`)
	var fp = flag.String("p", "",
		`This specifies the target package. This is only required when
"$GOPACKAGE" is not present in the environment. Therefore, this
can be ignored when used as a "go generate" directive. If this
is supplied while "$GOPACKAGE" is present, this takes priority.`)
	flag.Parse()

	ep := os.Getenv("GOPACKAGE")

	// Ensures that the "target" and "output" parameters are always
	// specified. This conditionally ensures that either the
	// parameters or the environment specify the "package"
	// parameter.
	if (*ft == "" || *fo == "") || (*fp == "" && ep == "") {
		flag.Usage()
		os.Exit(1)
	}

	p_target := ep
	if *fp != "" {
		p_target = *fp
	}

	// Attempt to load information about packages from the source code
	// located in the current working directory. This does not include
	// information from external packages. If an external package is
	// required to build metadata for the target struct, it should
	// be download and available in GOROOT.
	found, err := packages.Load(
		&packages.Config{
			Mode: packages.NeedName |
				packages.NeedTypes |
				packages.NeedTypesInfo |
				packages.NeedSyntax |
				packages.NeedImports |
				packages.NeedFiles,
			Tests: false,
		},
	)
	if err != nil {
		panic(err)
	}

	f, err := os.Create(*fo)
	if err != nil {
		panic(err)
	}

	context := NewContext(f)
	for _, p := range found {
		// "golang.org/x/tools/go/packages" and "golang.org/x/tools/go/loader"
		// use different type definitions to represent packages. Instead of
		// implementing functions to handle each representation of the same
		// data, convert the data to our own representation.
		context.Packages[p.PkgPath] =
			&Package{
				Path:      p.PkgPath,
				Fset:      p.Fset,
				Files:     p.Syntax,
				TypesInfo: p.TypesInfo,
			}
	}

	if p, ok := context.Packages[p_target]; ok {
		for _, d := range p.TypesInfo.Defs {
			if d == nil {
				continue
			}

			// `p.TypesInfo.Defs` maps names to definitions. This can include
			// both variables and type-names. `types.TypeName` represents the
			// type-name for either "defined" or "aliased" types. This
			// ensures variables are not considered.
			//
			// Do not use aliases! While is is possible to locate the type-
			// definition for a type-alias (and then find the corresponding
			// `ast.Node`), this functionality is not implemented. Just use
			// the original struct name.
			if _, ok := d.(*types.TypeName); !ok {
				continue
			}

			// ensure that the `d` is struct type definition.
			if _, ok := d.Type().Underlying().(*types.Struct); !ok {
				continue
			}

			if d.Name() != *ft {
				continue
			}

			ProcessStruct(context.StartStruct(p), *ft)
		}
	}
}
