package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// TablePlus dark: charcoal surfaces, light-gray text, blue icons, green status only.
const (
	colorPGBlue   = "#336791"
	colorPGBlueHi = "#5B9BD5"
	colorWhite    = "#E0E0E0"
	colorDim      = "#6B7280"
	colorMuted    = "#8B949E"
	colorRed      = "#F85149"
	colorYellow   = "#D29922"
	colorGreen    = "#3FB950"
	colorBorder   = "238"
	colorBorderHi = "240"
	colorAccent   = "39" // TablePlus / es-tui accent blue
	colorSurface  = "234"
	colorSurface2 = "235"
	colorRowAlt   = "235"
	// Match es-tui / redis-tui: bright blue selection band (ANSI 39).
	colorSelectBg  = "39"
	colorSelectDim = "25" // darker blue when the pane is unfocused
	colorCellSel   = "33" // focused cell within a selected row
	colorHeaderBg  = "236"
	colorGridLine  = "238"
	colorSearchBg  = "236"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorWhite))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorMuted))
	normalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite))
	// Bold dark text on bright blue band — high-contrast cursor line.
	selectedStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color(colorSelectBg))
	selectedDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color(colorSelectDim))
	selectedPropStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorSelectBg))
	cellSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color(colorCellSel))
	keyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	descStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	metaDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent))
	pgBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPGBlue)).Bold(true)
	pgBlueHiStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPGBlueHi)).Bold(true)
	greenStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen))
	yellowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorYellow))
	blueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent))

	logoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPGBlue)).Bold(true)

	connCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorderHi)).
			Padding(0, 1).
			MarginBottom(0)

	connCardSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorGreen)).
				Padding(0, 1).
				MarginBottom(0)

	statsBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorderHi)).
			Padding(0, 1)

	badgeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colorSurface2)).
			Foreground(lipgloss.Color(colorMuted)).
			Padding(0, 1)

	badgeSSLStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("22")).
			Foreground(lipgloss.Color(colorGreen)).
			Padding(0, 1)

	badgeROStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("58")).
			Foreground(lipgloss.Color(colorYellow)).
			Padding(0, 1)

	statusPillStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorGreen)).
			Bold(true)

	sidebarTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim)).PaddingLeft(1)
	sectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			PaddingLeft(1).PaddingRight(1)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorMuted)).
				Background(lipgloss.Color(colorHeaderBg))

	zebraStyle = lipgloss.NewStyle().Background(lipgloss.Color(colorRowAlt))

	kindTableStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPGBlueHi))
	kindViewStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	kindSeqStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	kindFnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	kindTypeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	kindExtStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))

	gridSepStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGridLine))
	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color(colorSelectBg)).
			Padding(0, 1)
	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorDim)).
				Background(lipgloss.Color(colorSurface2)).
				Padding(0, 1)

	searchBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Background(lipgloss.Color(colorSearchBg))
	searchBarActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorWhite)).
				Background(lipgloss.Color(colorSearchBg)).
				Bold(true)

	nullCellStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim)).Italic(true)

	// Value / datatype palette (syntax-highlight feel)
	typeIntStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // sky
	typeFloatStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))  // cyan
	typeTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114")) // green
	typeBoolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("176")) // purple
	typeTimeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("180")) // gold
	typeJSONStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("215")) // orange
	typeUUIDStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("141")) // violet
	typeBinaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray
	typeGeoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("43"))  // teal
	typeOtherStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	pkBadgeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("22")).
			Foreground(lipgloss.Color("46")).
			Bold(true).
			Padding(0, 1)
	fkColStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("117")) // light blue for *_id
	nullYesStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	nullNoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	boolTrueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true)
	boolFalseStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	numCellStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	strCellStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	timeCellStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))
	jsonCellStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	uuidCellStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	schemaNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	rowCountStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sizeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("180"))

	// Constraint / index badges
	badgePKStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("22")).
			Foreground(lipgloss.Color("46")).
			Bold(true).
			Padding(0, 1)
	badgeFKStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("17")).
			Foreground(lipgloss.Color("117")).
			Bold(true).
			Padding(0, 1)
	badgeUniqueStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("58")).
				Foreground(lipgloss.Color(colorYellow)).
				Bold(true).
				Padding(0, 1)
	badgeCheckStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(colorSurface2)).
			Foreground(lipgloss.Color("215")).
			Padding(0, 1)

	// Activity backend states
	stateActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true)
	stateIdleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	stateXactStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorYellow)).Bold(true)
	stateWaitStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("215"))
	stateAbortStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).Bold(true)
	durHotStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))
	durWarmStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorYellow))
	durCoolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
)

func kindStyle(kind string) lipgloss.Style {
	switch strings.ToLower(kind) {
	case "table":
		return kindTableStyle
	case "view", "matview":
		return kindViewStyle
	case "sequence":
		return kindSeqStyle
	case "function":
		return kindFnStyle
	case "type":
		return kindTypeStyle
	case "extension":
		return kindExtStyle
	default:
		return dimStyle
	}
}

