package ui

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

type palette struct {
	Background lipgloss.Color
	Surface    lipgloss.Color
	Panel      lipgloss.Color
	Text       lipgloss.Color
	Muted      lipgloss.Color
	Accent     lipgloss.Color
	AccentAlt  lipgloss.Color
	Border     lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	BarFill    lipgloss.Color
	BarEmpty   lipgloss.Color
}

var palettes = map[string]palette{
	"catppuccin": {
		Background: lipgloss.Color("#1e1e2e"),
		Surface:    lipgloss.Color("#313244"),
		Panel:      lipgloss.Color("#45475a"),
		Text:       lipgloss.Color("#cdd6f4"),
		Muted:      lipgloss.Color("#a6adc8"),
		Accent:     lipgloss.Color("#cba6f7"),
		AccentAlt:  lipgloss.Color("#f38ba8"),
		Border:     lipgloss.Color("#585b70"),
		Success:    lipgloss.Color("#94e2d5"),
		Warning:    lipgloss.Color("#f9e2af"),
		BarFill:    lipgloss.Color("#94e2d5"),
		BarEmpty:   lipgloss.Color("#313244"),
	},
	"dracula": {
		Background: lipgloss.Color("#282a36"),
		Surface:    lipgloss.Color("#343746"),
		Panel:      lipgloss.Color("#3c4053"),
		Text:       lipgloss.Color("#f8f8f2"),
		Muted:      lipgloss.Color("#6272a4"),
		Accent:     lipgloss.Color("#ff79c6"),
		AccentAlt:  lipgloss.Color("#bd93f9"),
		Border:     lipgloss.Color("#44475a"),
		Success:    lipgloss.Color("#50fa7b"),
		Warning:    lipgloss.Color("#f1fa8c"),
		BarFill:    lipgloss.Color("#50fa7b"),
		BarEmpty:   lipgloss.Color("#343746"),
	},
	"gruvbox": {
		Background: lipgloss.Color("#282828"),
		Surface:    lipgloss.Color("#3c3836"),
		Panel:      lipgloss.Color("#504945"),
		Text:       lipgloss.Color("#ebdbb2"),
		Muted:      lipgloss.Color("#a89984"),
		Accent:     lipgloss.Color("#fabd2f"),
		AccentAlt:  lipgloss.Color("#d3869b"),
		Border:     lipgloss.Color("#665c54"),
		Success:    lipgloss.Color("#b8bb26"),
		Warning:    lipgloss.Color("#fe8019"),
		BarFill:    lipgloss.Color("#b8bb26"),
		BarEmpty:   lipgloss.Color("#3c3836"),
	},
	"solarized_dark": {
		Background: lipgloss.Color("#002b36"),
		Surface:    lipgloss.Color("#073642"),
		Panel:      lipgloss.Color("#0a3a45"),
		Text:       lipgloss.Color("#fdf6e3"),
		Muted:      lipgloss.Color("#93a1a1"),
		Accent:     lipgloss.Color("#b58900"),
		AccentAlt:  lipgloss.Color("#268bd2"),
		Border:     lipgloss.Color("#586e75"),
		Success:    lipgloss.Color("#859900"),
		Warning:    lipgloss.Color("#cb4b16"),
		BarFill:    lipgloss.Color("#859900"),
		BarEmpty:   lipgloss.Color("#073642"),
	},
	"nord": {
		Background: lipgloss.Color("#2e3440"),
		Surface:    lipgloss.Color("#3b4252"),
		Panel:      lipgloss.Color("#434c5e"),
		Text:       lipgloss.Color("#e5e9f0"),
		Muted:      lipgloss.Color("#81a1c1"),
		Accent:     lipgloss.Color("#88c0d0"),
		AccentAlt:  lipgloss.Color("#bf616a"),
		Border:     lipgloss.Color("#4c566a"),
		Success:    lipgloss.Color("#a3be8c"),
		Warning:    lipgloss.Color("#ebcb8b"),
		BarFill:    lipgloss.Color("#a3be8c"),
		BarEmpty:   lipgloss.Color("#3b4252"),
	},
	"tokyonight": {
		Background: lipgloss.Color("#1a1b26"),
		Surface:    lipgloss.Color("#24283b"),
		Panel:      lipgloss.Color("#292e42"),
		Text:       lipgloss.Color("#c0caf5"),
		Muted:      lipgloss.Color("#545c7e"),
		Accent:     lipgloss.Color("#7aa2f7"),
		AccentAlt:  lipgloss.Color("#bb9af7"),
		Border:     lipgloss.Color("#414868"),
		Success:    lipgloss.Color("#9ece6a"),
		Warning:    lipgloss.Color("#e0af68"),
		BarFill:    lipgloss.Color("#9ece6a"),
		BarEmpty:   lipgloss.Color("#24283b"),
	},
	"everforest": {
		Background: lipgloss.Color("#2b3339"),
		Surface:    lipgloss.Color("#323c41"),
		Panel:      lipgloss.Color("#3a454a"),
		Text:       lipgloss.Color("#d3c6aa"),
		Muted:      lipgloss.Color("#859289"),
		Accent:     lipgloss.Color("#a7c080"),
		AccentAlt:  lipgloss.Color("#e69875"),
		Border:     lipgloss.Color("#475258"),
		Success:    lipgloss.Color("#9da9a0"),
		Warning:    lipgloss.Color("#dbbc7f"),
		BarFill:    lipgloss.Color("#a7c080"),
		BarEmpty:   lipgloss.Color("#323c41"),
	},
}

func paletteFor(name string) palette {
	if p, ok := palettes[name]; ok {
		return p
	}
	return palettes["catppuccin"]
}

func themeNames() []string {
	names := make([]string, 0, len(palettes))
	for k := range palettes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func nextThemeName(current string, step int) string {
	names := themeNames()
	if len(names) == 0 {
		return current
	}
	idx := 0
	for i, name := range names {
		if name == current {
			idx = i
			break
		}
	}
	idx = (idx + step) % len(names)
	if idx < 0 {
		idx += len(names)
	}
	return names[idx]
}
