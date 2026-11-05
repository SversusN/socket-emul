package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"socket-emul/internal/hub"
)

const maxLogLines = 200

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSpamOn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	styleHashVal = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

type (
	logMsg    string
	countMsg  int
	spamTick  struct{ sent int }
	serverErr struct{ err error }
)

type model struct {
	hub      *hub.Hub
	hashIn   textinput.Model
	uidIn    textinput.Model
	focused  int // 0 — hash, 1 — uid
	clients  int
	spamming bool
	interval time.Duration
	log      []string
}

func newModel(h *hub.Hub) tea.Model {
	hashIn := textinput.New()
	hashIn.Placeholder = "50903EF31F25667F2A70F0773E52B6FBC51160C7"
	hashIn.SetValue("50903EF31F25667F2A70F0773E52B6FBC51160C7")
	hashIn.Focus()
	hashIn.CharLimit = 64
	hashIn.Width = 48

	uidIn := textinput.New()
	uidIn.Placeholder = "00000000000280A0"
	uidIn.SetValue("00000000000280A0")
	uidIn.CharLimit = 16
	uidIn.Width = 48

	return model{
		hub:      h,
		hashIn:   hashIn,
		uidIn:    uidIn,
		interval: 300 * time.Millisecond,
		log:      []string{},
	}
}

func waitLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg { return logMsg(<-ch) }
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		waitLog(m.hub.Events()),
		tea.Every(time.Second, func(t time.Time) tea.Msg { return countMsg(m.hub.ClientCount()) }),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "shift+tab":
			if m.focused == 0 {
				m.focused = 1
				m.hashIn.Blur()
				m.uidIn.Focus()
			} else {
				m.focused = 0
				m.uidIn.Blur()
				m.hashIn.Focus()
			}
			return m, nil
		case "f2":
			m.send("version", "0.4")
			return m, nil
		case "f3":
			m.send("uid", strings.ToUpper(m.uidIn.Value()))
			return m, nil
		case "f4", "enter":
			m.sendHash()
			return m, nil
		case "f5":
			return m.toggleSpam()
		case "+", "=":
			if m.interval < 5*time.Second {
				m.interval += 50 * time.Millisecond
				m.appendLog(styleDim.Render(fmt.Sprintf("интервал спама: %v", m.interval)))
			}
			return m, nil
		case "-":
			if m.interval > 50*time.Millisecond {
				m.interval -= 50 * time.Millisecond
				m.appendLog(styleDim.Render(fmt.Sprintf("интервал спама: %v", m.interval)))
			}
			return m, nil
		}

	case logMsg:
		m.appendLog(string(msg))
		return m, waitLog(m.hub.Events())

	case countMsg:
		m.clients = int(msg)
		return m, tea.Every(time.Second, func(t time.Time) tea.Msg { return countMsg(m.hub.ClientCount()) })

	case spamTick:
		if m.spamming {
			n := m.sendHash()
			return m, tea.Tick(m.interval, func(t time.Time) tea.Msg {
				return spamTick{sent: n}
			})
		}
		return m, nil

	case serverErr:
		m.appendLog(styleErr.Render(fmt.Sprintf("сервер: %v", msg.err)))
		return m, nil
	}

	var cmd tea.Cmd
	if m.focused == 0 {
		m.hashIn, cmd = m.hashIn.Update(msg)
	} else {
		m.uidIn, cmd = m.uidIn.Update(msg)
	}
	return m, cmd
}

func (m *model) appendLog(s string) {
	m.log = append(m.log, s)
	if len(m.log) > maxLogLines {
		m.log = m.log[len(m.log)-maxLogLines:]
	}
}

func (m *model) send(typ, data string) int {
	n := m.hub.Broadcast(hub.Msg{Type: typ, Data: data})
	m.appendLog(fmt.Sprintf("→ {%s: '%s', data: '%s'} — клиентов: %d",
		styleDim.Render("type"), styleOK.Render(typ), styleHashVal.Render(data), n))
	return n
}

func (m *model) sendHash() int {
	hash := strings.ToUpper(strings.TrimSpace(m.hashIn.Value()))
	if hash == "" {
		m.appendLog(styleErr.Render("хэш пустой — введите значение"))
		return 0
	}
	return m.send("hash", hash)
}

func (m model) toggleSpam() (tea.Model, tea.Cmd) {
	m.spamming = !m.spamming
	if m.spamming {
		m.appendLog(styleSpamOn.Render(fmt.Sprintf("СПАМ включён (каждые %v)", m.interval)))
		return m, tea.Tick(m.interval, func(t time.Time) tea.Msg { return spamTick{} })
	}
	m.appendLog(styleDim.Render("спам выключен"))
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	title := styleTitle.Render("NFC-ридер эмулятор")
	addr := styleDim.Render("ws://localhost:8181")
	clients := styleDim.Render(fmt.Sprintf("клиентов: %d", m.clients))
	b.WriteString(fmt.Sprintf("%s  %s   %s\n\n", title, addr, clients))

	focusMark := func(i int) string {
		if m.focused == i {
			return styleOK.Render("▶")
		}
		return " "
	}
	b.WriteString(fmt.Sprintf("%s Хэш карты: %s\n", focusMark(0), m.hashIn.View()))
	b.WriteString(fmt.Sprintf("%s UID карты:  %s\n\n", focusMark(1), m.uidIn.View()))

	spamStatus := styleDim.Render("выкл")
	if m.spamming {
		spamStatus = styleSpamOn.Render(fmt.Sprintf("ВКЛ (%v)", m.interval))
	}
	b.WriteString(fmt.Sprintf("Спам хэшем: %s\n\n", spamStatus))

	b.WriteString(styleDim.Render("F2 — version   F3 — uid   F4/Enter — hash   F5 — спам   +/- — интервал   Tab — поле   Esc — выход") + "\n\n")

	b.WriteString(styleTitle.Render("Лог:") + "\n")
	visible := m.log
	if len(visible) > 15 {
		visible = visible[len(visible)-15:]
	}
	for _, line := range visible {
		b.WriteString(line + "\n")
	}

	return b.String()
}
