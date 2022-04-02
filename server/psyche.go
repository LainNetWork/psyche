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
	"strings"
	"sync"
	"time"
)

var (
	RepoInitError = errors.New("git仓库初始化失败！")
	//SuffixNotSupportError   = errors.New("不支持的文件格式！")
	//ContextNotInitError     = errors.New("环境尚未初始化，请传入配置对象指针！")
	//ContextInitializedError = errors.New("环境已初始化，请勿重复初始化！")
	//NeedDoublePointInput    = errors.New("为了自动更新配置，请传入配置对象的二级指针！")
)

type ClientConfig struct {
	Url             string // git 仓库地址
	Auth            transport.AuthMethod
	Branch          string        // 分支
	RefreshDuration time.Duration // 自动刷新周期,默认为-1，不开启
}

type Client struct {
	repo          *git.Repository
	clientConfig  *ClientConfig
	configMap     sync.Map // configPath:configText
	currentHead   string   // 仓库当前的Head Hash
	headMu        sync.Mutex
	branchRef     plumbing.ReferenceName
	eventListener map[EventType][]func(...interface{})
}

func NewPsycheClient(opts ...func(config *ClientConfig)) (*Client, error) {
	psycheClient := &Client{}
	psycheClient.eventListener = make(map[EventType][]func(...interface{}))
	auth := &http.BasicAuth{
		Username: os.Getenv("PSYCHE_GIT_USERNAME"),
		Password: os.Getenv("PSYCHE_GIT_PASSWORD"),
	}
	psycheClient.clientConfig = &ClientConfig{
		Url:             os.Getenv("PSYCHE_GIT_URL"),
		Auth:            auth,
		RefreshDuration: time.Duration(0),
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
	psycheClient.branchRef = plumbing.NewBranchReferenceName(psycheClient.clientConfig.Branch)
	clone, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		ReferenceName: psycheClient.branchRef,
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
		Branch: psycheClient.branchRef,
		Force:  true,
	})
	if err != nil {
		log.Println(err.Error())
		return nil, RepoInitError
	}
	psycheClient.repo = clone
	return psycheClient, nil
}

type EventType int

const (
	NewConfigEvent EventType = iota
)

func (psycheClient *Client) PublishEvent(eventType EventType, content interface{}) {
	for _, f := range psycheClient.eventListener[eventType] {
		f(content)
	}
}

func (psycheClient *Client) On(eventType EventType, fun func(...interface{})) {
	listener := psycheClient.eventListener[eventType]
	psycheClient.eventListener[eventType] = append(listener, fun)
}

func (psycheClient *Client) needAutoRefresh() bool {
	return psycheClient.clientConfig.RefreshDuration > 0
}

//StartAutoUpdate 拉取配置，如配置了定时刷新，则启动定时器
func (psycheClient *Client) StartAutoUpdate() {
	if psycheClient.needAutoRefresh() {
		go func() {
			defer func() {
				fmt.Println(recover())
			}()
			ticker := time.NewTicker(psycheClient.clientConfig.RefreshDuration)
			for range ticker.C {
				err := psycheClient.refresh()
				if err != nil {
					log.Println("刷新配置异常！", err.Error())
				}
			}
		}()
	}
}

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
	log.Println("刷新配置")
	worktree, err := psycheClient.repo.Worktree()
	if err != nil {
		return err
	}
	err = worktree.Pull(&git.PullOptions{
		ReferenceName: psycheClient.branchRef,
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
		psycheClient.headMu.Lock()
		psycheClient.currentHead = head.Hash().String()
		psycheClient.configMap.Range(func(key, value interface{}) bool {
			configPath := key.(string)
			config, err2 := psycheClient.getConfigByPath(configPath)
			if err2 != nil {
				log.Println(err2.Error())
			}
			psycheClient.configMap.Store(key, config)
			return false
		})
		psycheClient.PublishEvent(NewConfigEvent, nil)
		psycheClient.headMu.Unlock()
	} else {
		return nil
	}
	return nil
}

func (psycheClient *Client) GetConfig(projectName, env string, suffix string) (*string, error) {
	//缓存中存在的话，从缓存中读取对应环境的配置数据，如果不存在，则从仓库中加载
	load, ok := psycheClient.configMap.Load(psycheClient.GetConfigPath(projectName, env, suffix))
	if ok {
		config := load.(*string)
		return config, nil
	}
	content, err := psycheClient.getConfigContent(projectName, env, suffix)
	if err != nil {
		return nil, err
	}
	psycheClient.configMap.Store(psycheClient.GetConfigPath(projectName, env, suffix), content) // 缓存该环境配置数据
	return content, err
}

func (psycheClient *Client) getConfigByPath(path string) (*string, error) {
	return psycheClient.getConfigContent(psycheClient.divideConfigPath(path))
}

func (psycheClient *Client) getConfigContent(projectName string, env string, suffix string) (*string, error) {
	worktree, err := psycheClient.repo.Worktree()
	if err != nil {
		return nil, err
	}
	open, err := worktree.Filesystem.Open(psycheClient.GetConfigPath(projectName, env, suffix))
	if err != nil {
		return nil, err
	}
	all, err := ioutil.ReadAll(open)
	if err != nil {
		return nil, err
	}
	config := string(all)
	return &config, nil
}

func (psycheClient *Client) GetConfigPath(projectName string, env string, suffix string) string {
	return fmt.Sprintf("%s/%s.%s", projectName, env, suffix)
}

func (psycheClient *Client) divideConfigPath(path string) (string, string, string) {
	f1 := strings.Index(path, "/")
	f2 := strings.LastIndex(path, ".")
	projectName := path[:f1]
	env := path[f1+1 : f2]
	suffix := path[f2+1:]
	return projectName, env, suffix
}
