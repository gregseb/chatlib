/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/gregseb/freyabot/chat"
	"github.com/gregseb/freyabot/irc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		chatOpts := make([]chat.Option, 0)
		if co, err := irc.Init(); err != nil {
			panic(err)
		} else if co != nil {
			chatOpts = append(chatOpts, *co)
		}
		chat, err := chat.New(chatOpts...)
		if err != nil {
			panic(err)
		}

		chat.Start(c)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	irc.Flags(startCmd)
	bindAllFlags(startCmd, false, []string{irc.ConfigPrefix})
	viper.SetEnvPrefix(cmdName)
	viper.AutomaticEnv()
}
