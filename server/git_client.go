package main

import (
	"errors"
	"fmt"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

var (
	RepoInitError = errors.New("git仓库初始化失败！")
	//SuffixNotSupportError   = errors.New("不支持的文件格式！")
	//ContextNotInitError     = errors.New("环境尚未初始化，请传入配置对象指针！")
	//ContextInitializedError = errors.New("环境已初始化，请勿重复初始化！")
	//NeedDoublePointInput    = errors.New("为了自动更新配置，请传入配置对象的二级指针！")
)

type Config struct {
	Url         string // git 仓库地址
	Auth        transport.AuthMethod
	Branch      string // 分支
	ProjectName string // 项目名，即在git中文件夹名以及文件名
	Suffix      string // 配置文件后缀名，目前仅支持yml和yaml
	//Env             string        // 环境名
	//RefreshDuration time.Duration // 自动刷新周期,默认为-1，不开启
}

var psycheClient *Client

type Client struct {
	repo         *git.Repository
	clientConfig *Config
	configMap    sync.Map // env:configText
	currentHead  string   // 仓库当前的Head Hash
	needUpdate   bool
	//configPointPtr interface{}   // 指向配置对象指针的指针
	//configPoint    reflect.Value //指向配置对象指针的Value缓存
	//configType     reflect.Type
	//watched        bool   // 监听到配置变动的同时，是否刷新配置对象。目前只允许监听一个配置对象
	//configContent  []byte // 读到最新的配置文件的缓存
}

func NewPsycheClient(opts ...func(config *Config)) (*Client, error) {
	psycheClient = &Client{}
	auth := &http.BasicAuth{
		Username: os.Getenv("PSYCHE_GIT_USERNAME"),
		Password: os.Getenv("PSYCHE_GIT_PASSWORD"),
	}
	psycheClient.clientConfig = &Config{
		Url:    os.Getenv("PSYCHE_GIT_URL"),
		Suffix: "yml",
		Auth:   auth,
		//RefreshDuration: time.Duration(-1),
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

//// GetConfig 直接获取缓存的配置
//func (psycheClient *Client) GetConfig() string {
//	return string(psycheClient.configContent)
//}
//
//func (psycheClient *Client) needAutoRefresh() bool {
//	return psycheClient.clientConfig.RefreshDuration > 0
//}

//func (psycheClient *Client) Watch(configPtrPtr interface{}) error {
//	if psycheClient.watched {
//		return ContextInitializedError
//	}
//	//判断是否是二级指针，获取对象struct类型
//	of := reflect.TypeOf(configPtrPtr)
//	var p = of
//	var count = 0
//	for p.Kind() == reflect.Ptr {
//		count++
//		p = p.Elem()
//		if p.Kind() == reflect.Struct {
//			break
//		}
//	}
//	if count != 2 {
//		return NeedDoublePointInput
//	}
//	psycheClient.configPointPtr = configPtrPtr
//	value := reflect.ValueOf(configPtrPtr)
//	psycheClient.configPoint = value.Elem() // 配置对象的指针
//	psycheClient.configType = p
//	psycheClient.watched = true
//	return nil
//}

// Start 拉取配置，如配置了定时刷新，则启动定时器
//func (psycheClient *Client) Start() error {
//	err := renew()
//	if err != nil {
//		return err
//	}
//	if psycheClient.needAutoRefresh() {
//		go func() {
//			ticker := time.NewTicker(psycheClient.clientConfig.RefreshDuration)
//			for range ticker.C {
//				err := renew()
//				if err != nil {
//					log.Println("刷新配置异常！", err.Error())
//				}
//			}
//		}()
//	}
//	return nil
//}

//func renew() error {
//	err, hasNew := psycheClient.refresh()
//	if err != nil {
//		return err
//	}
//	//配置监听并判断是否有更新
//	if psycheClient.watched && hasNew {
//		err = psycheClient.refreshConfig()
//		if err != nil {
//			return err
//		}
//	}
//	return nil
//}

// refresh 拉取最新配置
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
	head, err := psycheClient.repo.Head()
	if err != nil {
		return err
	}
	if psycheClient.currentHead != head.Hash().String() {
		psycheClient.currentHead = head.Hash().String()
		psycheClient.needUpdate = true
	} else {
		return nil
	}
	return nil
}

//func (psycheClient *Client) refreshConfig() error {
//	if !psycheClient.watched {
//		return ContextNotInitError
//	}
//	switch psycheClient.clientConfig.Suffix {
//	case "yaml", "yml":
//		{
//			value := reflect.New(psycheClient.configType).Interface()
//			err := yaml.Unmarshal(psycheClient.configContent, value)
//			if err != nil {
//				return err
//			}
//			psycheClient.configPoint.Set(reflect.ValueOf(value).Convert(reflect.PtrTo(psycheClient.configType)))
//		}
//	default:
//		return SuffixNotSupportError
//	}
//	return nil
//}

func (psycheClient *Client) GetConfigFile(env string) (string, error) {
	//不需要更新的话，从缓存中读取对应环境的配置数据，如果不存在，则从仓库中加载
	if psycheClient.needUpdate == false {
		load, ok := psycheClient.configMap.Load(env)
		if ok {
			return load.(string), nil
		}
	}
	worktree, err := psycheClient.repo.Worktree()
	if err != nil {
		return "", err
	}
	open, err := worktree.Filesystem.Open(psycheClient.GetConfigPath(env))
	if err != nil {
		return "", err
	}
	all, err := ioutil.ReadAll(open)
	if err != nil {
		return "", err
	}
	config := string(all)
	psycheClient.configMap.Store(env, config) // 缓存该环境配置数据
	return config, err
}

func (psycheClient *Client) GetConfigPath(env string) string {
	config := psycheClient.clientConfig
	return fmt.Sprintf("%s/%s.%s", config.ProjectName, env, config.Suffix)
}
