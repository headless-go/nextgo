package codegen

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
	"golang.org/x/tools/go/packages"

	"github.com/headless-go/nextgo"
)

// ObjectCache is a lazily evaluated mapping of objects to Wire structures.
type ObjectCache struct {
	Packages map[string]*packages.Package
}

func NewObjectCache(pkgs []*packages.Package) *ObjectCache {
	if len(pkgs) == 0 {
		panic("object cache must have Packages to draw from")
	}
	oc := &ObjectCache{
		Packages: make(map[string]*packages.Package),
	}
	// Depth-first search of all dependencies to gather import path to
	// Packages.Package mapping. go/Packages guarantees that for a single
	// call to Packages.Load and an import path X, there will exist only
	// one *Packages.Package value with PkgPath X.
	stk := append([]*packages.Package(nil), pkgs...)
	for len(stk) > 0 {
		p := stk[len(stk)-1]
		stk = stk[:len(stk)-1]
		if oc.Packages[p.PkgPath] != nil {
			continue
		}
		oc.Packages[p.PkgPath] = p
		for _, imp := range p.Imports {
			stk = append(stk, imp)
		}
	}
	return oc
}

func (oc *ObjectCache) ObjectOf(o *ast.Ident) types.Object {

	for _, p := range oc.Packages {
		if t := p.TypesInfo.ObjectOf(o); t != nil {
			return t
		}
	}
	return nil
}

func unquote(str string) string {
	if s, err := strconv.Unquote(str); err == nil {
		return s
	}
	return str
}

type node struct {
	*HandleFunc
	*Mapping
}

func (n *node) Pos() token.Position {
	if n.HandleFunc != nil {
		return n.HandleFunc.Pos
	}
	return n.Mapping.pos
}

type API struct {
	Handlers    []HandleFunc
	Annotations []Mapping
	Schemas     map[string]spec.Schema
}

type Parser struct {
	packages *packages.Package
	fset     *token.FileSet
	objCache *ObjectCache
	api      *API
	errs     []error
}

// parseMappingWith 解析 mapping.Mapping() 调用
func (p *Parser) parseMappingWith(node *ast.GenDecl) error {

	if node.Tok != token.VAR {
		return nil
	}

	for _, spec := range node.Specs {
		switch n := spec.(type) {
		case *ast.ValueSpec:
			for _, v := range n.Values {
				with := Mapping{}
				with.pos = p.fset.Position(v.Pos())
				p.parseMappingWithCallExpr(v, &with)
				if with.Scope != "" {
					p.api.Annotations = append(p.api.Annotations, with)
				}
			}
		}
	}
	return nil
}

func (p *Parser) isMappingWithCallExpr(sel *ast.SelectorExpr) bool {

	obj := p.objCache.ObjectOf(sel.Sel)
	if obj != nil {
		return reflect.TypeOf(nextgo.Mapping).PkgPath() == obj.Pkg().Path()
	}
	return false
}

func (p *Parser) parseMappingWithCallExpr(expr ast.Expr, with *Mapping) (ok bool) {

	callExpr, ok := expr.(*ast.CallExpr)
	if !ok {
		switch t := expr.(type) {
		case *ast.SelectorExpr:
			switch t.Sel.Name {
			case "Mapping", "MappingFile":
				with.Scope = t.Sel.Name
			}
		}
		return
	}

	switch t := callExpr.Fun.(type) {
	case *ast.SelectorExpr:
		if !p.isMappingWithCallExpr(t) {
			return
		}
		switch t.Sel.Name {
		case "Label":
			with.Label = p.parseMappingLabel(callExpr.Args)
		case "Middleware":
			with.Middleware = p.parseMappingMiddleware(callExpr.Args)
		case "StatusCode":
			with.HttpCode = p.parseMappingHttpCode(callExpr.Args)
		case "PathPrefix":
			with.PathPrefix = true
		case "HttpMethod":
			with.HttpMethod = p.parseMappingHttpMethod(callExpr.Args)
		case "BindQuery":
			with.BindQuery = p.parseMappingBind(callExpr.Args)
		case "BindHeader":
			with.BindHeader = p.parseMappingBind(callExpr.Args)
		}
		p.parseMappingWithCallExpr(t.X, with)
	}
	return true
}

func (p *Parser) parseMappingLabel(args []ast.Expr) map[string]string {

	labels := map[string]string{}
	for _, arg := range p.mustArgsToString(args) {
		if key, value, ok := strings.Cut(strings.Trim(arg, "\""), "="); ok {
			labels[key] = value
		}
	}
	return labels
}

