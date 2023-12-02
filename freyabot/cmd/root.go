/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const cmdName = "freyabot"

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "freyabot",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.freyabot.yaml)")
	// Logging flags
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Log level. One of: trace, debug, info, warn, error, fatal, panic")
	rootCmd.PersistentFlags().BoolP("log-pretty", "p", false, "Pretty print logs. Use only for debugging")

	bindAllFlags(rootCmd, true, []string{"log"})

	initLogging()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".freyabot" (without extension).
		viper.AddConfigPath("/etc/freyabot")
		viper.AddConfigPath("$HOME/.config/freyabot")
		viper.AddConfigPath("$HOME/.freyabot")
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func initLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// Set log level
	switch viper.GetString("log.level") {
	case "trace":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		panic("Invalid log level: " + viper.GetString("log.level"))
	}
	log.Info().Msg("Log level set to " + strings.ToUpper(viper.GetString("log.level")))
	// Check if we should pretty print logs
	if viper.GetBool("log.pretty") {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		log.Info().Msg("Pretty print logs enabled")
	}
}

func bindAllFlags(command *cobra.Command, pflags bool, prefixes []string) {
	// Turn prefixes into a regexp pattern
	pattern := "(" + strings.Join(prefixes, "|") + ")-(.*)"
	re := regexp.MustCompile(pattern)

	visit := func(f *pflag.Flag) {
		if !re.MatchString(f.Name) {
			return
		}
		m := re.FindStringSubmatch(f.Name)
		viper.BindPFlag(m[1]+"."+m[2], f)
	}

	if pflags {
		command.PersistentFlags().VisitAll(visit)
	} else {
		command.Flags().VisitAll(visit)
	}
}
