package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"

	codegen2 "github.com/headless-go/nextgo/app/nextgo/cmd/codegen"
)

var (
	src    string
	output string
)

func init() {
	apiCmd.AddCommand(generateCmd)
	apiCmd.AddCommand(createCmd)

	generateCmd.PersistentFlags().StringVar(&src, "src", "", "your api dir")
	generateCmd.PersistentFlags().StringVar(&output, "out", "", "the output of generated code your api dir")
}

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "API related commands",
	Long:  `Create and generate REST API endpoints using directory-based routing.`,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate API code from existing endpoints",
	Long:  `Generate router and handler code from your API directory structure.`,
	Run:   runGenerate,
}

var createCmd = &cobra.Command{
	Use:   "create [path]",
	Short: "Create a new API endpoint",
	Long: `Create a new API endpoint with the specified path.
Example: nextgo api create users/{userId}/profile`,
	Args: cobra.ExactArgs(1),
	Run:  runCreate,
}

func runGenerate(cmd *cobra.Command, args []string) {
	src, _ = filepath.Abs(src)
	output, _ = filepath.Abs(output)

	goModeFile, err := findGoMod(output)
	if err != nil {
		log.Fatalln(err)
	}
	projectDir := filepath.Dir(goModeFile)

	modName, err := getModuleName(goModeFile)
	if err != nil {
		log.Fatalln(err)
	}

	output = filepath.Join(output, filepath.Base(src))
	modNamePrefix := filepath.Join(modName, strings.TrimPrefix(output, projectDir))

	apis, err := codegen2.Parse(src)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(codegen2.Beautify(apis.Apis))

	if err := codegen2.Generate(src, output, modNamePrefix, apis); err != nil {
		log.Fatalln(err)
	}
}

func runCreate(cmd *cobra.Command, args []string) {
	path := args[0]

	// Convert path to directory structure
	apiPath := filepath.Join("api", path)

	// Create directories
	if err := os.MkdirAll(filepath.Dir(apiPath), 0755); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Extract path parameters
	parts := strings.Split(path, "/")
	params := make([]string, 0)
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			param := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			params = append(params, param)
		}
	}

	// Create handler file
	handlerFile := filepath.Join(apiPath, "handler.go")
	if err := createHandlerFile(handlerFile, params); err != nil {
		log.Fatalf("Failed to create handler file: %v", err)
	}

	fmt.Printf("Created new API endpoint at %s\n", apiPath)
}

func createHandlerFile(path string, params []string) error {
	tmpl := template.Must(template.New("handler").Parse(`package {{ .Package }}

import (
	"context"
)

type Request struct {
	// Add your request fields here
}

type Response struct {
	// Add your response fields here
}

func Handler(ctx context.Context{{ range .Params }}, {{ . }} string{{ end }}, req Request) (*Response, error) {
	return &Response{}, nil
}
`))

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	data := struct {
		Package string
		Params  []string
	}{
		Package: filepath.Base(filepath.Dir(path)),
		Params:  params,
	}

	return tmpl.Execute(f, data)
}

// findGoMod find go.mod path
func findGoMod(dir string) (string, error) {
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parentDir
	}
}

func getModuleName(modPath string) (string, error) {
	data, err := os.ReadFile(modPath)
	if err != nil {
		return "", err
	}

	modFile, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return "", err
	}
	return modFile.Module.Mod.Path, nil
}
