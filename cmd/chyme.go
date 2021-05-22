package main

import (
	"github.com/spf13/cobra"
)

var MainCmd = &cobra.Command{
	Use: "chyme",
	Short: "extract transform load",
	Version: "1",
}