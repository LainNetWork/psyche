package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"log"
	http2 "net/http"
	"sync"
)

type Server struct {
	psycheClient *Client
	conn         []*Conn
	Token        string
	connMu       sync.Mutex
}
type Conn struct {
	projectName string
	env         string
	suffix      string
	conn        *websocket.Conn
}

func (w *Server) Start() {
	w.conn = make([]*Conn, 0)
	w.psycheClient.On(NewConfigEvent, w.publishConfig)
	engine := gin.Default()
	engine.GET("/config/:projectName/:env/:suffix", w.fetchConfig)
	engine.GET("/config/update", w.updateConfig)
	err := engine.Run(":8080")
	if err != nil {
		panic(err.Error())
	}
}

func (w *Server) publishConfig(...interface{}) {
	w.connMu.Lock()
	for i := 0; i < len(w.conn); {
		conn := w.conn[i]
		config, err := w.psycheClient.GetConfig(conn.projectName, conn.env, conn.suffix)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		err = conn.conn.WriteJSON(Result{
			IsOk: true,
			Msg:  "success",
			Data: config,
		})
		if err != nil {
			_ = conn.conn.Close()
			log.Println("发送消息异常", err.Error())
			//remove conn
			w.conn = append(w.conn[:i], w.conn[i+1:]...)
		} else {
			i++
		}
	}
	w.connMu.Unlock()
}

var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http2.Request) bool {
		return true
	},
}

func (w *Server) updateConfig(ctx *gin.Context) {
	err := w.psycheClient.refresh()
	if err != nil {
		log.Println("更新失败：", err.Error())
		Error(ctx, "更新失败！")
	}
	Success(ctx, "更新成功！")
}

func (w *Server) fetchConfig(ctx *gin.Context) {
	env := ctx.Param("env")
	if env == "" {
		Error(ctx, "环境参数缺失！")
		return
	}
	projectName := ctx.Param("projectName")
	if projectName == "" {
		Error(ctx, "项目名缺失")
		return
	}
	suffix := ctx.Param("suffix")
	if suffix == "" {
		Error(ctx, "后缀缺失")
		return
	}
	socketUpgrade := websocket.IsWebSocketUpgrade(ctx.Request)
	if socketUpgrade {
		conn, err := upgrade.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			log.Println("连接失败", err.Error())
			return
		}
		c := &Conn{
			projectName: projectName,
			env:         env,
			suffix:      suffix,
			conn:        conn,
		}
		w.connMu.Lock()
		w.conn = append(w.conn, c)
		w.connMu.Unlock()
		w.HandlerApi(conn, projectName, env, suffix)
	} else {
		// http请求时，直接返回配置
		file, err := w.psycheClient.GetConfig(projectName, env, suffix)
		if err != nil {
			log.Println(err.Error())
			Error(ctx, "获取配置文件失败！")
		} else {
			SuccessWithData(ctx, file)
		}
	}

}
func (w *Server) WriteError(conn *websocket.Conn, message string, data interface{}) {
	_ = conn.WriteJSON(Result{
		IsOk: false,
		Msg:  message,
		Data: data,
	})
}

func (w *Server) WriteSuccess(conn *websocket.Conn, message string, data interface{}) {
	_ = conn.WriteJSON(Result{
		IsOk: true,
		Msg:  message,
		Data: data,
	})
}
func (w *Server) HandlerApi(conn *websocket.Conn, projectName string, env string, suffix string) {
	go func() {
		defer func() { _ = conn.Close() }()
		for {
			msgType, p, err := conn.ReadMessage()
			if err != nil {
				log.Println("与客户端连接异常！", err.Error())
				break
			}
			if msgType == websocket.PingMessage {
				_ = conn.WriteMessage(websocket.PongMessage, nil)
				continue
			}
			if msgType == websocket.TextMessage {
				result := &Command{}
				err := jsoniter.Unmarshal(p, result)
				if err != nil {
					log.Println("收到不合规的通讯")
					continue
				}
				if result.Type == FetchConfig {
					config, err := w.psycheClient.GetConfig(projectName, env, suffix)
					if err != nil {
						log.Println(err.Error())
						w.WriteError(conn, "获取配置异常！", nil)
						continue
					}
					w.WriteSuccess(conn, "success", config)
				}
			}
		}
	}()

}
