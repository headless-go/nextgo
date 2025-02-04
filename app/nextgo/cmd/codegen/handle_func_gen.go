package codegen

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/justinas/alice"
	"github.com/samber/lo"
	"golang.org/x/tools/imports"

	http2 "github.com/headless-go/nextgo/http"
)

var fns = template.FuncMap{
	"notlast": func(x int, a interface{}) bool {
		return x != reflect.ValueOf(a).Len()-1
	},
}

//go:embed tpl/handle_func.gotmpl
var handleTpl string

//go:embed tpl/handle_import.gotmpl
var importTpl string

//go:embed tpl/server.gotmpl
var serverTpl string

type option struct {
	srcDir          string
	outputDir       string
	outputPkgPrefix string
	handleTmpl      *template.Template
	importTmpl      *template.Template
	serverTmpl      *template.Template
}

func newDefaultOption() *option {

	o := option{}
	var err error
	o.handleTmpl, err = template.New("handle").Funcs(fns).Parse(handleTpl)
	if err != nil {
		panic("failed new option:" + err.Error())
	}

	o.importTmpl, err = template.New("import").Funcs(fns).Parse(importTpl)
	if err != nil {
		panic("failed new option:" + err.Error())
	}

	o.serverTmpl, err = template.New("server").Funcs(fns).Parse(serverTpl)
	if err != nil {
		panic("failed new option:" + err.Error())
	}

	return &o
}

type GenerateOptionFunc func(opt *option)

// Generate generates the code for the given APIs.
func Generate(srcDir, outputDir, outPkgPrefix string, apis *RestfulApi) error {

	o := newDefaultOption()
	o.outputDir = outputDir
	o.srcDir = srcDir
	o.outputPkgPrefix = outPkgPrefix

	for _, handlers := range apis.Apis {
		if err := o.generateApiHandler(handlers); err != nil {
			return err
		}
	}

	if err := o.generateServer(filepath.Dir(outputDir), outPkgPrefix, apis.Apis); err != nil {
		return err
	}
	return nil
}

type serverApiData struct {
	Match             string
	Patten            string
	Method            string
	HandleFunc        string
	HandleFuncPackage string
	Middlewares       []string
}

type serverData struct {
	Middlewares []string
	Apis        []serverApiData
	Imports     []PackageItem
	PackageName string

	RouteInfoPackage  PackageItem
	AliceChainPackage PackageItem
	GoHttpPackage     PackageItem
}

func (o *option) generateServer(outputDir string, packagePrefix string, apis map[string]map[string]HandleFunc) error {

	svrData := serverData{
		PackageName: filepath.Base(o.outputDir),
	}

	var imports []PackageItem
	for _, p := range apis {
		for _, h := range p {
			imports = append(imports, h.GeneratedPackageInfo)
			break
		}
	}
	imports = append(imports, getPackageItem[http2.Server]())
	imports = aliasImports(lo.UniqBy(append(imports, defaultPkgs...), func(item PackageItem) string { return item.Path }))

	svrData.RouteInfoPackage = aliasImportsPackage(imports, routeInfoPackage)
	svrData.GoHttpPackage = aliasImportsPackage(imports, goHttpPackage)
	svrData.AliceChainPackage = aliasImportsPackage(imports, aliceChainPackage)

	var apiData []serverApiData
	for patten, p := range apis {
		for m, h := range p {
			h.mergeMapping()
			d := serverApiData{
				Match: func(isPrefix bool) string {
					if isPrefix {
						return "PathPrefix"
					}
					return "Path"
				}(h.With.PathPrefix),
				Patten:     patten,
				Method:     m,
				HandleFunc: h.Name,
				HandleFuncPackage: func(h HandleFunc) string {
					p, _ := lo.Find(imports, func(item PackageItem) bool {
						return item.Path == h.GeneratedPackageInfo.Path
					})
					if p.Alias != "" {
						return p.Alias
					}
					return p.Name
				}(h),
				Middlewares: h.Middlewares,
			}
			svrData.Middlewares = lo.Uniq(append(svrData.Middlewares, d.Middlewares...))
			apiData = append(apiData, d)
		}
	}

	svrData.Imports = imports

	svrData.Apis = apiData

	var buf bytes.Buffer
	if err := o.serverTmpl.Execute(&buf, svrData); err != nil {
		return err
	}
	return writeFile(filepath.Join(outputDir, "server.go"), buf.Bytes())
}

