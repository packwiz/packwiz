package cmd

import (
	"fmt"
	"github.com/packwiz/packwiz/core"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var packFile string
var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "packwiz",
	Short: "A command line tool for creating Minecraft modpacks",
}

// Execute starts the root command for packwiz
func Execute() {
	if err := rootCmd.Execute(); err != nil {
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
	_ = viper.BindPFlag("pack-file", rootCmd.PersistentFlags().Lookup("pack-file"))

	// Make mods-folder an alias for meta-folder
	viper.RegisterAlias("mods-folder", "meta-folder")
	rootCmd.SetGlobalNormalizationFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "mods-folder" {
			return "meta-folder"
		}
		return pflag.NormalizedName(name)
	})

	var metaFolder string
	rootCmd.PersistentFlags().StringVar(&metaFolder, "meta-folder", "", "The folder in which new metadata files will be added, defaulting to a folder based on the category (mods, resourcepacks, etc; if the category is unknown the current directory is used)")
	_ = viper.BindPFlag("meta-folder", rootCmd.PersistentFlags().Lookup("meta-folder"))

	var metaFolderBase string
	rootCmd.PersistentFlags().StringVar(&metaFolderBase, "meta-folder-base", ".", "The base folder from which meta-folder will be resolved, defaulting to the current directory (so you can put all mods/etc in a subfolder while still using the default behaviour)")
	_ = viper.BindPFlag("meta-folder-base", rootCmd.PersistentFlags().Lookup("meta-folder-base"))

	defaultCacheDir, err := core.GetPackwizCache()
	cacheUsage := "The directory where packwiz will cache downloaded mods"
	if err == nil {
		cacheUsage += "(default \"" + defaultCacheDir + "\")"
	}
	rootCmd.PersistentFlags().String("cache", defaultCacheDir, cacheUsage)
	_ = viper.BindPFlag("cache.directory", rootCmd.PersistentFlags().Lookup("cache"))

	file, err := core.GetPackwizLocalStore()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	file = filepath.Join(file, ".packwiz.toml")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "The config file to use (default \""+file+"\")")

	var nonInteractive bool
	rootCmd.PersistentFlags().BoolVarP(&nonInteractive, "yes", "y", false, "Accept all prompts with the default or \"yes\" option (non-interactive mode) - may pick unwanted options in search results")
	_ = viper.BindPFlag("non-interactive", rootCmd.PersistentFlags().Lookup("yes"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		dir, err := core.GetPackwizLocalStore()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(dir)
		viper.SetConfigName(".packwiz")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