func (p *Parser) parseMappingHttpMethod(args []ast.Expr) string {

	for _, arg := range p.mustArgsToString(args) {
		return arg
	}
	p.AddErr(args[0], "unexpected HttpMethod arg: %v", args[0])
	return ""
}

func toType(obj types.Object) Type {

	t := Type{
		Name: obj.Name(),
	}
	t.FullName = t.Name
	t.PackageName = t.Name
	if p := obj.Pkg(); p != nil {
		t.PackageItem = PackageItem{
			Name: obj.Pkg().Name(),
			Path: obj.Pkg().Path(),
		}
		t.FullName = p.Path() + "." + t.Name
		t.PackageName = p.Name() + "." + t.Name
	}
	return t
}

func (p *Parser) parseMappingBind(args []ast.Expr) []Type {
	var bind []Type

	f := func(t *ast.Ident) Type {
		obj := p.objCache.ObjectOf(t)
		return toType(obj)
	}
	for _, v := range args {
		switch t := v.(type) {
		case *ast.CompositeLit:
			if i, ok := t.Type.(*ast.SelectorExpr); ok {
				bind = append(bind, f(i.Sel))
				continue
			}
			if i, ok := t.Type.(*ast.Ident); !ok {
				p.AddErr(v, "unexpected Mapping Bind arg: %v, must format like foo.Foo{}", v)
			} else {
				bind = append(bind, f(i))
			}

		default:
			p.AddErr(v, "unexpected Mapping Bind arg: %v, must format like foo.Foo{}", v)
		}
	}
	return bind
}

func (p *Parser) parseMappingHttpCode(args []ast.Expr) int {

	for _, arg := range p.mustArgsToString(args) {
		code, err := strconv.Atoi(unquote(arg))
		if err == nil {
			return code
		}
		break
	}
	p.AddErr(args[0], "unexpected HttpStatus arg: %v", args[0])
	return 0
}

func (p *Parser) parseMappingMiddleware(args []ast.Expr) []string {

	middlewares := make([]string, 0)
	for _, a := range p.mustArgsToString(args) {
		middlewares = append(middlewares, unquote(a))
	}
	return middlewares
}

func (p *Parser) mustArgsToString(args []ast.Expr) []string {

	ss := make([]string, 0)
	for _, arg := range args {
		switch a := arg.(type) {
		case *ast.BasicLit:
			ss = append(ss, unquote(a.Value))
		case *ast.Ident:
			o := p.objCache.ObjectOf(a)

			c, ok := o.(*types.Const)
			if !ok {
				p.AddErr(arg, "args %v expect to be const string, but got: %v", arg, reflect.TypeOf(o))
				continue
			}
			ss = append(ss, strings.Trim(c.Val().String(), "\""))
		case *ast.SelectorExpr:
			o := p.objCache.ObjectOf(a.Sel)
			c, ok := o.(*types.Const)
			if !ok {
				p.AddErr(arg, "args %v expect to be const string, but got: %v", arg, reflect.TypeOf(o))
				continue
			}

			ss = append(ss, strings.Trim(c.Val().String(), "\""))
		default:
			p.AddErr(arg, "args %v expect to be const string, but got: %v ", arg, reflect.TypeOf(a))
		}
	}
	return ss
}

func (p *Parser) AddErr(expr ast.Expr, format string, a ...any) {
	p.errs = append(p.errs, fmt.Errorf("%s: %s", p.fset.Position(expr.Pos()), fmt.Sprintf(format, a...)))
}

func (p *Parser) Visit(node ast.Node) ast.Visitor {

	switch n := node.(type) {
	case *ast.TypeSpec:

	case *ast.FuncDecl:
		handler, err := p.ParseHandler(n)
		if err == nil {
			p.api.Handlers = append(p.api.Handlers, *handler)
		}
	case *ast.GenDecl:
		if n.Tok == token.VAR {
			_ = p.parseMappingWith(n)
			return p
		}

		if n.Tok == token.TYPE {
			_ = p.parseSchema(n)
		}

	}
	return p
}

