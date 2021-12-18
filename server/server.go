package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/ogier/pflag"
	"log"
)

var client *Client

func init() {
	pflag.String("url", "", "仓库地址")
	pflag.String("branch", "master", "仓库分支")
	pflag.String("suffix", "yml", "配置文件后缀")
}

func main() {
	//err2 := viper.BindEnv()
	c, err := NewPsycheClient(func(config *Config) {
		config.Url = "https://git.lain.fun/config"
		config.Branch = "test"
		config.Suffix = "yml"
		config.Auth = &http.BasicAuth{
			Username: "Rein",
			Password: "IwakuraRein",
		}
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
