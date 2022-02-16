package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"time"
)

var (
	NeedDoublePointInput  = errors.New("为了自动更新配置，请传入配置对象的二级指针！")
	SuffixNotSupportError = errors.New("不支持的文件格式！")
	ConnectionFailError   = errors.New("连接配置中心失败！")
	DataFormatError       = errors.New("数据格式异常！")
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
	ServerAddr         string
	ProjectName        string
	Env                string
	Suffix             string
	AutoUpdateDuration time.Duration
	conn               *websocket.Conn
	configContent      string
	configPtrInfo      []configPtrInfo
	configMu           sync.Mutex
}
type configPtrInfo struct {
	configPointPtr interface{}
	configPoint    reflect.Value
	configType     reflect.Type
}

func (psyche *PsycheClient) Connect() error {
	u := url.URL{Scheme: "ws", Host: psyche.ServerAddr, Path: fmt.Sprintf("/config/%s/%s/%s", psyche.ProjectName, psyche.Env, psyche.Suffix)}
	dial, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	psyche.conn = dial
	//通过http接口同步调用一次，初始化配置
	u.Scheme = "http"
	httpUrl := u.String()
	get, err := http.Get(httpUrl)
	if err != nil || get.StatusCode != 200 {
		return ConnectionFailError
	}
	all, err := ioutil.ReadAll(get.Body)
	if err != nil {
		return ConnectionFailError
	}
	result := &Result{}
	err = json.Unmarshal(all, result)
	if err != nil {
		return DataFormatError
	}
	err = psyche.renewWatch(result.Data)
	if err != nil {
		return err
	}
	//建立长连接，接收推送
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
			err2 = psyche.renewWatch(r.Data)
			if err2 != nil {
				log.Println("更新消息异常", err2.Error())
				continue
			}
		}
	}()
	return nil
}

func (psyche *PsycheClient) renewWatch(config string) error {
	psyche.configMu.Lock()
	psyche.configContent = config
	switch psyche.Suffix {
	case "yaml", "yml":
		{
			for _, info := range psyche.configPtrInfo {
				value := reflect.New(info.configType).Interface()
				err := yaml.Unmarshal([]byte(psyche.configContent), value)
				if err != nil {
					return err
				}
				info.configPoint.Set(reflect.ValueOf(value).Convert(reflect.PtrTo(info.configType)))
			}
		}
	default:
		return SuffixNotSupportError
	}
	psyche.configMu.Unlock()
	return nil
}

// Watch 监听对象指针
func (psyche *PsycheClient) Watch(configPtrPtr interface{}) error {
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
	value := reflect.ValueOf(configPtrPtr)
	info := configPtrInfo{
		configPointPtr: configPtrPtr,
		configPoint:    value.Elem(),
		configType:     p,
	}
	psyche.configPtrInfo = append(psyche.configPtrInfo, info)
	return nil
}
