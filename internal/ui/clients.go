package ui

import (
	"charm.land/bubbles/v2/key"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tunapm/internal/money"
	"tunapm/internal/store"
)

type Clients struct {
	st            store.Repo
	width, height int
	clients       []store.Client
	projects      []store.Project
	totals        map[int64]store.ProjectTotals
	selectedID    int64
}

func newClients(st store.Repo) *Clients {
	return &Clients{st: st, width: 100, height: 20}
}

func (c *Clients) layout(width, height int) {
	c.width, c.height = width, height
	c.reload()
}

func (c *Clients) reload() {
	clients, err := c.st.ListClients()
	if err != nil {
		return
	}
	c.clients = clients
	c.projects, _ = c.st.ListProjects()
	c.totals, _ = c.st.AllProjectTotals()

	if len(clients) == 0 {
		c.selectedID = 0
		return
	}
	found := false
	for _, client := range clients {
		if client.ID == c.selectedID {
			found = true
			break
		}
	}
	if !found {
		c.selectedID = clients[0].ID
	}
}

func (c *Clients) selectedClient() *store.Client {
	for i := range c.clients {
		if c.clients[i].ID == c.selectedID {
			return &c.clients[i]
		}
	}
	return nil
}

func (c *Clients) selectByID(id int64) bool {
	for _, client := range c.clients {
		if client.ID == id {
			c.selectedID = id
			return true
		}
	}
	return false
}

func (c *Clients) selectedIndex() int {
	for i := range c.clients {
		if c.clients[i].ID == c.selectedID {
			return i
		}
	}
	return 0
}

func (c *Clients) clientProjects(id int64) []store.Project {
	var out []store.Project
	for _, p := range c.projects {
		if p.ClientID == id {
			out = append(out, p)
		}
	}
	return out
}

func (c *Clients) listWidth() int {
	return max(min(c.width/2, 64), 30)
}

func (c *Clients) view() string {
	return masterDetail(c.width, c.height, c.listWidth(), c.listView, c.detailView)
}

func (c *Clients) listView(width, height int) string {
	var lines []string
	const countW = 9
	nameW := max(width-2-countW-1, 8)
	lines = append(lines, headStyle.Render(padRight("  CLIENTE", 2+nameW)+" "+padLeft("PROYECTOS", countW)))
	if len(c.clients) == 0 {
		lines = append(lines, "", mutedStyle.Render("  Sin clientes. n crea uno."))
	}
	visible := max(height-1, 1)
	start := c.selectedIndex() - visible/2
	if start > len(c.clients)-visible {
		start = len(c.clients) - visible
	}
	if start < 0 {
		start = 0
	}
	for i := start; i < len(c.clients) && i < start+visible; i++ {
		client := c.clients[i]
		selected := client.ID == c.selectedID
		marker := cursorMark(selected)
		name := titleStyle.Render(truncate(client.Name, nameW))
		if room := nameW - lipglossWidth(truncate(client.Name, nameW)) - 3; room > 3 && client.Company != "" {
			name += faintStyle.Render(" · " + truncate(client.Company, room))
		}
		count := len(c.clientProjects(client.ID))
		countText := mutedStyle.Render("-")
		if count > 0 {
			countText = fmt.Sprint(count)
		}
		line := marker + padRight(name, nameW) + " " + padLeft(countText, countW)
		if selected {
			line = selectedStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:height], "\n")
}

func (c *Clients) detailView(width, height int) string {
	client := c.selectedClient()
	if client == nil {
		return mutedStyle.Render("Selecciona un cliente")
	}
	lines := []string{titleStyle.Render(client.Name)}
	if client.Company != "" {
		lines = append(lines, mutedStyle.Render(client.Company))
	}
	lines = append(lines, "")
	if client.Email != "" {
		lines = append(lines, field("Email", client.Email))
	}
	if client.Phone != "" {
		lines = append(lines, field("Teléfono", client.Phone))
	}
	if client.Email == "" && client.Phone == "" {
		lines = append(lines, mutedStyle.Render("Sin datos de contacto. e edita."))
	}

	budget := map[string]int64{}
	paid := map[string]int64{}
	pending := map[string]int64{}
	projects := c.clientProjects(client.ID)
	lines = append(lines, "", sectionStyle.Render("Proyectos"))
	if len(projects) == 0 {
		lines = append(lines, mutedStyle.Render("Sin proyectos. ctrl+k y p crea uno."))
	}
	for _, p := range projects {
		nameW := max(width-11-13, 8)
		lines = append(lines, padRight(truncate(p.Name, nameW), nameW)+" "+padRight(projectStatusText(p.Status), 10)+
			" "+padLeft(money.Format(p.BudgetCents, p.Currency), 12))
		budget[p.Currency] += p.BudgetCents
		totals := c.totals[p.ID]
		paid[p.Currency] += totals.PaidCents
		pending[p.Currency] += totals.ScheduledCents - totals.PaidCents
	}

	lines = append(lines, "", sectionStyle.Render("Totales"))
	for _, currency := range []string{store.CurrencyGTQ, store.CurrencyUSD} {
		if budget[currency] == 0 && paid[currency] == 0 && pending[currency] == 0 {
			continue
		}
		lines = append(lines,
			field("Presupuestado", money.Format(budget[currency], currency)),
			field("Cobrado", successStyle.Render(money.Format(paid[currency], currency))),
			field("Por cobrar", titleStyle.Render(money.Format(budget[currency]-paid[currency], currency))),
		)
	}

	if client.Notes != "" {
		lines = append(lines, "", sectionStyle.Render("Notas"), wrapLine(client.Notes, width))
	}
	return clipLines(lines, width, height)
}

func (c *Clients) info() string {
	info := fmt.Sprintf("Clientes · %s", plural(len(c.clients), "cliente", "clientes"))
	return info
}

func (c *Clients) handleKey(a *App, msg tea.KeyPressMsg) {
	c.shortHelp() // actualiza qué atajos aplican
	client := c.selectedClient()
	switch {
	case key.Matches(msg, clientKeys.Down):
		if len(c.clients) > 0 {
			idx := min(c.selectedIndex()+1, len(c.clients)-1)
			c.selectByID(c.clients[idx].ID)
		}
	case key.Matches(msg, clientKeys.Up):
		if len(c.clients) > 0 {
			idx := max(c.selectedIndex()-1, 0)
			c.selectByID(c.clients[idx].ID)
		}
	case key.Matches(msg, clientKeys.New):
		a.openClientForm(store.Client{})
	case key.Matches(msg, clientKeys.Edit):
		if client != nil {
			a.openClientForm(*client)
		}
	case key.Matches(msg, clientKeys.Delete):
		if client == nil {
			return
		}
		id, name := client.ID, client.Name
		a.confirmAction(fmt.Sprintf("¿Borrar al cliente %q?", name), func(a *App) {
			err := a.st.DeleteClient(id)
			if errors.Is(err, store.ErrClientHasProjects) {
				a.errf("No se puede: el cliente tiene proyectos")
				return
			}
			if err != nil {
				a.errf("No se pudo borrar: %v", err)
				return
			}
			a.toastf("Cliente borrado")
			a.reloadAll()
		})
	case key.Matches(msg, clientKeys.Tasks):
		if client == nil {
			return
		}
		id := client.ID
		a.tasks.mode, a.tasks.search = store.FilterOpen, ""
		a.tasks.projectFilter, a.tasks.clientFilter = nil, &id
		a.tasks.filterLabel = "cliente: " + client.Name
		a.tasks.reload()
		a.setTab(tabTasks)
	}
}
