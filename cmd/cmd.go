package cmd

import (
	"github.com/spf13/cobra"
)

var MainCmd = &cobra.Command{
	Use: "vault_aws",
	Short: "extract transform load",
	Version: "1",
}