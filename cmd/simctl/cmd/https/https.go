package https

import (
	"github.com/spf13/cobra"
)

var HttpsCmd = &cobra.Command{Use: "https"}

func init() {
	HttpsCmd.AddCommand(httpsStartCmd)
	HttpsCmd.AddCommand(httpsStopCmd)
	HttpsCmd.AddCommand(httpsStatusCmd)
}
