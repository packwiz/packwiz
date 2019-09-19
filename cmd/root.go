package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var packFile string
var modsFolder string
var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "packwiz",
	Short: "A command line tool for creating Minecraft modpacks",
}

// Execute starts the root command for packwiz
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Add adds a new command as a subcommand to packwiz
func Add(newCommand *cobra.Command) {
	rootCmd.AddCommand(newCommand)
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&packFile, "pack-file", "pack.toml", "The modpack metadata file to use")
	viper.BindPFlag("pack-file", rootCmd.PersistentFlags().Lookup("pack-file"))

	rootCmd.PersistentFlags().StringVar(&modsFolder, "mods-folder", "mods", "The default folder to store mod metadata files in")
	viper.BindPFlag("mods-folder", rootCmd.PersistentFlags().Lookup("mods-folder"))

	file, err := os.UserConfigDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	file = filepath.Join(file, "packwiz", ".packwiz.toml")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "The config file to use (default \""+file+"\")")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(filepath.Join(dir, "packwiz"))
		viper.SetConfigName(".packwiz")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
