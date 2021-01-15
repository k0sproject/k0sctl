/*
Copyright 2021 Mirantis, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0sctl/config"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	debug   bool
	// Config represents a desired configuration of the k0s cluster
	Config config.ClusterConfig
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "k0sctl",
	Short: "A tool to manage k0s cluster operations",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// initialize configuration
		err := initConfig()
		if err != nil {
			fmt.Printf("err: %v", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.k0sctl.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Sets log level to DEBUG. Default set to FATAL")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	// look for k0s.yaml in PWD
	if cfgFile == "" {
		execFolderPath, err := os.Getwd()
		if err != nil {
			return err
		}
		cfgFile = filepath.Join(execFolderPath, "k0s.yaml")
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// check if config file exists
	if fileExists(cfgFile) {
		viper.SetConfigFile(cfgFile)
	}

	// Add env vars to Config
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		viper.ConfigFileUsed()
	}

	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("error parsing config %s", err)
	}

	return nil
}

func fileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
