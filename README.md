## Psyche - 简易的以Git为配置中心的轻量配置组件

### 主要功能：

1. 使用git作为配置源存储yml格式的配置文件，可根据文件名区分不同的环境
2. 自动更新配置

### 使用方法：

1. 创建你的远程git仓库
2. 在仓库创建你的项目配置文件夹，创建配置文件并按环境命名，如：/projectName/dev.yml
3. 在需要使用配置的项目中引入本包，可参考如下代码
 
```go
import (
    "github.com/LainNetWork/psyche"
    "log"
)

type TestConfig struct {
    userName string `yaml:"userName"`
    password string `yaml:"password"`
    Notice struct{
        tags []int64 `yaml:"tags"`
    } `yaml:"notice"`
}
//必须是指针，否则无法自动更新到此引用上
var config = &TestConfig{}

func init() {
    psycheClient, err := psyche.NewPsycheClient(func(config *psyche.Config) {
        // 使用本配置，你需要在git仓库中，按如下路径存放配置文件：
    	// projectName/dev.yml
        config.Url = "https://gitxxxxxxx" //你的git仓库地址
        config.ProjectName = "projectName" //projectName会被识别为仓库中的文件夹名
        config.Env = "dev"  //环境会被识别为配置文件名
        config.Auth = &http.BasicAuth{
            Username: "",
            Password: "",
        }
        //30s自动更新一次配置，小于等于零则不自动更新。更多设置参考 psyche.Config
        config.RefreshDuration = 30 * time.Second
    })
    if err == nil {
        err := psycheClient.Init(config)
        if err != nil {
            log.Println(err.Error())
            panic("start error!")
        }
    }
}
```