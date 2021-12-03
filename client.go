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
	NeedDoublePointInput  = errors.New("为了自动更新配置，请传入配置对象的二级指针！")
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
	repo           *git.Repository
	clientConfig   *Config
	configPointPtr interface{}   // 指向配置对象指针的指针
	configPoint    reflect.Value //指向配置对象指针的Value缓存
	configType     reflect.Type
	initialized    bool   // 上下文是否初始化，目前只允许监听一个配置对象，初始化后不得更改
	configContent  []byte // 读到配置文件的缓存
	mutex          sync.Mutex
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

// GetConfig 直接获取缓存的配置
func (psycheClient *Client) GetConfig() string {
	return string(psycheClient.configContent)
}

func (psycheClient *Client) needAutoRefresh() bool {
	return psycheClient.clientConfig.RefreshDuration > 0
}

func (psycheClient *Client) Watch(configPtrPtr interface{}) error {
	of := reflect.TypeOf(configPtrPtr)
	var p = of
	var count = 0
	for p.Kind() == reflect.Ptr {
		count++
		p = p.Elem()
		if p.Kind() == reflect.Struct {
			break
		}
	}
	if count != 2 {
		return NeedDoublePointInput
	}
	psycheClient.configPointPtr = configPtrPtr
	value := reflect.ValueOf(configPtrPtr) // 配置对象的指针
	psycheClient.configPoint = value.Elem()
	psycheClient.configType = p
	psycheClient.initialized = true
	return nil
}

// Init 拉取配置，如配置了定时刷新，则启动定时器
func (psycheClient *Client) Init() error {
	if psycheClient.needAutoRefresh() {
		go func() {
			ticker := time.NewTicker(psycheClient.clientConfig.RefreshDuration)
			for range ticker.C {
				err := psycheClient.refresh()
				if err != nil {
					log.Println("刷新配置异常！", err.Error())
					return
				}
				err = psycheClient.refreshConfig()
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
	head, err := psycheClient.repo.Head()
	if err != nil {
		return err
	}
	log.Println(head.Hash())
	psycheClient.configContent = all
	return nil
}

func (psycheClient *Client) refreshConfig() error {
	if !psycheClient.initialized {
		return ContentNotInitError
	}
	switch psycheClient.clientConfig.Suffix {
	case "yaml", "yml":
		{
			value := reflect.New(psycheClient.configType).Interface()
			err := yaml.Unmarshal(psycheClient.configContent, value)
			if err != nil {
				return err
			}
			psycheClient.configPoint.Set(reflect.ValueOf(value).Convert(reflect.PtrTo(psycheClient.configType)))
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
