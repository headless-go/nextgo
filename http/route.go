package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/headless-go/nextgo/http/codec"
)

type ctxKey int

const (
	routeinfoContextKey ctxKey = iota
)

type RouteInfo struct {
	HTTPMethod      string
	Method          string
	Patten          string
	Desc            string
	Label           map[string]string
	Middleware      []string
	HandlerFuncPkg  string
	HandlerFuncName string
	HandleFunc      string
	Response        []any
	Request         []any
	PathParam       []string
}

func WithRouteInfo(r *RouteInfo) func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			req = req.WithContext(context.WithValue(req.Context(), routeinfoContextKey, r))
			next.ServeHTTP(rw, req)
		})
	}
}

func RouteInfoFromContext(ctx context.Context) *RouteInfo {
	r, _ := ctx.Value(routeinfoContextKey).(*RouteInfo)
	return r
}

type Option interface {
	codec.Codec
	Validator
	AddRoute(RouteInfo)
}

var defaultLogRouterFunc = func(r RouteInfo) {
	fmt.Printf("%s %s -> %s.%s.%s\n", r.HTTPMethod, r.Patten, r.HandlerFuncPkg, r.HandlerFuncName, r.Method)
}

type option struct {
	codec.Codec
	Validator
	routes         []RouteInfo
	onRouteAddFunc []func(info RouteInfo)
}

func (o option) AddRoute(info RouteInfo) {
	for _, f := range o.onRouteAddFunc {
		f(info)
	}
}

func NewDefaultOption(opts ...OptionFunc) Option {

	o := option{
		Validator:      &validator{},
		onRouteAddFunc: []func(info RouteInfo){defaultLogRouterFunc},
	}

	for _, f := range opts {
		f(&o)
	}
	return &o
}

type OptionFunc func(opt *option)

func WithCodec(c codec.Codec) OptionFunc {
	return func(api *option) {
		api.Codec = c
	}
}

func WithOnRouteAdded(f ...func(info RouteInfo)) OptionFunc {
	return func(opt *option) {
		opt.onRouteAddFunc = append(opt.onRouteAddFunc, f...)
	}
}

func WithValidator(v Validator) OptionFunc {
	return func(opt *option) {
		opt.Validator = v
	}
}

type Validator interface {
	Struct(s interface{}) error
}

type validator struct{}

func (v *validator) Struct(s any) error {
	return nil
}
