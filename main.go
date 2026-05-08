package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	colorGreen  = lipgloss.Color("#00FF00")
	colorCyan   = lipgloss.Color("#00FFFF")
	colorPurple = lipgloss.Color("#8A2BE2")
	colorPink   = lipgloss.Color("#FF00FF")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(colorGreen).
			Padding(0, 2).
			MarginBottom(1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGreen).
			Padding(1, 2).
			Width(60)

	labelStyle = lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Width(15)
	valueStyle = lipgloss.NewStyle().Foreground(colorPink)

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).MarginTop(1)
)

type metricsMsg struct {
	cpuPercent   float64
	memTotal     uint64
	memUsed      uint64
	diskTotal    uint64
	diskUsed     uint64
	hostUptime   uint64
	hostPlatform string
}

type model struct {
	metrics metricsMsg

	cpuProg  progress.Model
	memProg  progress.Model
	diskProg progress.Model

	quitting bool
}

func initialModel() model {
	progOpts := []progress.Option{
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	}

	cpuP := progress.New(progOpts...)
	memP := progress.New(progOpts...)
	diskP := progress.New(progOpts...)

	return model{
		cpuProg:  cpuP,
		memProg:  memP,
		diskProg: diskP,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		fetchMetricsCmd(),
		tea.SetWindowTitle("GOPHER OS // SYSTEM MONITOR"),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case metricsMsg:
		m.metrics = msg
		return m, tea.Tick(time.Second*1, func(time.Time) tea.Msg {
			return fetchMetricsMsg()
		})
	case tea.WindowSizeMsg:
		w := msg.Width - 20
		if w < 10 {
			w = 10
		}
		if w > 40 {
			w = 40
		}
		m.cpuProg.Width = w
		m.memProg.Width = w
		m.diskProg.Width = w
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "\n  Система мониторинга отключена. До свидания!\n\n"
	}

	// CPU
	cpuView := fmt.Sprintf("%s %s\n%s", 
		labelStyle.Render("CPU LOAD:"), 
		valueStyle.Render(fmt.Sprintf("%5.1f%%", m.metrics.cpuPercent)),
		m.cpuProg.ViewAs(m.metrics.cpuPercent/100.0),
	)

	// MEMORY
	memUsedGB := float64(m.metrics.memUsed) / 1024 / 1024 / 1024
	memTotalGB := float64(m.metrics.memTotal) / 1024 / 1024 / 1024
	memRatio := 0.0
	if m.metrics.memTotal > 0 {
		memRatio = float64(m.metrics.memUsed) / float64(m.metrics.memTotal)
	}
	memView := fmt.Sprintf("%s %s\n%s",
		labelStyle.Render("RAM LOAD:"),
		valueStyle.Render(fmt.Sprintf("%5.1f GB / %5.1f GB", memUsedGB, memTotalGB)),
		m.memProg.ViewAs(memRatio),
	)

	// DISK
	diskUsedGB := float64(m.metrics.diskUsed) / 1024 / 1024 / 1024
	diskTotalGB := float64(m.metrics.diskTotal) / 1024 / 1024 / 1024
	diskRatio := 0.0
	if m.metrics.diskTotal > 0 {
		diskRatio = float64(m.metrics.diskUsed) / float64(m.metrics.diskTotal)
	}
	diskView := fmt.Sprintf("%s %s\n%s",
		labelStyle.Render("SYSTEM DISK:"),
		valueStyle.Render(fmt.Sprintf("%5.1f GB / %5.1f GB", diskUsedGB, diskTotalGB)),
		m.diskProg.ViewAs(diskRatio),
	)

	// HOST INFO
	uptimeDur := time.Duration(m.metrics.hostUptime) * time.Second
	uptimeStr := fmt.Sprintf("%dh %dm %ds", int(uptimeDur.Hours()), int(uptimeDur.Minutes())%60, int(uptimeDur.Seconds())%60)

	hostView := lipgloss.JoinVertical(lipgloss.Left,
		fmt.Sprintf("%s %s", labelStyle.Render("OS:"), valueStyle.Render(m.metrics.hostPlatform)),
		fmt.Sprintf("%s %s", labelStyle.Render("UPTIME:"), valueStyle.Render(uptimeStr)),
	)

	title := titleStyle.Render(" NETRUNNER // NODE STATUS ")

	content := lipgloss.JoinVertical(lipgloss.Left,
		cpuView,
		"",
		memView,
		"",
		diskView,
		"",
		hostView,
	)

	panel := panelStyle.Render(content)

	help := helpStyle.Render("Press 'q' or 'ctrl+c' to exit.")

	return lipgloss.JoinVertical(lipgloss.Center, title, panel, help) + "\n"
}

func fetchMetricsCmd() tea.Cmd {
	return func() tea.Msg {
		return fetchMetricsMsg()
	}
}

func fetchMetricsMsg() tea.Msg {
	c, _ := cpu.Percent(0, false)
	var cp float64
	if len(c) > 0 {
		cp = c[0]
	}

	v, _ := mem.VirtualMemory()

	var diskPath = "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:"
	}
	d, _ := disk.Usage(diskPath)

	i, _ := host.Info()

	var mUsed, mTotal, dUsed, dTotal, hUp uint64
	var hPlat string
	if v != nil {
		mUsed = v.Used
		mTotal = v.Total
	}
	if d != nil {
		dUsed = d.Used
		dTotal = d.Total
	}
	if i != nil {
		hUp = i.Uptime
		hPlat = i.Platform + " " + i.PlatformVersion
	}

	return metricsMsg{
		cpuPercent:   cp,
		memUsed:      mUsed,
		memTotal:     mTotal,
		diskUsed:     dUsed,
		diskTotal:    dTotal,
		hostUptime:   hUp,
		hostPlatform: hPlat,
	}
}

func main() {
	cpu.Percent(0, false)

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
