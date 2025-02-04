package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/spec"
)

func GenerateSwag(api *RestfulApi, outputDir string, templateDir string) error {
	// Create a new Swagger spec
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger: "2.0",
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:       "API Documentation",
					Description: "Auto-generated API documentation",
					Version:     "1.0.0",
				},
			},
			Paths:       &spec.Paths{Paths: make(map[string]spec.PathItem)},
			Definitions: make(map[string]spec.Schema),
		},
	}

	if templateDir != "" {
		if err := filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			bs, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			str := os.ExpandEnv(string(bs))
			return json.Unmarshal([]byte(str), &swagger)
		}); err != nil {
			return err
		}
	}
	if swagger.Paths == nil {
		swagger.Paths = &spec.Paths{Paths: make(map[string]spec.PathItem)}
	}
	if swagger.Definitions == nil {
		swagger.Definitions = make(map[string]spec.Schema)
	}

	// Add schemas to definitions
	for name, schema := range api.Schemas {
		swagger.Definitions[name] = schema
	}

	// Process each API endpoint
	for path, methods := range api.Apis {
		pathItem := spec.PathItem{}

		for method, handler := range methods {

			handler.mergeMapping()

			operation := &spec.Operation{
				OperationProps: spec.OperationProps{
					Description: handler.Doc,
					Tags:        strings.Split(handler.With.Label["tag"], ","),
					Parameters:  []spec.Parameter{},
					Responses:   &spec.Responses{ResponsesProps: spec.ResponsesProps{StatusCodeResponses: make(map[int]spec.Response)}},
				},
			}

			// Add path parameters
			for _, arg := range handler.RequestArgs {
				if strings.Contains(path, "{"+arg.Name+"}") {
					param := spec.Parameter{
						ParamProps: spec.ParamProps{
							Name:     arg.Name,
							In:       "path",
							Required: true,
							Schema:   schemaFromType(arg.Type),
						},
					}
					operation.Parameters = append(operation.Parameters, param)
				}
			}

			// Add query parameters
			for _, queryType := range handler.With.BindQuery {
				if schema, ok := api.Schemas[queryType.PackageName]; ok {
					for name, prop := range schema.Properties {
						param := spec.Parameter{
							ParamProps: spec.ParamProps{
								Name:     name,
								In:       "query",
								Required: contains(schema.Required, name),
								Schema:   &prop,
							},
						}
						operation.Parameters = append(operation.Parameters, param)
					}
				}
			}

			// Add header parameters
			for _, headerType := range handler.With.BindHeader {
				if schema, ok := api.Schemas[headerType.PackageName]; ok {
					for name, prop := range schema.Properties {
						param := spec.Parameter{
							ParamProps: spec.ParamProps{
								Name:     name,
								In:       "header",
								Required: contains(schema.Required, name),
								Schema:   &prop,
							},
						}
						operation.Parameters = append(operation.Parameters, param)
					}
				}
			}

			// Add responses
			statusCode := handler.With.HttpCode
			if statusCode == 0 {
				statusCode = 200
			}

			response := spec.Response{
				ResponseProps: spec.ResponseProps{
					Description: "Successful operation",
				},
			}

			// Add response schema if available
			if len(handler.ResponseResult) > 0 {
				response.Schema = schemaFromType(handler.ResponseResult[0].Type)
			}

			operation.Responses.StatusCodeResponses[statusCode] = response

			// Add the operation to the path item based on HTTP method
			switch strings.ToUpper(method) {
			case "GET":
				pathItem.Get = operation
			case "POST":
				pathItem.Post = operation
			case "PUT":
				pathItem.Put = operation
			case "DELETE":
				pathItem.Delete = operation
			case "PATCH":
				pathItem.Patch = operation
			case "HEAD":
				pathItem.Head = operation
			case "OPTIONS":
				pathItem.Options = operation
			}
		}

		swagger.Paths.Paths[path] = pathItem
	}

	return os.WriteFile(filepath.Join(outputDir, "swagger.json"), []byte(Beautify(swagger)), 0664)
}

// Helper function to create a schema from a Type
func schemaFromType(t Type) *spec.Schema {
	if t.IsPrimitive() {
		schema := &spec.Schema{}
		switch t.Name {
		case "string":
			schema.Type = []string{"string"}
		case "int", "int32", "int64":
			schema.Type = []string{"integer"}
			if t.Name == "int64" {
				schema.Format = "int64"
			}
		case "float32", "float64":
			schema.Type = []string{"number"}
			if t.Name == "float64" {
				schema.Format = "double"
			}
		case "bool":
			schema.Type = []string{"boolean"}
		}
		return schema
	}

	// Reference to a defined schema
	return spec.RefSchema("#/definitions/" + t.PackageName)
}

// Helper function to check if a string is in a slice
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
