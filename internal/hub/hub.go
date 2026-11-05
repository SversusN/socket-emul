// Package hub — эмулятор NFC-ридера: WebSocket-сервер, рассылающий
// сообщения вида {"type":"hash","data":"..."} подключённым клиентам.
package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Msg — сообщение, которое ридер шлёт клиенту.
type Msg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// Hub хранит подключённых websocket-клиентов и рассылает им сообщения.
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	events  chan string // лог-события для TUI
}

// New создаёт Hub; events принимает строки для лога (буферизованный канал).
func New(events chan string) *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{}), events: events}
}

func (h *Hub) logf(format string, args ...any) {
	select {
	case h.events <- fmt.Sprintf(format, args...):
	default:
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeHTTP — endpoint ws://localhost:8181/
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logf("ошибка апгрейда: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	n := len(h.clients)
	h.mu.Unlock()
	h.logf("клиент подключился (всего: %d)", n)

	// Читаем входящие сообщения, чтобы ловить закрытие соединения.
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			n := len(h.clients)
			h.mu.Unlock()
			conn.Close()
			h.logf("клиент отключился (всего: %d)", n)
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// Broadcast отправляет сообщение всем подключённым клиентам.
// Возвращает количество клиентов, которым ушло сообщение.
func (h *Hub) Broadcast(msg Msg) int {
	data, err := json.Marshal(msg)
	if err != nil {
		return 0
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		_ = c.WriteMessage(websocket.TextMessage, data)
	}
	return len(h.clients)
}

// Events возвращает канал лог-событий для TUI.
func (h *Hub) Events() <-chan string { return h.events }

// Logf отправляет строку в лог-событий (неблокирующе).
func (h *Hub) Logf(format string, args ...any) { h.logf(format, args...) }

// ClientCount — текущее число подключённых клиентов.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
