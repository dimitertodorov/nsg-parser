package cmd

import (
	"github.com/spf13/cobra"
	"github.com/prometheus/common/version"
	"fmt"
	"os"
)

func init() {
	RootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of nsg-parser",
	Long:  `All software has versions. This is nsg-parser's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stdout, version.Print("nsg-parser"))
		os.Exit(0)
	},
}
