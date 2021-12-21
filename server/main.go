package main

import (
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"time"
)

func init() {
	pflag.String("url", "", "仓库地址")
	pflag.String("branch", "master", "仓库分支")
	pflag.String("suffix", "yml", "配置文件后缀")
	pflag.String("token", "", "git仓库鉴权token")
	pflag.Int64("refreshDuration", 0, "自动更新周期，单位秒，小于0时不更新")
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic("初始化异常！ " + err.Error())
	}
}

type ServerConfig struct {
	Url             string        `yaml:"url"`
	Token           string        `yaml:"token"`
	Branch          string        `yaml:"branch"`
	Suffix          string        `yaml:"suffix"`
	RefreshDuration time.Duration `yaml:"refreshDuration"`
}

func main() {
	serverConfig := &ServerConfig{}
	viper.SetConfigName("conf")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic("读取配置异常！" + err.Error())
	}
	err = viper.Unmarshal(serverConfig)
	if err != nil {
		panic("加载配置异常！" + err.Error())
	}
	c, err := NewPsycheClient(func(config *Config) {
		config.Url = serverConfig.Url
		if serverConfig.Branch != "" {
			config.Branch = serverConfig.Branch
		}
		if serverConfig.Suffix != "" {
			config.Suffix = serverConfig.Suffix
		}
		if serverConfig.Token != "" {
			config.Auth = &http.TokenAuth{Token: serverConfig.Token}
		}
		config.RefreshDuration = serverConfig.RefreshDuration * time.Second
	})
	if err != nil {
		panic(err.Error())
	}
	c.StartAutoUpdate()
	server := Server{
		psycheClient: c,
	}
	server.Start()
}
