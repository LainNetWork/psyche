package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"log"
	"time"
)

var client *Client

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
		config.Branch = serverConfig.Branch
		config.Suffix = serverConfig.Suffix
		if serverConfig.Token != "" {
			config.Auth = &http.TokenAuth{Token: serverConfig.Token}
		}
		config.RefreshDuration = serverConfig.RefreshDuration * time.Second
	})
	if err != nil {
		panic(err.Error())
	}
	client = c

	engine := gin.Default()
	engine.GET("/config/:projectName/:env", fetchConfig)
	engine.GET("/config/update", updateConfig)
	err = engine.Run(":8080")
	if err != nil {
		panic(err.Error())
	}
}

func updateConfig(ctx *gin.Context) {
	err := client.refresh()
	if err != nil {
		log.Println("更新失败：", err.Error())
		Error(ctx, "更新失败！")
	}
	Success(ctx, "更新成功！")
}

func fetchConfig(ctx *gin.Context) {
	env := ctx.Param("env")
	if env == "" {
		Error(ctx, "环境参数缺失！")
		return
	}
	projectName := ctx.Param("projectName")
	if projectName == "" {
		Error(ctx, "项目名缺失")
		return
	}

	file, err := client.GetConfigFile(projectName, env)
	if err != nil {
		log.Println(file)
		Error(ctx, "获取配置文件失败！")
	}
	SuccessWithData(ctx, file)
}
