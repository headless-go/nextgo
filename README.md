# Nextgo - Headless Go Framework

A lightweight framework for building REST APIs in Go, inspired by Next.js directory-based routing. Focuses on business logic while abstracting web framework details.

## Key Features

- **Intuitive Routing**: Directory-based automatic route generation
- **Parameter Handling**: Automatic decoding of path, query, body and header parameters
- **Codec Support**: Built-in request/response encoding/decoding
- **Middleware System**: Global, version-specific and route-specific middleware support
- **Clean Architecture**: Clear separation of concerns
- **API Documentation**: Automatic OpenAPI/Swagger generation

## Requirements

- Go 1.20+ 
- Go modules enabled

## Installation

Install the Nextgo CLI:

```bash
go install github.com/headless-go/nextgo/app/nextgo@latest
```

Install nextgo in your project:

```bash
go get -u github.com/headless-go/nextgo@latest
```

## Example Usage (Todo App)

See [headless-go/nextgo-example-todo](https://github.com/headless-go/nextgo-example-todo) for more info.

The todo example demonstrates key features:

### Directory Structure

```bash
├── api
│   └── v1
│       ├── healthcheck.go
│       ├── todos
│       │   ├── id
│       │   │   ├── active.go
│       │   │   └── done.go  
│       │   └── id.go
│       └── todos.go
```

### Generated Routes

```bash
GET /v1/healthcheck
POST /v1/todos
GET /v1/todos
PUT /v1/todos/{id}/done
PUT /v1/todos/{id}/active  
DELETE /v1/todos/{id}
```

### Example Handler (todos/id.go)

```go
type UpdateTodoItemRequest struct {
    Title string `json:"title" validate:"required"`
}

var _ = nextgo.Mapping.HttpMethod(http.MethodPut)

func UpdateTodoItem(ctx context.Context, id string, req UpdateTodoItemRequest) error {
    err := app.DefaultApplication().UpdateTodo(ctx, id, req.Title)
    if err != nil {
        return err
    }
    return nil
}
```

### Running the Example

1. Generate routes:
```bash
nextgo api generate --src=./api --out=./generated
```

2. Start server:
```bash
go run main.go
```

## Mapping

Mapping is used to define attributes for API handlers. There are two types of mappings:

1. `Mapping`: Applies to the nearest handler function below it in the current file.
2. `MappingFile`: Applies to all handler functions in the file.

Example:

```go
var _ = nextgo.Mapping.HttpMethod(http.MethodPost)

func CreateTodoItem(ctx context.Context, reqItem TodoItemRequest) (*TodoItem, error) { 
    // Handler implementation
}
```

### Available Mapping Methods

- **HttpMethod**: Specifies the HTTP method for the endpoint
- **PathPrefix**: Configures the API path to use prefix matching
- **StatusCode**: Sets the HTTP status code for successful responses
- **Middleware**: Configures middleware for the endpoint
- **Label**: Adds metadata tags to the endpoint
- **BindQuery**: Specifies parameters to be parsed from query string
- **BindHeader**: Specifies parameters to be parsed from headers

## Parameter Handling

### Path Parameters
- Automatically parsed from directory names (e.g. `id`)
- Must match parameter name in handler function
- Primitive types only

### Body Parameters  
- Struct parameters automatically decoded from request body
- JSON decoding handled automatically

### Query Parameters
- Use `Mapping.BindQuery` to automatically parse parameters from URL query string
- Mapped to struct fields in request types

### Header Parameters
- Automatically extracted from HTTP headers
- Can be mapped to specific struct fields using `Mapping.BindHeader`

## Parameter Validation

You can validate struct parameters by providing a Validator implementation. The recommended validator is [go-playground/validator](https://github.com/go-playground/validator).

## Middleware

### Middleware Declaration

Nextgo uses string constants to declare middleware, with the actual implementation provided by the user. Middleware execution order matches the declaration order in `Mapping.Middleware`.

```go
const (
    Auth      = "auth"
    AuthX     = "auth-"
    AccessLog = "accessLog"
    Recover   = "recover"
    RateLimit = "rateLimit"
)

var (
    _ = nextgo.Mapping.Middleware(Recover, RateLimit, AccessLog)
)
```

Middleware can be defined in:
1. A `middleware.go` file in a subdirectory (applies to all APIs in that path)
2. Directly on an API handler (applies only to that endpoint)

Middleware features:
1. Inheritance: Child path APIs inherit middleware from parent paths
2. Merging: Middleware with a `-` suffix removes previous middleware with the same prefix

### Middleware Implementation

Middleware must implement the following function signature:
```go
func(next http.Handler) http.Handler
```

Example of wrapping third-party middleware:

```go
package middleware

import (
    "io"
    "net/http"

    "github.com/gorilla/handlers"
)

func AccessLoggingHandler(out io.Writer) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return handlers.LoggingHandler(out, next)
    }
}
```

## Supported Web Framework

- Mux:[headless-go/nextgo-mux](https://github.com/headless-go/nextgo-mux)
- Gin:[headless-go/nextgo-gin](https://github.com/headless-go/nextgo-gin)
- Echo:[headless-go/nextgo-echo](https://github.com/headless-go/nextgo-echo)

To support you own favorite framework, just implement the following interface.
```go
type Server interface {
	HandleFunc(method string, path string, handler http.HandlerFunc)
}
```

## API Documentation

Generate OpenAPI/Swagger docs:
```bash
nextgo swag generate --src=./api --out=./generated
```

## Contributing

Contributions welcome! Please submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.
