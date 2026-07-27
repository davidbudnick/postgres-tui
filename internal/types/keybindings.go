package types

// KeyBindings holds customizable key bindings.
type KeyBindings struct {
	Up       string `json:"up"`
	Down     string `json:"down"`
	Left     string `json:"left"`
	Right    string `json:"right"`
	PageUp   string `json:"page_up"`
	PageDown string `json:"page_down"`
	Top      string `json:"top"`
	Bottom   string `json:"bottom"`
	Select   string `json:"select"`
	Back     string `json:"back"`
	Quit     string `json:"quit"`
	Help     string `json:"help"`
	Refresh  string `json:"refresh"`
	Delete   string `json:"delete"`
	Add      string `json:"add"`
	Edit     string `json:"edit"`
	Search   string `json:"search"`
	Filter   string `json:"filter"`
	Query    string `json:"query"`
	Activity string `json:"activity"`
	Server   string `json:"server"`
	Describe string `json:"describe"`
	Export   string `json:"export"`
	Logs     string `json:"logs"`
	Palette  string `json:"palette"`
}

// DefaultKeyBindings returns the default key bindings.
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		Up:       "k",
		Down:     "j",
		Left:     "h",
		Right:    "l",
		PageUp:   "ctrl+u",
		PageDown: "ctrl+d",
		Top:      "g",
		Bottom:   "G",
		Select:   "enter",
		Back:     "esc",
		Quit:     "q",
		Help:     "?",
		Refresh:  "r",
		Delete:   "d",
		Add:      "a",
		Edit:     "e",
		Search:   "/",
		Filter:   "f",
		Query:    ";", // open SQL editor (also ":" as alias in UI)
		Activity: "A",
		Server:   "i",
		Describe: "D",
		Export:   "x",
		Logs:     "L",
		Palette:  "ctrl+p",
	}
}