// pgTypeStyle colors a PostgreSQL type name in structure views.
func pgTypeStyle(dataType string) lipgloss.Style {
	t := strings.ToLower(dataType)
	switch {
	case strings.Contains(t, "int") || strings.Contains(t, "serial") ||
		t == "oid" || t == "xid" || t == "cid":
		return typeIntStyle
	case strings.Contains(t, "numeric") || strings.Contains(t, "decimal") ||
		strings.Contains(t, "float") || strings.Contains(t, "double") ||
		strings.Contains(t, "real") || strings.Contains(t, "money"):
		return typeFloatStyle
	case strings.Contains(t, "bool"):
		return typeBoolStyle
	case strings.Contains(t, "timestamp") || strings.Contains(t, "date") ||
		strings.Contains(t, "time") || t == "interval":
		return typeTimeStyle
	case strings.Contains(t, "json") || strings.Contains(t, "xml"):
		return typeJSONStyle
	case strings.Contains(t, "uuid"):
		return typeUUIDStyle
	case strings.Contains(t, "bytea") || strings.Contains(t, "bit"):
		return typeBinaryStyle
	case strings.Contains(t, "geo") || strings.Contains(t, "point") ||
		strings.Contains(t, "polygon") || strings.Contains(t, "path") ||
		strings.Contains(t, "circle") || strings.Contains(t, "box"):
		return typeGeoStyle
	case strings.Contains(t, "char") || strings.Contains(t, "text") ||
		strings.Contains(t, "name") || t == "citext":
		return typeTextStyle
	default:
		return typeOtherStyle
	}
}

// cellValueStyle picks a color from the cell's string contents (grids have no OIDs).
func cellValueStyle(raw string) lipgloss.Style {
	if raw == "" {
		return nullCellStyle
	}
	s := strings.TrimSpace(raw)
	low := strings.ToLower(s)
	switch low {
	case "true", "t", "yes", "on":
		return boolTrueStyle
	case "false", "f", "no", "off":
		return boolFalseStyle
	case "null", "∅":
		return nullCellStyle
	}
	// UUID
	if len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' {
		return uuidCellStyle
	}
	// JSON-ish
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return jsonCellStyle
	}
	// ISO date/time
	if looksLikeTime(s) {
		return timeCellStyle
	}
	// number
	if looksLikeNumber(s) {
		return numCellStyle
	}
	return strCellStyle
}

func looksLikeNumber(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '-' || s[0] == '+' {
		i = 1
		if len(s) == 1 {
			return false
		}
	}
	dots := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			dots++
			if dots > 1 {
				return false
			}
			continue
		}
		if c == 'e' || c == 'E' {
			// scientific
			rest := s[i+1:]
			if rest == "" {
				return false
			}
			return looksLikeNumber(rest) || (rest[0] >= '0' && rest[0] <= '9') ||
				((rest[0] == '+' || rest[0] == '-') && len(rest) > 1)
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func looksLikeTime(s string) bool {
	// 2024-01-01 or 2024-01-01T12:00:00 or with space
	if len(s) < 10 {
		return false
	}
	if s[4] == '-' && s[7] == '-' {
		return true
	}
	// time only 12:34:56
	if len(s) >= 8 && s[2] == ':' && s[5] == ':' {
		return true
	}
	return false
}

// constraintBadgeStyle colors PK/FK/UNIQUE/CHECK badges.
func constraintBadgeStyle(ctype string) lipgloss.Style {
	t := strings.ToUpper(strings.TrimSpace(ctype))
	switch {
	case strings.Contains(t, "PRIMARY") || t == "P" || t == "PK":
		return badgePKStyle
	case strings.Contains(t, "FOREIGN") || t == "F" || t == "FK":
		return badgeFKStyle
	case strings.Contains(t, "UNIQUE") || t == "U":
		return badgeUniqueStyle
	case strings.Contains(t, "CHECK") || t == "C":
		return badgeCheckStyle
	case strings.Contains(t, "EXCLUDE"):
		return badgeCheckStyle
	default:
		return badgeStyle
	}
}

// activityStateStyle colors pg_stat_activity.state values.
func activityStateStyle(state string) lipgloss.Style {
	s := strings.ToLower(strings.TrimSpace(state))
	switch {
	case s == "active":
		return stateActiveStyle
	case s == "idle":
		return stateIdleStyle
	case strings.Contains(s, "aborted"):
		return stateAbortStyle
	case strings.Contains(s, "transaction") || strings.Contains(s, "idle-xact"):
		return stateXactStyle
	case strings.Contains(s, "wait") || s == "fastpath":
		return stateWaitStyle
	default:
		return normalStyle
	}
}

// durationStyle heats long-running activity durations.
func durationStyle(d time.Duration) lipgloss.Style {
	switch {
	case d >= 30*time.Second:
		return durHotStyle
	case d >= 5*time.Second:
		return durWarmStyle
	default:
		return durCoolStyle
	}
}

func kindGlyph(kind string) string {
	switch strings.ToLower(kind) {
	case "table":
		return "▣"
	case "view", "matview":
		return "◎"
	case "sequence":
		return "№"
	case "function":
		return "ƒ"
	case "type":
		return "T"
	case "extension":
		return "◆"
	default:
		return "·"
	}
}

// toolGlyph is a single-cell ASCII tag — no emoji/wide glyphs (those break alignment).
func toolGlyph(nav NavSection) string {
	switch nav {
	case navQuery:
		return ">"
	case navActivity:
		return "*"
	case navERD:
		return "+"
	case navServer:
		return "@"
	case navDatabases:
		return "#"
	default:
		return "·"
	}
}

func toolStyle(nav NavSection) lipgloss.Style {
	switch nav {
	case navQuery:
		return accentStyle.Bold(true)
	case navActivity:
		return yellowStyle.Bold(true)
	case navERD:
		return kindViewStyle.Bold(true)
	case navServer:
		return kindTableStyle.Bold(true)
	case navDatabases:
		return typeFloatStyle.Bold(true)
	default:
		return dimStyle
	}
}

func panelStyle(focused bool, width, height int) lipgloss.Style {
	border := lipgloss.Color(colorBorder)
	if focused {
		border = lipgloss.Color(colorSelectBg)
	}
	s := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(border)
	if width > 2 {
		s = s.Width(width)
	}
	if height > 2 {
		s = s.Height(height)
	}
	return s
}
