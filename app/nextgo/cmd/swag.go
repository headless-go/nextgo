package cmd

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/spf13/cobra"

	codegen2 "github.com/headless-go/nextgo/app/nextgo/cmd/codegen"
)

var (
	swgOutput      string
	templateOutput string
)

func init() {

	swagCodegenCmd.PersistentFlags().StringVar(&src, "src", "", "your api dir")
	swagCodegenCmd.PersistentFlags().StringVar(&swgOutput, "out", "", "the output dir of swagger doc")
	swagCodegenCmd.PersistentFlags().StringVar(&templateOutput, "template", "", "the template of swagger doc")
}

var swagCodegenCmd = &cobra.Command{
	Use:   "swag",
	Short: "Generate swag doc",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		src, _ = filepath.Abs(src)
		apis, err := codegen2.Parse(src)
		if err != nil {
			log.Fatalln(err)
		}

		outputDir, _ := filepath.Abs(swgOutput)
		if err := codegen2.GenerateSwag(apis, outputDir, templateOutput); err != nil {
			log.Fatalln(fmt.Errorf("generate swagger failed: %v", err))
		}
	},
}