func (o *option) getGeneratedPkgPath(relativeFilename string) string {

	pkg := filepath.Dir(strings.TrimPrefix(relativeFilename, o.srcDir))
	return filepath.Join(o.outputPkgPrefix, pkg)
}

func (o *option) generateApiHandler(handlers map[string]HandleFunc) error {

	var buf bytes.Buffer

	apiImports := o.generateApiImports(handlers)
	var bs bytes.Buffer
	if err := o.importTmpl.Execute(&bs, apiImports); err != nil {
		return err
	}

	buf.WriteString(bs.String())
	buf.WriteString("\n")

	for k, v := range handlers {
		s, err := o.generateHandler(v, apiImports)
		if err != nil {
			return err
		}
		buf.WriteString(s)
		buf.WriteString("\n")

		v.GeneratedPackageInfo = PackageItem{
			Name: v.RouteInfoPackage.Name,
			Path: o.getGeneratedPkgPath(strings.TrimPrefix(v.Pos.Filename, o.srcDir)),
		}
		handlers[k] = v

	}

	for _, v := range handlers {
		f := strings.TrimPrefix(v.Pos.Filename, o.srcDir)
		if err := writeFile(filepath.Join(o.outputDir, f), buf.Bytes()); err != nil {
			return err
		}
		return nil
	}
	return nil
}

type PackageItem struct {
	Name  string
	Path  string
	Alias string
}

type packageImports struct {
	PackageName string
	Imports     []PackageItem
}

func (o *option) generateApiImports(handlers map[string]HandleFunc) *packageImports {

	p := packageImports{}

	add := false
	for _, v := range handlers {
		if !add {
			p.Imports = append(p.Imports, v.RouteInfoPackage)
			add = true
		}

		p.PackageName = v.PackageName
		for _, a := range append(v.RequestArgs, v.ResponseResult...) {
			if a.Package.Path == "" {
				continue
			}
			p.Imports = append(p.Imports, PackageItem{
				Name: a.Package.Name,
				Path: a.Package.Path,
			})
		}
	}

	p.Imports = lo.UniqBy(append(p.Imports, defaultPkgs...), func(item PackageItem) string { return item.Name + " " + item.Path })
	p.Imports = lo.Filter(p.Imports, func(item PackageItem, index int) bool { return item.Path != "context" })

	p.Imports = aliasImports(p.Imports)

	return &p
}

func aliasImports(imports []PackageItem) []PackageItem {
	nameCount := make(map[string]int)
	for _, imp := range imports {
		nameCount[imp.Name]++
	}

	for i, imp := range imports {
		if nameCount[imp.Name] > 1 {
			imports[i].Alias = imp.Name + strconv.Itoa(i)
		}
	}
	return imports
}

func writeFile(f string, data []byte) error {

	dir, _ := filepath.Split(f)
	if err := os.MkdirAll(dir, os.ModeDir|os.ModePerm); err != nil {
		return nil
	}

	out, err := imports.Process(".", data, &imports.Options{
		FormatOnly: true,
	})
	if err != nil {
		return fmt.Errorf("imports process err:%w, code:%s", err, string(data))
	}
	data = []byte("// Code generated by nextgo; DO NOT EDIT.\n\n" + string(out))

	return os.WriteFile(f, data, 0664)
}

