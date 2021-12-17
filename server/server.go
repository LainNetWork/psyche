package main

import (
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func main() {
	client, err := NewPsycheClient(func(config *Config) {
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
	file, err := client.GetConfigFile("prod123")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(file)
	//engine := gin.Default()
	//engine.GET("/config/:env")
	//err := engine.Run(":8080")
	//if err != nil {
	//	panic(err.Error())
	//}
}

//func Handler(ctx gin.Context)  {
//	env := ctx.Param("env")
//
//}
