package main

import (
	"context"
	"lagent/internal/agent"
	"lagent/internal/config"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	// 增加缓冲区，避免慢客户端阻塞
	WriteBufferSize: 1024,
	ReadBufferSize:  1024,
}

type WebSocketMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("升级失败: %v", err)
		return
	}
	defer conn.Close()
	log.Println("WebSocket 连接建立")

	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Printf("加载配置失败: %v", err)
		sendError(conn, "配置加载失败: "+err.Error())
		return
	}
	// 创建 Agent（每个连接独立）
	ag, err := agent.NewAgent(cfg)
	if err != nil {
		log.Printf("创建 Agent 失败: %v", err)
		sendError(conn, "Agent初始化失败: "+err.Error())
		return
	}
	log.Println("Agent 初始化成功")

	// 设置读超时（避免僵尸连接）
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		return nil
	})

	for {
		var msg WebSocketMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("读取消息失败: %v", err)
			break
		}
		log.Printf("收到消息: type=%s, content_len=%d", msg.Type, len(msg.Content))

		if msg.Type != "user_message" {
			log.Printf("忽略非用户消息类型: %s", msg.Type)
			continue
		}
		if msg.Content == "" {
			continue
		}

		// 调用 Agent 流式生成
		ctx := context.Background()
		err = ag.StreamGenerate(ctx, msg.Content, func(chunk string) {
			resp := WebSocketMessage{
				Type:    "chunk",
				Content: chunk,
			}
			if err := conn.WriteJSON(resp); err != nil {
				log.Printf("发送 chunk 失败: %v", err)
				// 发生写入错误，终止生成（不再继续发送）
				panic("write error") // 触发 defer 清理
			}
		})
		if err != nil {
			log.Printf("StreamGenerate 错误: %v", err)
			sendError(conn, err.Error())
		} else {
			// 发送完成信号
			conn.WriteJSON(WebSocketMessage{Type: "done"})
			log.Println("流式生成完成")
		}
	}
	log.Println("WebSocket 连接关闭")
}

func sendError(conn *websocket.Conn, errMsg string) {
	conn.WriteJSON(WebSocketMessage{
		Type:    "error",
		Content: errMsg,
	})
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/templates/index.html")
}

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleWebSocket)
	log.Println("Web 服务启动在 http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
