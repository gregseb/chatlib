/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/gregseb/freyabot/chat"
	"github.com/gregseb/freyabot/irc"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := context.Background()
		irc, err := irc.New(
			irc.WithNick("freyabot"),
			irc.WithNetwork("irc.rizon.net", 7000),
			//irc.WithNetwork("irc.rizon.net", 6697),
			//irc.WithTLS(&tls.Config{ServerName: "irc.rizon.net"}),
			irc.WithChannel("#freyabot"),
		)
		if err != nil {
			panic(err)
		}
		chat, err := chat.New(
			irc,
		)
		if err != nil {
			panic(err)
		}

		chat.Start(c)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
