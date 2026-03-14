package cmd

import (
	"fmt"
	"os"
	"simulacrum/cmd/simctl/cmd/dns"
	"simulacrum/cmd/simctl/cmd/http"
	"simulacrum/cmd/simctl/cmd/https"
	"simulacrum/cmd/simctl/cmd/ntp"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "simctl",
	Short: "control plane for simulacrum",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(dns.DnsCmd)
	rootCmd.AddCommand(http.HttpCmd)
	rootCmd.AddCommand(https.HttpsCmd)
	rootCmd.AddCommand(ntp.NtpCmd)
}
