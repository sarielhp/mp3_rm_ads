package uicommon

type ColorID int

const (
	ColorDefault ColorID = iota - 1
	ColorBg
	ColorFg
	ColorDate
	ColorSender
	ColorSubject
	ColorHighlightBg
	ColorHighlightFg
	ColorDim
	ColorBorder
	ColorTopBg
	ColorMsgEndBg
	ColorQuote2
	ColorQuote3
	ColorSpam
	ColorPoli
	ColorICS
)

type Theme struct {
	Colors [16]string
}

func (t *Theme) Get(id ColorID) string {
	if int(id) < 0 || int(id) >= len(t.Colors) {
		return t.Colors[ColorFg]
	}
	return t.Colors[id]
}

type ThemeManager struct {
	current *Theme
	names   map[string]*Theme
}

func NewDefaultTheme() *Theme {
	return &Theme{
		Colors: [16]string{
			"#2b2b2b",
			"#f8f8f2",
			"#888888",
			"#66ccff",
			"#ffffff",
			"#cc2222",
			"#ffdd44",
			"#777777",
			"#4a9e6e",
			"4",
			"92",
			"#336699",
			"#1a3a55",
			"#ff6666",
			"#ff7f7f",
			"#0a2a0a",
		},
	}
}

func NewThemeManager() *ThemeManager {
	return &ThemeManager{
		current: NewDefaultTheme(),
		names:   make(map[string]*Theme),
	}
}

func (tm *ThemeManager) Theme() *Theme {
	if tm.current == nil {
		tm.current = NewDefaultTheme()
	}
	return tm.current
}

func (tm *ThemeManager) Switch(name string) {
	if t, ok := tm.names[name]; ok {
		tm.current = t
	}
}
