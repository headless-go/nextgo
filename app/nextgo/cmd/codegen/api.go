package codegen

import (
	"fmt"
	"go/token"
	"go/types"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-openapi/spec"
)

// Type
// type User struct{}
// PackageItem {"user", "xxx.xx/xxx/user", "userPkg"}
// Name: User
// Kind: struct
// Star: false
type Type struct {
	PackageItem
	// without package info
	Name string
	Kind string
	Star bool

	// package path and Name
	FullName string

	PackageName string
}

func (t Type) EqualTo(t2 Type) bool {

	return t.Name == t2.Name && t.Path == t2.Path
}

func (t Type) IsPrimitive() bool {
	switch t.Name {
	case "string", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "bool", "byte", "rune", "uintptr", "error":
		return true
	}
	return false
}

// Mapping store parsed mapping info
type Mapping struct {
	// With or WithGlobal
	Scope      string
	HttpMethod string
	Label      map[string]string
	Middleware []string
	HttpCode   int
	PathPrefix bool
	BindQuery  []Type
	BindHeader []Type

	// expr line in file, value from pos
	line int
	pos  token.Position
}

// Arg is handle func input and out parameter
type Arg struct {
	Name string
	Desc string
	Star bool
	// packageName.TypeName
	Type Type
	// Location 参数位置
	Location    string
	ObjectTypes types.Object
	Package     PackageItem

	//
	PathParamName string
}

type HandleFunc struct {
	Doc            string
	Name           string
	RequestArgs    []Arg
	ResponseResult []Arg
	ResponseError  bool
	With           *Mapping
	WithGlobal     *Mapping

	Imports     []PackageItem
	Patten      string
	PackageName string

	RouteInfoPackage  PackageItem
	AliceChainPackage PackageItem
	GoHttpPackage     PackageItem

	ParentMiddlewares []string
	// Middlewares
	Middlewares []string
	Pos         token.Position

	// help for generate code
	GeneratedPackageInfo PackageItem

	mappingMerged bool
}

func (h *HandleFunc) mergeMapping() {

	if h.mappingMerged {
		return
	}
	if h.With == nil {
		h.With = &Mapping{HttpMethod: "GET"}
	}

	h.mappingMerged = true

	if h.WithGlobal == nil {
		return
	}

	h.With.Middleware = append(h.WithGlobal.Middleware, h.With.Middleware...)
	h.With.BindQuery = append(h.WithGlobal.BindQuery, h.With.BindQuery...)
	h.With.BindHeader = append(h.WithGlobal.BindHeader, h.With.BindHeader...)
	l := map[string]string{}
	for k, v := range h.WithGlobal.Label {
		l[k] = v
	}
	for k, v := range h.With.Label {
		l[k] = v
	}
	h.With.Label = l
	h.Middlewares = resolveIncludeExclude(append(h.ParentMiddlewares, h.With.Middleware...))
	return
}

type RestfulApi struct {
	Apis    map[string]map[string]HandleFunc
	Schemas map[string]spec.Schema
}

func BuildRestfulApi(prefix string, api API) (*RestfulApi, error) {

	handlers, err := bindHandleFuncMapping(api.Handlers, api.Annotations)
	if err != nil {
		return nil, err
	}

	apis := make(map[string]map[string]HandleFunc)

	for _, h := range handlers {
		if h.With == nil {
			h.With = &Mapping{
				HttpMethod: http.MethodGet,
				HttpCode:   http.StatusOK,
			}
		}

		// Get the HTTP method from annotation, default to GET if not specified
		method := h.With.HttpMethod
		if method == "" {
			method = http.MethodGet
		}

		path := getApiPath(prefix, h.Pos.Filename)
		ss := strings.Split(path, "/")
	L1:
		for i, s := range ss {
			for _, a := range h.RequestArgs {
				if a.Name == s {
					ss[i] = "{" + a.Name + "}"
					continue L1
				}
			}
		}
		path = strings.Join(ss, "/")
		if apis[path] == nil {
			apis[path] = make(map[string]HandleFunc)
		}
		h.Patten = path
		apis[path][method] = h

	}

	ret := &RestfulApi{Apis: apis, Schemas: api.Schemas}
	return ret, nil
}

func BindMiddlewareFile(annotation []Mapping) []Mapping {

	var middlewareFile []Mapping
	for _, a := range annotation {
		_, f := filepath.Split(a.pos.Filename)
		if f == "middleware.go" {
			middlewareFile = append(middlewareFile, a)
		}
	}
	slices.SortFunc(middlewareFile, func(a, b Mapping) int {
		if a.pos.Filename < b.pos.Filename {
			return -1
		}
		return 1
	})
	return middlewareFile
}

func bindHandleFuncMapping(handlers []HandleFunc, annotations []Mapping) ([]HandleFunc, error) {

	nodeMap := make(map[string][]*node)

	for k, v := range handlers {
		nodeMap[v.Pos.Filename] = append(nodeMap[v.Pos.Filename], &node{HandleFunc: &handlers[k]})
	}

	globalMapping := map[string][]Mapping{}

	for k, v := range annotations {
		if v.Scope == "Mapping" {
			nodeMap[v.pos.Filename] = append(nodeMap[v.pos.Filename], &node{Mapping: &annotations[k]})
		} else {
			if len(globalMapping[v.pos.Filename]) > 0 {
				return nil, fmt.Errorf("multiple GlobalMapping found %s:%d", v.pos.Filename, v.pos.Line)
			}
			globalMapping[v.pos.Filename] = append(globalMapping[v.pos.Filename], annotations[k])
		}
	}

	for _, fileNodes := range nodeMap {
		slices.SortFunc(fileNodes, func(a, b *node) int {
			return a.Pos().Line - b.Pos().Line
		})

		for i := 0; i < len(fileNodes)-1; i++ {
			if fileNodes[i].Mapping != nil {
				if n := fileNodes[i+1]; n.Mapping != nil {
					return nil, fmt.Errorf("multiple annotations found %s:%d",
						n.Pos().Filename, n.Pos().Line)
				} else {
					fileNodes[i+1].HandleFunc.With = fileNodes[i].Mapping
				}
			}
		}
	}

	middlewareFile := BindMiddlewareFile(annotations)
	for k, v := range handlers {
		for _, m := range middlewareFile {
			dir, _ := filepath.Split(m.pos.Filename)
			if strings.HasPrefix(v.Pos.Filename, dir) {
				handlers[k].ParentMiddlewares = append(handlers[k].ParentMiddlewares, m.Middleware...)
			}
		}
		if m, ok := globalMapping[v.Pos.Filename]; ok {
			handlers[k].WithGlobal = &m[0]
		}
	}

	return handlers, nil
}

func getApiPath(prefix, str string) string {
	str = strings.TrimPrefix(str, prefix)
	str = strings.TrimSuffix(str, ".go")
	return str
}
