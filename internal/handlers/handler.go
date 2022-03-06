package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/CloudyKit/jet/v6"
	"github.com/gorilla/websocket"
)

type WebSocketConnection struct {
	*websocket.Conn
}

type WsJsonPayload struct {
	Action   string              `json:"action"`
	Username string              `json:"username"`
	Message  string              `json:"message"`
	Conn     WebSocketConnection `json:"-"`
}

var views = jet.NewSet(
	jet.NewOSFileSystemLoader("./html"),
	jet.InDevelopmentMode(),
)

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var wsChan = make(chan WsJsonPayload)
var clients = make(map[WebSocketConnection]string)

// WsJsonResponse defines the response sent back from websocket
type WsJsonResponse struct {
	Action         string   `json:"action"`
	Message        string   `json:"message"`
	MessageType    string   `json:"messageType"`
	ConnectedUsers []string `json:"connectedUsers"`
}

func Home(w http.ResponseWriter, r *http.Request) {
	err := renderPage(w, "home.jet", nil)
	if err != nil {
		log.Println("renderPage error: ", err)
	}
}

// WsEndpoint upgrades connection to websocket
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WsEndpoint/Upgrade error: ", err)
	}

	log.Println("Client connected to endpoint")

	var response WsJsonResponse
	response.Message = "<em><small>Connected to server</small></em>"

	conn := WebSocketConnection{Conn: ws}
	clients[conn] = ""

	err = ws.WriteJSON(response)
	if err != nil {
		log.Println("WsEndpoint/WriteJSON error: ", err)
	}

	go ListenForWs(&conn)
}

func ListenForWs(conn *WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("ListenForWs error: ", fmt.Sprintf("%v", r))
		}
	}()

	var payload WsJsonPayload

	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			// do nothing
		} else {
			payload.Conn = *conn
			wsChan <- payload
		}
	}
}

func ListenToWsChannel() {
	var response WsJsonResponse

	for {
		event := <-wsChan

		switch event.Action {
		case "username":
			// Get a list of all users and send it back via broadcast
			clients[event.Conn] = event.Username

			users := getUserList()
			response.Action = "listUsers"
			response.ConnectedUsers = users
			broadcastToAll(response)

		case "left":
			response.Action = "listUsers"
			delete(clients, event.Conn)
			users := getUserList()
			response.ConnectedUsers = users
			broadcastToAll(response)
		}
	}
}

func getUserList() []string {
	var userList []string

	for _, client := range clients {
		if client != "" {
			userList = append(userList, client)
		}
	}

	sort.Strings(userList)
	return userList
}

func broadcastToAll(response WsJsonResponse) {
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			log.Println("broadcastToAll error: ", err)
			_ = client.Close()
			delete(clients, client)
		}
	}
}

func renderPage(w http.ResponseWriter, tmpl string, data jet.VarMap) error {
	view, err := views.GetTemplate(tmpl)
	if err != nil {
		log.Println("GetTemplate error: ", err)
		return err
	}

	err = view.Execute(w, data, nil)
	if err != nil {
		log.Println("Execute error: ", err)
		return err
	}

	return nil
}
