package cmd

import (
	"fmt"
	"github.com/prometheus/common/version"
	"github.com/spf13/cobra"
	"os"
)

var (
	short bool
)

var versionCmd = &cobra.Command{
	Use:              "version",
	Short:            "Print the version number of nsg-parser and other build information.",
	Long:             `All software has versions. This is nsg-parser's`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {},
	Run: func(cmd *cobra.Command, args []string) {
		runVersion()
		os.Exit(0)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&short, "short", "s", false, "Print shorter version")
}

func runVersion() {
	if short != false {
		fmt.Printf(version.Version)
		return
	}
	fmt.Println(version.Print("nsg-parser"))
	return
}
