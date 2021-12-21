package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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
	conn        *websocket.Conn
}

func (w *Server) Start() {
	w.conn = make([]*Conn, 0)
	w.psycheClient.On(NewConfigEvent, w.publishConfig)
	engine := gin.Default()
	engine.GET("/config/:projectName/:env", w.fetchConfig)
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
		config, err := w.psycheClient.GetConfig(conn.projectName, conn.env)
		if err != nil {
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
	conn, err := upgrade.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Println("连接失败", err.Error())
		return
	}
	c := &Conn{
		projectName: projectName,
		env:         env,
		conn:        conn,
	}
	w.connMu.Lock()
	w.conn = append(w.conn, c)
	w.connMu.Unlock()
	// 第一次链接，获取文件进行推送
	file, err := w.psycheClient.GetConfig(projectName, env)
	if err != nil {
		_ = conn.WriteJSON(Result{
			IsOk: false,
			Msg:  "获取配置文件失败！",
		})
		_ = conn.Close()
	} else {
		_ = conn.WriteJSON(Result{
			IsOk: true,
			Msg:  "success",
			Data: file,
		})
	}
}
func (w *Server) HandlerApi(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()
}
