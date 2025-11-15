package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// These will be set by the build process via ldflags
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// NewVersionCommand creates a new version command
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  "Print the version information for crossbench",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stdout, "crossbench version %s\n", version)
			fmt.Fprintf(os.Stdout, "  commit: %s\n", commit)
			fmt.Fprintf(os.Stdout, "  date: %s\n", date)
			fmt.Fprintf(os.Stdout, "  go: %s\n", runtime.Version())
			fmt.Fprintf(os.Stdout, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}

