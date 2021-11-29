package client

import (
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"os"
)

type config struct {
	Url      string // git 仓库地址
	Username string // git 用户名
	PassWord string // git 密码
	Branch   string // 默认分支
}

var psycheClientConfig *config

type PsycheClient struct {
}

func init() {
	psycheClientConfig = &config{
		Url:      os.Getenv("PSYCHE_GIT_URL"),
		Username: os.Getenv("PSYCHE_GIT_USERNAME"),
		PassWord: os.Getenv("PSYCHE_GIT_PASSWORD"),
	}
	branch := os.Getenv("PSYCHE_GIT_DEFAULT_BRANCH")
	if branch == "" {
		branch = "master"
	}

	options := &git.CloneOptions{
		URL: psycheClientConfig.Url,
		Auth: &http.BasicAuth{
			Username: psycheClientConfig.Username,
			Password: psycheClientConfig.PassWord,
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
		Branch: plumbing.ReferenceName(psycheClientConfig.Branch),
		Force:  true,
	})

}

func Refresh() {

}
