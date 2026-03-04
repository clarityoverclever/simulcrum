package http

import (
	"fmt"
	"simulacrum/internal/core"

	"github.com/spf13/cobra"
)

var httpStartCmd = &cobra.Command{
	Use: "start",
	Run: func(cmd *cobra.Command, args []string) {
		message := core.ControlMessage{Service: "http", Action: "start"}
		err := core.SendControlMessage(message)
		if err != nil {
			fmt.Errorf("Failed to send message: %v", err)
		}
	},
}
