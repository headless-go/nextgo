package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "nextgo",
	Short: "A lightweight framework for building REST APIs in Go",
	Long: `NextGo is a convention-over-configuration framework for building REST APIs in Go.
It focuses on business logic while handling web framework details automatically.
Complete documentation is available at https://github.com/headless-go/nextgo`,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	rootCmd.AddCommand(swagCodegenCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
