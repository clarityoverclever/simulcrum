package http

import (
	"github.com/spf13/cobra"
)

var HttpCmd = &cobra.Command{Use: "http"}

func init() {
	HttpCmd.AddCommand(httpStartCmd)
	HttpCmd.AddCommand(httpStopCmd)
	HttpCmd.AddCommand(httpStatusCmd)
}
