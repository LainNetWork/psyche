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
    UserName string `yaml:"username"`
    Password string `yaml:"password"`
    Notice struct{
        Tags []int64 `yaml:"tags"`
    } `yaml:"notice"`
}
//必须是指针，否则无法自动更新到此引用上
var Config = &TestConfig{}
func init() {
    psycheClient, err := psyche.NewPsycheClient(func(config *psyche.Config) {
        // 使用本配置，你需要在git仓库中，按如下路径存放配置文件：
    	// projectName/dev.yml
        config.Url = "https://gitxxxxxxx" //你的git仓库地址
        config.ProjectName = "projectName" //projectName会被识别为仓库中的文件夹名
        config.Env = "dev"  //环境会被识别为配置文件名
        config.Auth = &http.BasicAuth{ // 根据实际情况配置，支持git-go所支持的鉴权方式
            Username: "UserName",
            Password: "PassWord",
        }
        //30s自动更新一次配置，小于等于零则不自动更新，默认为-1。更多设置参考 psyche.Config
        config.RefreshDuration = 30 * time.Second
    })
    if err != nil {
        panic(err.Error())
    }
    // 如需要自动更新项目中的配置，调用本方法监听。需要传入配置对象指针的指针，即二级指针，否则会返回错误
    // 如果不调本方法更新配置对象也可，可以通过GetConfig方法获取最新的配置文件的文本
    err = psycheClient.Watch(&Config)
    if err != nil {
        panic(err.Error())
    }
    err = psycheClient.Start()
    if err != nil {
        panic(err.Error())
    }
}
```
