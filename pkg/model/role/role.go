// Copyright (c) 2024 Parseable, Inc
//
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package role

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/parseablehq/pb/pkg/model/button"
	"github.com/parseablehq/pb/pkg/model/selection"
	"github.com/parseablehq/pb/pkg/ui"
)

var (
	privileges    = []string{"admin", "editor", "ingestor", "reader", "writer"}
	navigationMap = []string{"role", "button"}
)

// Style for role selection widget
var (
	FocusPrimary  = ui.Adaptive(func(p ui.Palette) lipgloss.Color { return p.Accent })
	FocusSecondry = ui.Adaptive(func(p ui.Palette) lipgloss.Color { return p.Accent2 })

	StandardPrimary  = ui.Adaptive(func(p ui.Palette) lipgloss.Color { return p.Body })
	StandardSecondry = ui.Adaptive(func(p ui.Palette) lipgloss.Color { return p.Mute })

	focusedStyle           = lipgloss.NewStyle().Foreground(FocusPrimary)
	blurredStyle           = lipgloss.NewStyle().Foreground(StandardPrimary)
	selectionFocusStyle    = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).BorderForeground(StandardSecondry)
	selectionFocusStyleAlt = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).BorderForeground(FocusPrimary)
	selectionBlurStyle     = lipgloss.NewStyle().Height(3).AlignVertical(lipgloss.Center).MarginLeft(1).MarginRight(1)
)

type Model struct {
	focusIndex int
	navMap     *[]string
	Selection  selection.Model
	button     button.Model
	Success    bool
}

func (m *Model) FocusSelected() {
	m.Selection.Blur()
	m.Selection.FocusStyle = selectionFocusStyle
	m.button.Blur()

	switch (*m.navMap)[m.focusIndex] {
	case "role":
		m.Selection.Focus()
		m.Selection.FocusStyle = selectionFocusStyleAlt
	case "button":
		m.button.Focus()
	}
}

func New() Model {
	selection := selection.New(privileges)
	selection.BlurredStyle = selectionBlurStyle

	button := button.New("Submit")
	button.FocusStyle = focusedStyle
	button.BlurredStyle = blurredStyle

	m := Model{
		focusIndex: 0,
		navMap:     &navigationMap,
		Selection:  selection,
		button:     button,
		Success:    false,
	}

	m.FocusSelected()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case button.Pressed:
		m.Success = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			if m.button.Focused() && !m.button.Invalid {
				m.button, cmd = m.button.Update(msg)
				return m, cmd
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyDown, tea.KeyTab, tea.KeyEnter:
			m.focusIndex++
			if m.focusIndex >= len(*m.navMap) {
				m.focusIndex = 0
			}
			m.FocusSelected()
		case tea.KeyUp, tea.KeyShiftTab:
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = len(*m.navMap) - 1
			}
			m.FocusSelected()
		default:
			switch (*m.navMap)[m.focusIndex] {
			case "role":
				m.Selection, cmd = m.Selection.Update(msg)
			case "button":
				m.button, cmd = m.button.Update(msg)
			}
		}
	}
	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder

	for _, item := range *m.navMap {
		switch item {
		case "role":
			var buffer string
			if m.Selection.Focused() {
				buffer = lipgloss.JoinHorizontal(lipgloss.Center, "◀ ", m.Selection.View(), " ▶")
			} else {
				buffer = m.Selection.View()
			}
			fmt.Fprintln(&b, buffer)
		case "button":
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, m.button.View())
		}
	}

	return b.String()
}
