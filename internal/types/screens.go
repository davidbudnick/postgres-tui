package types

// Screen represents the current view in the application.
type Screen int

const (
	ScreenConnections Screen = iota
	ScreenAddConnection
	ScreenEditConnection
	ScreenDatabases
	ScreenBrowser
	ScreenTableData
	ScreenTableDetail
	ScreenQuery
	ScreenActivity
	ScreenERD
	ScreenServerInfo
	ScreenHelp
	ScreenConfirmDelete
	ScreenTestConnection
	ScreenLogs
	ScreenFavorites
	ScreenExport
	ScreenCommandPalette
)

// String returns a human-readable name for the screen.
func (s Screen) String() string {
	names := map[Screen]string{
		ScreenConnections:    "Connections",
		ScreenAddConnection:  "Add Connection",
		ScreenEditConnection: "Edit Connection",
		ScreenDatabases:      "Databases",
		ScreenBrowser:        "Browser",
		ScreenTableData:      "Table Data",
		ScreenTableDetail:    "Table Detail",
		ScreenQuery:          "Query",
		ScreenActivity:       "Activity",
		ScreenERD:            "ERD",
		ScreenServerInfo:     "Server Info",
		ScreenHelp:           "Help",
		ScreenConfirmDelete:  "Confirm Delete",
		ScreenTestConnection: "Test Connection",
		ScreenLogs:           "Logs",
		ScreenFavorites:      "Favorites",
		ScreenExport:         "Export",
		ScreenCommandPalette: "Command Palette",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}