func (p *Parser) parseSchema(node *ast.GenDecl) error {
	if node.Tok != token.TYPE {
		return nil
	}

	if p.api.Schemas == nil {
		p.api.Schemas = make(map[string]spec.Schema)
	}

	for _, sp := range node.Specs {
		typeSpec, ok := sp.(*ast.TypeSpec)
		if !ok {
			continue
		}

		typeFullName := p.packages.Name + "." + typeSpec.Name.Name

		if _, ok := p.api.Schemas[typeFullName]; ok {
			continue
		}

		// We're only interested in struct types
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		// Create new schema for this struct
		schema := spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type:       []string{"object"},
				Properties: make(map[string]spec.Schema),
			},
		}

		for _, field := range structType.Fields.List {
			fieldType := field.Type
			if star, ok := fieldType.(*ast.StarExpr); ok {
				fieldType = unwrapStarExpr(star)
			}

			var fieldNames []string
			if len(field.Names) == 0 {
				// Embedded field
				switch t := fieldType.(type) {
				case *ast.Ident:
					fieldNames = append(fieldNames, t.Name)
				case *ast.SelectorExpr:
					fieldNames = append(fieldNames, t.Sel.Name)
				}
			} else {
				for _, name := range field.Names {
					fieldNames = append(fieldNames, name.Name)
				}
			}

			// Parse field tags
			var jsonName string
			var required bool
			if field.Tag != nil {
				tags := parseStructTags(field.Tag.Value)
				if name, ok := tags["json"]; ok {
					jsonName = name
				}
				if v, ok := tags["validate"]; ok && strings.Contains(v, "required") {
					required = true
				}
			}

			// Get field type information
			var fieldSchema spec.Schema
			switch t := fieldType.(type) {
			case *ast.Ident:
				p.objCache.ObjectOf(t).Pkg()
				fieldSchema = p.typeToSchema(p.objCache.ObjectOf(t).Pkg(), t.Name)
			case *ast.SelectorExpr:
				fieldSchema = p.typeToSchema(p.objCache.ObjectOf(t.Sel).Pkg(), t.Sel.Name)
			case *ast.ArrayType:
				fieldSchema = spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"array"},
						Items: &spec.SchemaOrArray{
							Schema: &spec.Schema{},
						},
					},
				}
				switch elemType := t.Elt.(type) {
				case *ast.Ident:
					*fieldSchema.Items.Schema = p.typeToSchema(p.objCache.ObjectOf(elemType).Pkg(), elemType.Name)
				case *ast.SelectorExpr:
					*fieldSchema.Items.Schema = p.typeToSchema(p.objCache.ObjectOf(elemType.Sel).Pkg(), elemType.Sel.Name)
				case *ast.StarExpr:
					switch st := unwrapStarExpr(elemType).(type) {
					case *ast.Ident:
						*fieldSchema.Items.Schema = p.typeToSchema(p.objCache.ObjectOf(st).Pkg(), st.Name)
					case *ast.SelectorExpr:
						*fieldSchema.Items.Schema = p.typeToSchema(p.objCache.ObjectOf(st.Sel).Pkg(), st.Sel.Name)
					}
				}
			}

			// Add field comments as description
			if field.Doc != nil {
				var comments []string
				for _, comment := range field.Doc.List {
					// Trim the comment prefix and any leading/trailing whitespace
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					comments = append(comments, text)
				}
				fieldSchema.Description = strings.Join(comments, " ")
			}

			// Add field to schema
			for _, name := range fieldNames {
				if jsonName == "" {
					jsonName = name
				}
				schema.Properties[jsonName] = fieldSchema
				if required {
					schema.Required = append(schema.Required, jsonName)
				}
			}
		}

		// Add schema to API schemas
		p.api.Schemas[typeFullName] = schema
	}

	return nil
}

// Helper function to convert Go types to OpenAPI schema types
func (p *Parser) typeToSchema(pkg *types.Package, typeName string) spec.Schema {
	schema := spec.Schema{
		SchemaProps: spec.SchemaProps{},
	}

	pkgNamePrefix := ""
	if pkg != nil {
		pkgNamePrefix = pkg.Name() + "."
	}

	switch typeName {
	case "string":
		schema.Type = []string{"string"}
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		schema.Type = []string{"integer"}
		if strings.HasSuffix(typeName, "64") {
			schema.Format = "int64"
		}
	case "float32", "float64":
		schema.Type = []string{"number"}
		if typeName == "float64" {
			schema.Format = "double"
		}
	case "bool":
		schema.Type = []string{"boolean"}
	case "time.Time":
		schema.Type = []string{"string"}
		schema.Format = "date-time"

	case "[]byte":
		schema.Type = []string{"string"}
		schema.Format = "binary"

	default:
		// Reference to another schema
		schema.Ref = spec.MustCreateRef("#/definitions/" + pkgNamePrefix + typeName)
	}

	return schema
}

// Helper function to parse struct tags
func parseStructTags(tag string) map[string]string {
	tags := make(map[string]string)

	// Parse using reflect.StructTag
	st := reflect.StructTag(tag)

	// Get common tags
	if v := st.Get("json"); v != "" {
		name := strings.Split(v, ",")[0]
		if name != "-" {
			tags["json"] = name
		}
	}

	if v := st.Get("validate"); v != "" {
		tags["validate"] = v
	}

	if v := st.Get("example"); v != "" {
		tags["example"] = v
	}

	return tags
}

