package Config

import (
	"bytes"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io"
	"log"

	"github.com/spf13/viper"
)

func loadConfigFile(cfgPath, cfgFile string) error {
	if cfgPath != "" {
		viper.AddConfigPath(cfgPath)
	}
	viper.SetConfigFile(cfgFile)
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println(fmt.Errorf("Fatal error config file: Config file not found %w \n", err))
		} else {
			log.Println(fmt.Errorf("Fatal error config file: %w \n", err))
		}
	}

	// 监听配置文件的变化并自动加载
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Println("Config file changed:", e.Name)
	})

	return err
}

func LoadConfigFile(cfgPath, cfgFile string) {
	if err := loadConfigFile(cfgPath, cfgFile); err != nil {
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}
}

func LoadConfigFileViaMultiplePaths(cfgPaths []string, cfgFile string) {
	for _, path := range cfgPaths {
		viper.AddConfigPath(path)
	}
	viper.SetConfigFile(cfgFile)
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println(fmt.Errorf("Fatal error config file: Config file not found %w \n", err))
		} else {
			log.Println(fmt.Errorf("Fatal error config file: %w \n", err))
		}
	}

	// 监听配置文件的变化并自动加载
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Println("Config file changed:", e.Name)
	})

}

func loadConfigByte(data []byte, filetype string) error {
	var err error

	switch filetype {
	case "yaml":
	case "yml":
	case "toml":
	case "json":
	default:
		err = fmt.Errorf("file ext not support")
	}

	if err != nil {
		return err
	}

	viper.SetConfigType(filetype)
	err = viper.ReadConfig(bytes.NewBuffer(data))

	return err
}

func LoadConfigByte(data []byte, filetype string) {
	if err := loadConfigByte(data, filetype); err != nil {
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}
}

// MergeConfig 合并配置 byte
func MergeConfig(byteCfg io.Reader) error {
	return viper.MergeConfig(byteCfg)
}

// MergeConfigWithPath 合并配置 文件路径 cfgPath 文件夹路径
func MergeConfigWithPath(cfgPath string) error {
	// 追加一份配置
	viper.AddConfigPath(cfgPath)
	err := viper.MergeInConfig() // Find and read the config file
	if err != nil {              // Handle errors reading the config file
		return fmt.Errorf("Fatal error config file: %w \n", err)
	} else {
		return nil
	}
}

// MergeConfigWithMap 合并配置 map
func MergeConfigWithMap(cfg map[string]interface{}) error {
	return viper.MergeConfigMap(cfg)
}

// GetEnv 获取 系统环境变量
func GetEnv(key string) interface{} {
	viper.AutomaticEnv()
	return viper.Get(key)
}
