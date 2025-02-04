# Nextgo, Headless Go

一个轻量级的 Go REST API 框架，专注于业务逻辑而非框架细节。
灵感来源于 Next.js 的目录式 API 路由。

## 特性

- **目录式路由**：基于目录结构自动生成路由
- **自动参数处理**：自动解码路径、查询、请求体和请求头参数
- **内置编解码支持**：自动请求解码和响应编码
- **中间件支持**：支持全局、版本特定和路由特定的中间件
- **清晰架构**：业务逻辑与 Web 框架细节分离
- **自动文档**：从代码生成 OpenAPI/Swagger 文档

## 安装

```bash
go get github.com/headless-go/nextgo
```

## 使用示例（Todo 应用）

包含的 todo 示例演示了主要特性：

### 目录结构

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

### 生成的路由

```bash
GET /v1/healthcheck
POST /v1/todos
GET /v1/todos
PUT /v1/todos/{id}/done
PUT /v1/todos/{id}/active  
DELETE /v1/todos/{id}
```

### 示例处理器（todos/id.go）

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

### 运行示例

1. 生成路由：
```bash
nextgo api generate --src=./api --out=./generated
```

2. 启动服务器：
```bash
go run main.go
```

## 映射

映射用于定义 API 处理器的属性。有两种类型的映射：

1. `Mapping`：应用于当前文件中其下方最近的处理器函数。
2. `MappingFile`：应用于文件中的所有处理器函数。

示例：

```go
var _ = nextgo.Mapping.HttpMethod(http.MethodPost)

func CreateTodoItem(ctx context.Context, reqItem TodoItemRequest) (*TodoItem, error) { 
    // 处理器实现
}
```

### 可用的映射方法

- **HttpMethod**：指定端点的 HTTP 方法
- **PathPrefix**：配置 API 路径使用前缀匹配
- **StatusCode**：设置成功响应的 HTTP 状态码
- **Middleware**：配置端点的中间件
- **Label**：为端点添加元数据标签
- **BindQuery**：指定从查询字符串解析的参数
- **BindHeader**：指定从请求头解析的参数

## 参数处理

### 路径参数
- 从目录名自动解析（如 `id`）
- 必须与处理器函数中的参数名匹配
- 仅支持基本类型

### 请求体参数
- 结构体参数自动从请求体解码
- 自动处理 JSON 解码

### 查询参数
- 使用 `Mapping.BindQuery` 自动从 URL 查询字符串解析参数
- 映射到请求类型中的结构体字段

### 请求头参数
- 自动从 HTTP 请求头提取
- 可以使用 `Mapping.BindHeader` 映射到特定的结构体字段

## 参数验证

您可以通过提供 Validator 实现来验证结构体参数。推荐使用 [go-playground/validator](https://github.com/go-playground/validator)。

## 中间件

### 中间件声明

Nextgo 使用字符串常量来声明中间件，具体实现由用户提供。中间件执行顺序与在 `Mapping.Middleware` 中的声明顺序相匹配。

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

中间件可以定义在：
1. 子目录中的 `middleware.go` 文件（应用于该路径下的所有 API）
2. 直接在 API 处理器上（仅应用于该端点）

中间件特性：
1. 继承：子路径 API 继承父路径的中间件
2. 合并：带有 `-` 后缀的中间件会移除具有相同前缀的先前中间件

### 中间件实现

中间件必须实现以下函数签名：
```go
func(next http.Handler) http.Handler
```

包装第三方中间件的示例：

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

## API 文档

生成 OpenAPI/Swagger 文档：
```bash
nextgo swag generate --src=./api --out=./generated
```

## 贡献

欢迎贡献！请提交 Pull Request。

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。