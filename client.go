package psyche

import (
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"sync"
	"time"
)

var (
	RepoInitError         = errors.New("git仓库初始化失败！")
	SuffixNotSupportError = errors.New("不支持的文件格式！")
	ContentNotInitError   = errors.New("环境尚未初始化，请传入配置对象指针！")
)

type Config struct {
	Url             string // git 仓库地址
	Auth            transport.AuthMethod
	Branch          string        // 默认分支
	ProjectName     string        // 项目名，即在git中文件夹名以及文件名
	Suffix          string        // 配置文件后缀名，目前仅支持yml和yaml
	Env             string        // 环境名
	RefreshDuration time.Duration // 自动刷新周期,默认为-1，不开启
}

var psycheClient *Client

type Client struct {
	repo               *git.Repository
	clientConfig       *Config
	configCache        interface{} // 指向配置对象指针的指针
	configContentCache string      //读到文件的缓存
	mutex              sync.Mutex
}

func NewPsycheClient(opts ...func(config *Config)) (*Client, error) {
	psycheClient = &Client{}
	auth := &http.BasicAuth{
		Username: os.Getenv("PSYCHE_GIT_USERNAME"),
		Password: os.Getenv("PSYCHE_GIT_PASSWORD"),
	}
	psycheClient.clientConfig = &Config{
		Url:             os.Getenv("PSYCHE_GIT_URL"),
		Suffix:          "yml",
		Auth:            auth,
		RefreshDuration: time.Duration(-1),
	}
	branch := os.Getenv("PSYCHE_GIT_DEFAULT_BRANCH")
	if branch == "" {
		psycheClient.clientConfig.Branch = "master"
	} else {
		psycheClient.clientConfig.Branch = branch
	}
	for _, opt := range opts {
		opt(psycheClient.clientConfig)
	}
	clone, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		ReferenceName: plumbing.NewBranchReferenceName(psycheClient.clientConfig.Branch),
		URL:           psycheClient.clientConfig.Url,
		Auth:          psycheClient.clientConfig.Auth,
	})
	if err != nil {
		log.Println(err.Error())
		return nil, RepoInitError
	}
	worktree, err := clone.Worktree()
	if err != nil {
		log.Println(err.Error())
		return nil, RepoInitError
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(psycheClient.clientConfig.Branch),
		Force:  true,
	})
	if err != nil {
		log.Println(err.Error())
		return nil, RepoInitError
	}
	psycheClient.repo = clone
	return psycheClient, nil
}

// GetCacheConfig 直接获取缓存的配置
func (psycheClient *Client) GetCacheConfig() interface{} {
	return psycheClient.configCache
}

// Init 传入配置对象指针，刷新git仓库更新，如配置了定时刷新，则启动定时器
func (psycheClient *Client) Init(configPtr interface{}) error {
	psycheClient.configCache = configPtr
	if psycheClient.clientConfig.RefreshDuration > 0 {
		go func() {
			ticker := time.NewTicker(psycheClient.clientConfig.RefreshDuration)
			for range ticker.C {
				err := psycheClient.refresh()
				if err != nil {
					log.Println("刷新配置异常！", err.Error())
				}
			}
		}()
	}
	return psycheClient.refresh()
}

// refresh 刷新配置
func (psycheClient *Client) refresh() error {
	if psycheClient.configCache == nil {
		return ContentNotInitError
	}
	worktree, err := psycheClient.repo.Worktree()
	if err != nil {
		return err
	}
	err = worktree.Pull(&git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(psycheClient.clientConfig.Branch),
		Auth:          psycheClient.clientConfig.Auth,
		Force:         true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	open, err := worktree.Filesystem.Open(psycheClient.GetConfigPath())
	if err != nil {
		return err
	}
	all, err := ioutil.ReadAll(open)
	if err != nil {
		return err
	}
	psycheClient.configContentCache = string(all)
	switch psycheClient.clientConfig.Suffix {
	case "yaml", "yml":
		{
			psycheClient.mutex.Lock()
			refValue := reflect.ValueOf(psycheClient.configCache)
			i := refValue.Elem().Interface()
			err := yaml.Unmarshal(all, i)
			psycheClient.mutex.Unlock()
			if err != nil {
				return err
			}
		}
	default:
		return SuffixNotSupportError
	}
	return nil
}

func (psycheClient *Client) GetConfigPath() string {
	config := psycheClient.clientConfig
	return fmt.Sprintf("%s/%s.%s", config.ProjectName, config.Env, config.Suffix)
}
