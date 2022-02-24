## Psyche - 简易的以Git为配置中心的轻量配置组件

### 主要功能：

1. 使用git作为配置源存储yml格式的配置文件，可根据文件名区分不同的环境
2. 自动更新配置

### 使用方法：
#### 一、部署config-server
1. 创建你的远程git仓库
2. 在仓库创建你的项目配置文件夹，创建配置文件并按环境命名，如：/projectName/dev.yml
3. 配置部署server端，程序根目录配置conf.yml文件

eg:
```yaml
url: https://git.com/repo #配置文件仓库地址
token: 12345689           #token，接口权限校验
refreshDuration: 5        # 刷新频率，单位（秒）
```
#### 二、项目中引用client端

```go
import "github.com/LainNetWork/psyche/client"
```
```go
package main
import "github.com/LainNetWork/psyche/client"

type Config1 struct {
	Profile  string `yaml:"profile"`
	Username string `yaml:"username"`
}

type Config2 struct {
	Password string `yaml:"password"`
	TestName string `yaml:"testName"`
}


var Config1 *Config1
var Config2 *Config2

func init() {
	psycheClient := client.PsycheClient{
		ServerAddr:         os.Getenv("GIT_CONFIG_URL"),// 配置服务器地址
		ProjectName:        "repoName", //仓库名
		Env:                os.Getenv("GO_ENV"),// 环境
		Suffix:             "yml", //配置文件后缀
	}
	//注意，watch对象是配置对象的二级指针
	err := psycheClient.Watch(&Config1) //监听配置对象1
	if err != nil {
		panic(err.Error())
	}
	//可监听多个不同的配置对象
	err = psycheClient.Watch(&Config2) // 监听配置对象2
	if err != nil {
		panic(err.Error())
	}
	err = psycheClient.Connect() // 会先同步更新一次配置，之后由长连接进行推送
	if err != nil {
		panic(err.Error())
	}
}

```

当git仓库被更改，config-server监听到后会推送到各client端。在client端中直接使用配置对象的指针访问最新配置即可
> 注：Watch 二级指针在极端情况下有可能有配置不一致问题，有严格需求的，可以选择使用直接从client里拿最新配置文本的方式

### TodoList

- Server端的更新回调接口，作为定时刷新的备选方案，由git仓库触发更新
- Client端主动获取配置的API，作为替代自动更新二级指针的方案，防止极端情况下或许会出现的不一致问题