package data

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	DefaultWindowWidth  = 720
	DefaultWindowHeight = 500
)

// DefaultColumnWidths matches the [24, 64, 140, 100, 140, 100, 100]
// defaults written by PortProxyGUI's 1.4.0 migration.
var DefaultColumnWidths = []int{24, 64, 140, 100, 140, 100, 100}

// AppConfig is the window and column layout PortProxyGUI stores in the
// "configs" table.
type AppConfig struct {
	MainWindowWidth  int
	MainWindowHeight int
	ColumnWidths     []int
}

func DefaultAppConfig() AppConfig {
	return AppConfig{
		MainWindowWidth:  DefaultWindowWidth,
		MainWindowHeight: DefaultWindowHeight,
		ColumnWidths:     append([]int(nil), DefaultColumnWidths...),
	}
}

type ConfigRow struct {
	Item  string
	Key   string
	Value string
}

var columnWidthsRE = regexp.MustCompile(`^\[\s*(\d+)(?:\s*,\s*(\d+))*\s*\]$`)

// AppConfigFromRows mirrors PortProxyGUI.Data.AppConfig(Config[] rows).
func AppConfigFromRows(rows []ConfigRow) AppConfig {
	cfg := DefaultAppConfig()

	width, height := "", ""
	for _, r := range rows {
		if r.Item == "MainWindow" {
			switch r.Key {
			case "Width":
				width = r.Value
			case "Height":
				height = r.Value
			}
		}
	}
	if w, err := strconv.Atoi(width); err == nil {
		if h, err := strconv.Atoi(height); err == nil {
			cfg.MainWindowWidth, cfg.MainWindowHeight = w, h
		}
	}

	s := ""
	for _, r := range rows {
		if r.Item == "PortProxy" && r.Key == "ColumnWidths" {
			s = r.Value
			break
		}
	}
	if m := columnWidthsRE.FindStringSubmatch(s); m != nil {
		inner := strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
		parts := strings.Split(inner, ",")
		widths := make([]int, 0, len(parts))
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				widths = append(widths, n)
			}
		}
		cfg.ColumnWidths = widths
	} else {
		cfg.ColumnWidths = []int{}
	}
	return cfg
}

// ColumnWidthsString writes the format PortProxyGUI parses back:
// "[24, 64, 140, 100, 140, 100, 100]".
func ColumnWidthsString(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strconv.Itoa(w)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
