package main

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
	"log"
	"net/url"
	"reflect"
	"sync"
	"time"
)

var (
	NeedDoublePointInput  = errors.New("为了自动更新配置，请传入配置对象的二级指针！")
	SuffixNotSupportError = errors.New("不支持的文件格式！")
	WatchedError          = errors.New("已注册用于自动更新的指针，请勿重复调用")
)

type Result struct {
	IsOk bool   `json:"isOk"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

type CommandType int

type Command struct {
	Type   CommandType `json:"type"`
	Suffix string      `json:"suffix"`
}

type PsycheClient struct {
	conn               *websocket.Conn
	ServerAddr         string
	ProjectName        string
	Env                string
	Suffix             string
	watched            bool
	configContent      string
	configPointPtr     interface{}
	configPoint        reflect.Value
	configType         reflect.Type
	AutoUpdateDuration time.Duration
	configMu           sync.Mutex
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

func (psyche *PsycheClient) fetchConfig() error {
	r := &Result{}
	err := psyche.conn.ReadJSON(r)
	if err != nil {
		return err
	}
	if r.IsOk {
		switch psyche.Suffix {
		case "yaml", "yml":
			{
				value := reflect.New(psyche.configType).Interface()
				err := yaml.Unmarshal([]byte(psyche.configContent), value)
				if err != nil {
					return err
				}
				if psyche.watched {
					psyche.configPoint.Set(reflect.ValueOf(value).Convert(reflect.PtrTo(psyche.configType)))
				}
			}
		default:
			return SuffixNotSupportError
		}
	}
	return nil
}

// Watch 监听对象指针
func (psyche *PsycheClient) Watch(configPtrPtr interface{}) error {
	if psyche.watched {
		return WatchedError
	}
	//判断传入的是否是二级指针，获取对象struct类型
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
	psyche.configPointPtr = configPtrPtr
	value := reflect.ValueOf(configPtrPtr)
	psyche.configPoint = value.Elem() // 配置对象的指针
	psyche.configType = p
	psyche.watched = true
	return nil
}
