package main

import (
	"fmt"
	"os"

	"github.com/gjbravi/crossbench/cmd"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "crossbench",
		Short: "A CLI tool for rendering Crossplane compositions",
		Long:  "crossbench is a CLI tool that provides the same rendering functionalities as Crossplane's render command.",
	}

	// Add render command
	rootCmd.AddCommand(cmd.NewRenderCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

