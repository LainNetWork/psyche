## Psyche - 基于Git的简易配置中心

### 主要功能：

1. 使用git作为配置源存储yml格式的配置文件，可根据文件名区分不同的环境
2. 自动更新配置

### 使用方法：

1. 创建你的git仓库
2. 在仓库创建你的项目配置文件夹，创建配置文件并按环境命名，如：/projectName/dev.yml
3. 在需要使用配置的项目中引入本包
 
```go
import "github.com/LainNetWork/psyche"

func init() {
    psycheClient, err := psyche.NewPsycheClient(func(config *psyche.Config) {
        config.ProjectName = "robot-go"
        config.Env = "dev"
    })
}
```
