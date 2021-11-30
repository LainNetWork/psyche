package psyche

import (
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

var (
	SuffixNotSupportError = errors.New("不支持的文件格式！")
	ContentNotInitError   = errors.New("环境尚未初始化，请传入配置对象指针！")
)

type Config struct {
	Url         string // git 仓库地址
	Username    string // git 用户名
	PassWord    string // git 密码
	Branch      string // 默认分支
	ProjectName string // 项目名，即在git中文件夹名以及文件名
	Suffix      string // 配置文件后缀名，目前仅支持yml和yaml
	Env         string // 环境名
}

var psycheClient *Client

type Client struct {
	repo               *git.Repository
	clientConfig       *Config
	configCache        interface{}
	configContentCache string //读到文件的缓存
}

func NewPsycheClient(opts ...func(config *Config)) *Client {
	psycheClient = &Client{}
	psycheClient.clientConfig = &Config{
		Url:      os.Getenv("PSYCHE_GIT_URL"),
		Username: os.Getenv("PSYCHE_GIT_USERNAME"),
		PassWord: os.Getenv("PSYCHE_GIT_PASSWORD"),
		Suffix:   "yml",
	}
	branch := os.Getenv("PSYCHE_GIT_DEFAULT_BRANCH")
	if branch == "" {
		branch = "master"
	}
	for _, opt := range opts {
		opt(psycheClient.clientConfig)
	}
	options := &git.CloneOptions{
		URL: psycheClient.clientConfig.Url,
		Auth: &http.BasicAuth{
			Username: psycheClient.clientConfig.Username,
			Password: psycheClient.clientConfig.PassWord,
		},
	}
	clone, err := git.Clone(memory.NewStorage(), memfs.New(), options)
	if err != nil {
		panic("git仓库初始化失败！")
	}
	worktree, err := clone.Worktree()
	if err != nil {
		panic("git仓库初始化失败！")
	}
	err = worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(psycheClient.clientConfig.Branch),
		Force:  true,
	})
	psycheClient.repo = clone
	return psycheClient
}

// GetCacheConfig 直接获取缓存的配置
func (psycheClient Client) GetCacheConfig() interface{} {
	return psycheClient.configCache
}

// Init 传入配置对象指针，刷新git仓库更新
func (psycheClient Client) Init(configPtr interface{}) error {
	psycheClient.configCache = configPtr
	return psycheClient.Refresh()
}

// Refresh 刷新配置
func (psycheClient Client) Refresh() error {
	if psycheClient.configCache == nil {
		return ContentNotInitError
	}
	worktree, err := psycheClient.repo.Worktree()
	if err != nil {
		return err
	}
	err = worktree.Pull(&git.PullOptions{
		Auth: &http.BasicAuth{
			Username: psycheClient.clientConfig.Username,
			Password: psycheClient.clientConfig.PassWord,
		},
		Force: true,
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
	switch psycheClient.clientConfig.Suffix {
	case "yaml", "yml":
		{
			err := yaml.Unmarshal(all, psycheClient.configCache)
			if err != nil {
				return err
			}
		}
	default:
		return SuffixNotSupportError
	}
	return nil
}

func (psycheClient Client) GetConfigPath() string {
	config := psycheClient.clientConfig
	return fmt.Sprintf("%s/%s-%s.%s", config.ProjectName, config.ProjectName, config.Env, config.Suffix)
}
