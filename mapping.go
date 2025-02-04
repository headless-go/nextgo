package nextgo

// attrBase interface defines common configurations for HTTP API endpoints
// Example:
//
//	 var _ = Mapping.
//		BindQuery(ListApplicationsRequest{}).
//		BindHeader(ApiToken{}).
//		Label("code=LIST_CLUSTER_APP")
type attrBase interface {
	// BindQuery binds URL query parameters to a struct
	// Example: BindQuery(ListApplicationsRequest{}) binds 'page' and 'query' parameters
	// ListApplicationsRequest will be decoded from query
	BindQuery(...any) attrBase

	// BindHeader binds HTTP headers to a struct
	// Example: BindHeader(ApiToken{}) binds 'X-API-TOKEN' header
	// ApiToken will be decode from header
	BindHeader(...any) attrBase

	// Middleware adds middleware functions to the request pipeline
	// Example: Middleware(api.Auth) adds authentication middleware
	Middleware(...string) attrBase

	// Label adds tags for a route
	// Example: Label("code=CREATE_CLUSTER_APP", "auditlog.resource=CLUSTER_APP")
	Label(...string) attrBase
}

// attr interface extends attrBase with HTTP-specific configurations
type attr interface {
	// HttpMethod sets the HTTP method for the endpoint
	// Example: HttpMethod(http.MethodPost) sets POST method
	HttpMethod(string) attr

	// PathPrefix sets the API path match with prefix
	PathPrefix() attr

	// StatusCode sets the response status code
	// Example: StatusCode(http.StatusCreated) sets 201 status code
	StatusCode(i int) attr

	attrBase
}

// Mapping is used to define configurations for individual API endpoints
// Example:
// var _ = mapping.Mapping.StatusCode(httpPkg.StatusCreated).
//
//	HttpMethod(httpPkg.MethodPost).
//	Label("code=CREATE_CLUSTER_APP", "auditlog.resource=CLUSTER_APP")
var Mapping = empty{}

// MappingFile is used to define global API configurations in file scope
var MappingFile = emptyBase{}

// empty is a no-op implementation used for compile-time API definition checks
type empty struct{ emptyBase }

type emptyBase struct{}

func (n empty) HttpMethod(string) attr { return n }
func (n empty) PathPrefix() attr       { return n }
func (n empty) StatusCode(int) attr    { return n }

func (n emptyBase) Middleware(...string) attrBase { return n }
func (n emptyBase) Label(...string) attrBase      { return n }
func (n emptyBase) BindQuery(...any) attrBase     { return n }
func (n emptyBase) BindHeader(...any) attrBase    { return n }
