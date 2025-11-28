package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {

	http.HandleFunc("/ws", handleWebSocket)

	http.ListenAndServe(":8080", nil)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection to WebSocket", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	fmt.Println("New connection")

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		fmt.Println("Message:", string(message))
		// message = append(message, "have recieved"...)
		if string(message) == "你好" {
			err = conn.WriteMessage(messageType, []byte("你好呀 我是智能AI傻逼识别机器人 有什么需要我的帮助？"))
		} else if string(message) == "你是谁" {
			err = conn.WriteMessage(messageType, []byte("你好呀 我是智能AI傻逼识别机器人 有什么需要我的帮助？"))
		} else {
			err = conn.WriteMessage(messageType, []byte("赵涵一傻逼"))
		}
		if err != nil {
			fmt.Println("Error writing message:", err)
			break
		}
	}

	fmt.Println("Connection closed")

}
