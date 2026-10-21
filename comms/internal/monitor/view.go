package monitor

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"factory/comms/internal/comms"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	ink    = lipgloss.Color("#DBE4F3")
	muted  = lipgloss.Color("#8091AD")
	cyan   = lipgloss.Color("#69E0D4")
	blue   = lipgloss.Color("#85ACFF")
	amber  = lipgloss.Color("#FFD180")
	red    = lipgloss.Color("#FF8195")
	line   = lipgloss.Color("#34435D")
	dim    = lipgloss.NewStyle().Foreground(muted)
	strong = lipgloss.NewStyle().Foreground(ink).Bold(true)
	accent = lipgloss.NewStyle().Foreground(cyan).Bold(true)
)

// Remote content is untrusted: never interpret terminal escapes or control codes.
func clean(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(value))
}
func shorten(value string, width int) string {
	return ansi.Truncate(clean(value), max(1, width), "…")
}
func colorState(state string) lipgloss.Color {
	switch state {
	case "working", "exec":
		return cyan
	case "complete", "completed", "submitted", "read", "idle":
		return blue
	case "needs_input", "blocked", "offered", "sent":
		return amber
	case "failed", "failure", "cancelled":
		return red
	}
	return muted
}
func badge(state string) string {
	return lipgloss.NewStyle().Foreground(colorState(state)).Bold(true).Render(strings.ToUpper(clean(state)))
}
func box(content string, w, h int, focus bool) string {
	border := line
	if focus {
		border = cyan
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Width(w - 2).Height(h - 2).MaxHeight(h).Render(content)
}
func section(title string) string { return "\n" + accent.Render(title) + "\n" }
func (m Model) View() string {
	if m.width < 65 || m.height < 18 {
		return "Factory monitor\nResize to at least 65 columns × 18 rows.\nq quit"
	}
	width := m.width
	header := accent.Render("  ◈ FACTORY") + dim.Render("  /  OPERATIONS")
	connection := "CONNECTING"
	if !m.updated.IsZero() {
		connection = "CONNECTED"
	}
	if m.err != nil {
		connection = "DISCONNECTED · STALE"
	} else if m.paused {
		connection = "PAUSED"
	} else if m.busy {
		connection = "REFRESHING"
	}
	connectionColor := cyan
	if m.err != nil {
		connectionColor = red
	} else if m.paused {
		connectionColor = amber
	}
	header = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(width-29).Render(header), lipgloss.NewStyle().Foreground(connectionColor).Render(connection))
	project := dim.Render("  PROJECT  ") + shorten(m.project, width-13)
	active, attention, unread := 0, 0, 0
	for _, item := range m.data.tasks {
		if item.Task.Status == "working" {
			active++
		}
		attention += item.OpenAttention
		unread += item.UnreadMessages
	}
	metrics := fmt.Sprintf("  %s tasks  %s working  %s attention  %s unread", accent.Render(fmt.Sprint(len(m.data.tasks))), badge(fmt.Sprint(active)), lipgloss.NewStyle().Foreground(amber).Render(fmt.Sprint(attention)), strong.Render(fmt.Sprint(unread))) + dim.Render("  · current page")
	bodyHeight := m.height - 9
	sidebarWidth := min(34, width/3)
	list := m.taskList(sidebarWidth-2, bodyHeight-2)
	detailsWidth := width - sidebarWidth
	content := m.detail(detailsWidth - 4)
	lines := strings.Split(content, "\n")
	offset := min(m.scroll, max(0, len(lines)-(bodyHeight-2)))
	end := min(len(lines), offset+bodyHeight-2)
	clipped := make([]string, 0, end-offset)
	for _, row := range lines[offset:end] {
		clipped = append(clipped, ansi.Truncate(row, detailsWidth-2, "…"))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, box(list, sidebarWidth, bodyHeight, !m.detailFocus), box(strings.Join(clipped, "\n"), detailsWidth, bodyHeight, m.detailFocus))
	notice := "  READ ONLY · refresh 3s · contact ≠ progress"
	if !m.updated.IsZero() {
		notice += " · last success " + m.updated.Format("15:04:05")
	}
	if m.err != nil {
		notice = "  " + clean(m.err.Error()) + " · retained data is stale"
	}
	if m.before != 0 {
		notice += " · older page"
	}
	help := "  q quit  ↑↓ select/scroll  tab focus  1 overview  2 comms  3 events"
	navigation := "  p project  n older  b latest  r refresh  space pause  pg↑↓ scroll"
	return strings.Join([]string{header, project, "", metrics, body, dim.Render(shorten(notice, width)), dim.Render(shorten(help, width)), dim.Render(shorten(navigation, width))}, "\n")
}
func (m Model) taskList(width, height int) string {
	rows := []string{accent.Render(" TASKS") + dim.Render("  /  newest first"), ""}
	if len(m.data.tasks) == 0 && m.updated.IsZero() {
		return strings.Join(append(rows, dim.Render(" Awaiting first snapshot.")), "\n")
	}
	if len(m.data.tasks) == 0 {
		return strings.Join(append(rows, dim.Render(" No recorded tasks."), dim.Render(" p switch project · r retry")), "\n")
	}
	visible := max(1, (height-2)/3)
	start := max(0, m.selectedIndex()-visible+1)
	for _, item := range m.data.tasks[start:min(len(m.data.tasks), start+visible)] {
		prefix := "  "
		if item.Task.ID == m.selected {
			prefix = "▸ "
		}
		title := shorten(prefix+item.Task.Name, width)
		if item.Task.ID == m.selected {
			title = accent.Render(title)
		} else {
			title = strong.Render(title)
		}
		state := "  " + badge(item.Task.Status)
		if item.OpenAttention > 0 {
			state += lipgloss.NewStyle().Foreground(amber).Render(fmt.Sprintf("  ! %d", item.OpenAttention))
		}
		rows = append(rows, title, state, dim.Render("  "+shorten(item.Factory+" / "+item.ActiveStage, width-2)))
	}
	return strings.Join(rows, "\n")
}
func (m Model) detail(width int) string {
	tabs := []string{"1 OVERVIEW", "2 COMMS", "3 EVENTS"}
	for i := range tabs {
		if i == m.tab {
			tabs[i] = accent.Render(tabs[i])
		} else {
			tabs[i] = dim.Render(tabs[i])
		}
	}
	heading := " " + strings.Join(tabs, "   ") + "\n"
	if m.data.detail == nil {
		return heading + "\n Waiting for task data…"
	}
	d := m.data.detail
	header := strong.Render(shorten(d.Report.Task.Name, width)) + "\n" + badge(d.Report.Task.Status)
	if d.Factory != nil {
		header += "  " + dim.Render(clean(d.Factory.Name))
	}
	if d.Report.Task.Summary != "" {
		header += "\n" + shorten(d.Report.Task.Summary, width)
	}
	if d.Partial || d.Inbox.Partial {
		header += "\n" + badge("blocked") + " Bounded view; additional records exist."
	}
	switch m.tab {
	case 1:
		return heading + "\n" + header + "\n" + m.messageFlow(width) + section("MESSAGE STREAM  /  newest first") + m.messages(width)
	case 2:
		rows := []string{heading, header, section("EVENTS  /  recent recorded activity")}
		events := append([]comms.Event(nil), d.Events...)
		sort.SliceStable(events, func(i, j int) bool { return events[i].Sequence > events[j].Sequence })
		for _, event := range events {
			rows = append(rows, dim.Render(shorten(event.CreatedAt, 19))+"  "+badge(event.Kind)+"  "+strong.Render(clean(event.Actor)), shorten(event.Message, width), "")
		}
		if len(events) == 0 {
			rows = append(rows, dim.Render("No events recorded."))
		}
		rows = append(rows, dim.Render(fmt.Sprintf("Showing %d of %d recorded events.", len(events), d.Report.EventCount)))
		return strings.Join(rows, "\n")
	}
	rows := []string{heading, header, section(fmt.Sprintf("ATTENTION  /  %d open", d.Inbox.OpenCount))}
	if len(d.Inbox.Items) == 0 {
		rows = append(rows, dim.Render("No open requests."))
	}
	for _, item := range d.Inbox.Items {
		rows = append(rows, badge(item.Kind)+"  "+shorten(item.Title, width-18), "  → "+clean(item.Recipient)+"  "+shorten(item.Detail, width-12))
	}
	for _, failure := range d.Inbox.DeliveryFailures {
		rows = append(rows, badge("failure")+"  delivery "+clean(failure.State)+" · "+shorten(failure.Detail, width-25))
	}
	rows = append(rows, section("ACTIVE WORK  /  recorded assignments"))
	work := 0
	for _, a := range d.Report.Assignments {
		switch a.State {
		case "offered", "accepted", "working", "blocked", "needs_input":
		default:
			continue
		}
		work++
		rows = append(rows, badge(a.State)+"  "+shorten(a.Stage, width-18)+fmt.Sprintf(" · attempt %d", a.Attempt), "  "+shorten(a.Title, width-2))
	}
	if work == 0 {
		rows = append(rows, dim.Render("No active assignment. Stage is not inferred."))
	}
	rows = append(rows, section("AGENTS  /  contact and assignment state"))
	sessions := map[string]comms.SessionSnapshot{}
	for _, s := range d.Report.Sessions {
		sessions[s.ParticipantID] = s
	}
	roles := map[string]bool{}
	for _, p := range d.Report.Participants {
		roles[p.Role] = true
		contact := "unknown"
		if session, ok := sessions[p.ID]; ok {
			expires, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
			if err == nil && expires.After(time.Now()) {
				contact = session.ContactState
			}
		}
		if p.RetiredAt != nil {
			contact = "retired"
		}
		rows = append(rows, "● "+strong.Render(shorten(p.RoutingIdentity, width/2))+"  "+badge(contact))
	}
	for _, member := range d.Members {
		if !roles[member.Name] {
			rows = append(rows, dim.Render("○ "+clean(member.Name)+"  not started"))
		}
	}
	if len(d.Report.Participants) == 0 && len(d.Members) == 0 {
		rows = append(rows, dim.Render("No agents registered."))
	}
	rows = append(rows, m.messageFlow(width), dim.Render("2 comms: message contents and sent → read → exec"))
	return strings.Join(rows, "\n")
}
func (m Model) messageFlow(width int) string {
	d := m.data.detail
	type edge struct {
		from, to         string
		sent, read, exec int
	}
	edges := map[[2]string]*edge{}
	for _, message := range d.Messages {
		key := [2]string{message.Sender, message.Recipient}
		if edges[key] == nil {
			edges[key] = &edge{from: key[0], to: key[1]}
		}
		e := edges[key]
		switch message.Status {
		case "exec":
			e.exec++
		case "read":
			e.read++
		default:
			e.sent++
		}
	}
	ordered := make([]*edge, 0, len(edges))
	for _, e := range edges {
		ordered = append(ordered, e)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].from == ordered[j].from {
			return ordered[i].to < ordered[j].to
		}
		return ordered[i].from < ordered[j].from
	})
	rows := []string{section("COMMUNICATION FLOW  /  observed routes")}
	labelWidth := max(8, (width-9)/2)
	for _, e := range ordered {
		left := lipgloss.NewStyle().Width(labelWidth).Foreground(blue).Render(shorten(e.from, labelWidth))
		right := lipgloss.NewStyle().Foreground(cyan).Render(shorten(e.to, labelWidth))
		rows = append(rows, left+dim.Render(" ───▶ ")+right, "  "+badge("sent")+fmt.Sprintf(" %d   ", e.sent)+badge("read")+fmt.Sprintf(" %d   ", e.read)+badge("exec")+fmt.Sprintf(" %d", e.exec))
	}
	if len(ordered) == 0 {
		rows = append(rows, dim.Render("No messages recorded; no connections inferred."))
	}
	rows = append(rows, dim.Render(fmt.Sprintf("Routes from %d/%d messages · counts are current states", len(d.Messages), d.Report.MessageCount)))
	return strings.Join(rows, "\n")
}
func (m Model) messages(width int) string {
	messages := append([]comms.Message(nil), m.data.detail.Messages...)
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreatedAt > messages[j].CreatedAt })
	rows := []string{}
	for _, message := range messages {
		rows = append(rows, strong.Render(shorten(message.Sender+" → "+message.Recipient, width)), dim.Render(clean(message.Kind)+" · "+clean(message.CreatedAt))+"  "+badge(message.Status))
		rows = append(rows, lipgloss.NewStyle().Width(max(1, width-2)).Render(clean(message.Text)), "")
	}
	if len(rows) == 0 {
		rows = append(rows, dim.Render("No messages yet."))
	}
	return strings.Join(rows, "\n")
}