// parseFuncName 解析函数全名，返回包路径和包名
func parseFuncName(funcName string) (packagePath, packageName string) {
	// 分割字符串获取最后一个"/"之前的部分作为包路径
	slashIndex := strings.LastIndex(funcName, "/")
	if slashIndex == -1 {
		// 如果没有找到"/"，则假设没有包路径
		return "", ""
	}
	packagePath = funcName[:slashIndex]

	// 获取包名，包名是包路径的最后一部分
	packagePathParts := strings.Split(funcName[slashIndex+1:], ".")
	packageName = packagePathParts[0]

	return packageName, packagePath + "/" + packageName
}

func GetFuncName(a any) (pkgPath, name string, ok bool) {

	funcPtr := reflect.ValueOf(a).Pointer()
	funcForPC := runtime.FuncForPC(funcPtr)

	p, n := parseFuncName(funcForPC.Name())
	return p, n, true
}

func unwrapStarExpr(expr *ast.StarExpr) ast.Expr {

	switch x := expr.X.(type) {
	case *ast.StarExpr:
		return unwrapStarExpr(x)
	default:
		return x
	}
}

func (p *Parser) ParseHandler(decl *ast.FuncDecl) (*HandleFunc, error) {

	h := HandleFunc{
		Name:        decl.Name.Name,
		PackageName: p.objCache.ObjectOf(decl.Name).Pkg().Name(),
	}

	if decl.Doc != nil {
		var docs []string
		for _, doc := range decl.Doc.List {
			docs = append(docs, strings.TrimPrefix(doc.Text, "//"))
		}
		h.Doc = strings.TrimSpace(strings.Join(docs, "\n"))
	}

	obj := p.objCache.ObjectOf(decl.Name)
	pkg := obj.Pkg()

	h.RouteInfoPackage = PackageItem{
		Name: pkg.Name(),
		Path: pkg.Path(),
	}

	h.RequestArgs = p.parseHandlerArg(decl.Type.Params)
	h.ResponseResult = p.parseHandlerArg(decl.Type.Results)
	h.Pos = p.fset.Position(decl.Pos())
	return &h, nil
}

func (p *Parser) parseHandlerArg(list *ast.FieldList) []Arg {

	if list == nil {
		return []Arg{}
	}

	var args []Arg
	for _, l := range list.List {

		a := Arg{}

		a.Desc = l.Doc.Text() + "comment:" + l.Comment.Text()

		expr := l.Type
		if s, ok := l.Type.(*ast.StarExpr); ok {
			a.Star = true
			expr = unwrapStarExpr(s)
		}

		switch t := expr.(type) {
		case *ast.Ident:
			a.ObjectTypes = p.objCache.ObjectOf(t)
		case *ast.SelectorExpr:
			a.ObjectTypes = p.objCache.ObjectOf(t.Sel)
		default:
			p.AddErr(expr, "unexpected type: %v", reflect.TypeOf(t))
		}

		if a.ObjectTypes != nil {
			pkg := a.ObjectTypes.Pkg()
			if pkg != nil {
				a.Package = PackageItem{
					Name: pkg.Name(),
					Path: pkg.Path(),
				}
			}
			a.Type = toType(a.ObjectTypes)
		}

		for _, pn := range l.Names {
			na := a
			na.Name = pn.Name
			args = append(args, na)
		}

		if len(l.Names) == 0 {
			args = append(args, a)
		}
	}
	return args
}

func Beautify(o any) string {
	bs, _ := json.MarshalIndent(o, "", "  ")
	return string(bs)
}

func load(root string, tag ...string) ([]*packages.Package, error) {

	var dirs []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return packages.Load(&packages.Config{
		Mode:       packages.LoadAllSyntax,
		Dir:        root,
		BuildFlags: tag,
	}, dirs...)
}

func Parse(root string, tag ...string) (*RestfulApi, error) {
	pkgs, err := load(root)
	if err != nil {
		log.Fatalf("load pkgs failed: %v", err)
	}

	objCache := NewObjectCache(pkgs)

	api := API{}
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			v := &Parser{
				packages: p,
				fset:     p.Fset,
				objCache: objCache,
				api:      &api,
			}
			ast.Walk(v, f)
			for _, err := range v.errs {
				fmt.Println("ast walk failed", err)
			}
		}
	}

	restapi, err := BuildRestfulApi(root, api)

	return restapi, err
}
