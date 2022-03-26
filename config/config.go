package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

func LoadConfigFile(cfgFile string) error {
	viper.SetConfigFile(cfgFile)
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println(fmt.Errorf("Fatal error config file: Config file not found %w \n", err))
		} else {
			log.Println(fmt.Errorf("Fatal error config file: %w \n", err))
		}
	}
	return err
}