func (o *option) generateHandler(handle HandleFunc, pkgs *packageImports) (string, error) {
	handle.mergeMapping()
	autoNameHandleResponse(&handle)

	pkgsCache := map[string]PackageItem{}
	for _, v := range pkgs.Imports {
		pkgsCache[v.Path] = v
	}

L1:
	for i, a := range handle.RequestArgs {

		if p, ok := pkgsCache[a.Package.Path]; ok {
			if p.Alias != "" {
				handle.RequestArgs[i].Package = p
				handle.RequestArgs[i].Type.PackageName = p.Alias + "." + a.Type.Name
			}
		}

		for _, t := range []string{"context.Context", "net/http.Request", "net/http.ResponseWriter"} {
			if t == a.Type.FullName {
				continue L1
			}
		}

		handle.RequestArgs[i].Location = "body"

		if _, ok := lo.Find(handle.With.BindQuery, func(item Type) bool { return item.EqualTo(a.Type) }); ok {
			handle.RequestArgs[i].Location = "query"
			continue
		}
		if _, ok := lo.Find(handle.With.BindHeader, func(item Type) bool { return item.EqualTo(a.Type) }); ok {
			handle.RequestArgs[i].Location = "header"
			continue
		}
		if a.Type.IsPrimitive() {
			handle.RequestArgs[i].Location = "path"
			if handle.PackageName == handle.RequestArgs[i].Name {
				handle.RequestArgs[i].PathParamName = handle.RequestArgs[i].Name
				handle.RequestArgs[i].Name = generateVarName(handle.Name, "", handle.RequestArgs[i].PathParamName+"Param")
			}
		}
	}

	handle.RouteInfoPackage = aliasImportsPackage(pkgs.Imports, routeInfoPackage)
	handle.GoHttpPackage = aliasImportsPackage(pkgs.Imports, goHttpPackage)
	handle.AliceChainPackage = aliasImportsPackage(pkgs.Imports, aliceChainPackage)

	var bs bytes.Buffer
	if err := o.handleTmpl.Execute(&bs, handle); err != nil {
		return "", err
	}
	return bs.String(), nil
}

func aliasImportsPackage(imports []PackageItem, item PackageItem) PackageItem {
	item.Alias = item.Name
	for _, a := range imports {
		if a.Path == item.Path {
			if a.Alias != "" {
				item.Alias = a.Alias
			}
			break
		}
	}
	return item
}

// resolveIncludeExclude processes a string array where items ending with "-"
// indicate exclusions. For example: ["a", "b", "b-"] returns ["a"] because
// "b-" excludes "b" from the final result.
func resolveIncludeExclude(arrs []string) []string {

	isSub := func(s string) bool { return strings.HasSuffix(s, "-") }
	isX := func(s, x string) bool {
		return s+"-" == x
	}
	buildSub := func(s string, ss []string) int {
		i := 1
		for _, t := range ss {
			if t == s {
				i += 1
			} else if isX(s, t) {
				i += -1
			}
		}
		return i
	}

	var result []string
	for i, s := range arrs {
		if isSub(s) {
			continue
		}
		if buildSub(s, arrs[i+1:]) > 0 {
			result = append(result, s)
		}
	}
	return lo.Uniq(result)
}

func autoNameHandleResponse(h *HandleFunc) {

	for i, a := range h.ResponseResult {
		if a.Name != "" {
			continue
		}
		a.Name = generateVarName(h.Name, a.Type.Name, a.Name)
		h.ResponseResult[i] = a
	}
}

var getAutoIncrementName = func() func(filename, prefix string) string {
	m := map[string]int{}
	return func(filename, prefix string) string {
		countKey := filename + "." + prefix
		m[countKey] = m[countKey] + 1
		if m[countKey] == 1 {
			return prefix
		}
		return prefix + strconv.Itoa(m[countKey]-1)
	}
}()

func generateVarName(filename, tpe, varName string) string {
	if tpe == "error" {
		return "err"
	}

	if varName != "" {
		return getAutoIncrementName(filename, varName)
	}

	if tpe == "" {
		return getAutoIncrementName(filename, "_var")
	}

	ss := strings.Split(tpe, ".")
	tpe = ss[len(ss)-1]
	return getAutoIncrementName(filename, strings.ToLower(tpe[:1])+tpe[1:])
}

var routeInfoPackage = getPackageItem[http2.RouteInfo]()
var goHttpPackage = getPackageItem[http.Request]()
var aliceChainPackage = getPackageItem[alice.Chain]()

var defaultPkgs = append([]PackageItem{},
	routeInfoPackage,
	goHttpPackage,
	aliceChainPackage,
)

// getPackageItem 获取传入值的包信息，即使是 nil
func getPackageItem[T any]() PackageItem {
	// 获取泛型类型的反射信息
	t := reflect.TypeOf((*T)(nil)).Elem()
	p := PackageItem{
		Path: t.PkgPath(),
		Name: filepath.Base(t.PkgPath()),
	}
	return p
}
