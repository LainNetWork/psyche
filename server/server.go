package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"log"
)

var client *Client

func main() {
	c, err := NewPsycheClient(func(config *Config) {
		config.Url = "https://git.lain.fun/config"
		config.ProjectName = "robot-go"
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
	engine.GET("/config/:env", fetchConfig)
	err = engine.Run(":8080")
	if err != nil {
		panic(err.Error())
	}
}

func fetchConfig(ctx *gin.Context) {
	env := ctx.Param("env")
	file, err := client.GetConfigFile(env)
	if err != nil {
		log.Println(file)
		Error(ctx, "获取配置文件失败！")
	}
	SuccessWithData(ctx, file)
}
