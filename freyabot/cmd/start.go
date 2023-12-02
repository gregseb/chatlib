/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"regexp"
	"strings"

	"github.com/gregseb/freyabot/chat"
	"github.com/gregseb/freyabot/irc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func bindAllFlags(prefixes []string) {
	// Turn prefixes into a regexp pattern
	pattern := "(" + strings.Join(prefixes, "|") + ")-(.*)"
	re := regexp.MustCompile(pattern)

	startCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !re.MatchString(f.Name) {
			// panic
			panic("Flag name " + f.Name + " does not match pattern " + pattern)
		}
		m := re.FindStringSubmatch(f.Name)
		viper.BindPFlag(m[1]+"."+m[2], f)
	})
}

func init() {
	rootCmd.AddCommand(startCmd)
	irc.Flags(startCmd)
	bindAllFlags([]string{irc.ConfigPrefix})
	viper.SetEnvPrefix(cmdName)
	viper.AutomaticEnv()
}
