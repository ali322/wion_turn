package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

var App = new(AppConf)

type AppConf struct {
	PublicIP string `yaml:"publicIP"`
	Port     string `yaml:"port"`
	Users    string `yaml:"users"`
	Realm    string `yaml:"realm"`
	LogDir   string `yaml:"logDir"`
	IsTCP    bool   `yaml:"isTCP"`
}

func ReadAppConf() {
	workDir, _ := os.Getwd()
	viper.SetConfigFile(filepath.Join(workDir, "config.yaml"))
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(err)
	}
	if err := viper.Sub("app").Unmarshal(App); err != nil {
		log.Fatal(err)
	}
}
