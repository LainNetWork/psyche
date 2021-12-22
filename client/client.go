package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"sync"
	"time"
)

type Result struct {
	IsOk bool        `json:"isOk"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type PsycheClient struct {
	conn               *websocket.Conn
	ServerAddr         string
	ProjectName        string
	Env                string
	AutoUpdateDuration time.Duration
	mu                 sync.Mutex
}

func (psyche *PsycheClient) Connect() error {

	u := url.URL{Scheme: "ws", Host: psyche.ServerAddr, Path: fmt.Sprintf("/config/%s/%s", psyche.ProjectName, psyche.Env)}
	dial, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	psyche.conn = dial
	go func() {
		defer func() {
			_ = dial.Close()
			//当panic时不中断整个程序，关闭连接后退出
			recover()
		}()
		for {
			r := &Result{}
			err2 := dial.ReadJSON(r)
			if err2 != nil {
				log.Println("解析消息异常", err.Error())
				continue
			}
		}
	}()
	return nil
}

func (psyche *PsycheClient) GetConfig(value interface{}) {

}
