package main

import (
	"fmt"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"socket-emul/internal/hub"
)

func main() {
	h := hub.New(make(chan string, 64))

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", h.ServeHTTP)
		if err := http.ListenAndServe(":8181", mux); err != nil {
			h.Logf("ОШИБКА сервера: %v", err)
		}
	}()

	p := tea.NewProgram(newModel(h))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка запуска TUI:", err)
		os.Exit(1)
	}
}
