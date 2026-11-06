package main

import (
	"github.com/egoist/mygo/ui"
)

// design is a shadcn-inspired neutral palette with a single indigo accent.
// Everything the built-in widgets draw comes from t; the extra colors are
// for the custom chrome (header, cards, cells, status bar).
type design struct {
	t           *ui.Theme
	dark        bool
	bg          ui.Color
	surface     ui.Color
	surfaceAlt  ui.Color
	border      ui.Color
	text        ui.Color
	muted       ui.Color
	accent      ui.Color
	accentText  ui.Color
	success     ui.Color
	successBg   ui.Color
	danger      ui.Color
	dangerBg    ui.Color
	warning     ui.Color
	focus       ui.Color
	shadow      ui.Color
	soft        ui.Color
	accentHover ui.Color

	sideTop    ui.Color
	sideBottom ui.Color
	sideText   ui.Color
	sideMuted  ui.Color
	sideBorder ui.Color
	sideHover  ui.Color
	sideActive ui.Color
	sideBadge  ui.Color
	statFrom   ui.Color
	statTo     ui.Color
	statText   ui.Color
	glow       ui.Color
	selSwitch  ui.Color
}

func designFor(dark bool) *design {
	d := &design{dark: dark}
	// The sidebar is deliberately dark in both appearances, like a native
	// desktop app's navigation column.
	d.sideTop = ui.Oklch(0.235, 0.045, 266)
	d.sideBottom = ui.Oklch(0.155, 0.032, 266)
	d.sideText = ui.Oklch(0.975, 0.005, 260)
	d.sideMuted = ui.Oklch(0.68, 0.022, 266)
	d.sideBorder = ui.Oklch(0.33, 0.032, 266)
	d.sideHover = ui.Oklch(0.295, 0.045, 266)
	d.sideActive = ui.Oklch(0.345, 0.06, 266)
	d.sideBadge = ui.Oklch(0.31, 0.045, 266)
	d.statFrom = ui.Oklch(0.53, 0.19, 266)
	d.statTo = ui.Oklch(0.62, 0.17, 296)
	d.statText = ui.Oklch(0.99, 0, 0)
	d.glow = ui.Oklch(0.68, 0.16, 266).Alpha(0.45)
	// A dark track keeps the white switch knob visible on the accent-colored
	// selected row (mygo paints a chosen row with theme.Accent).
	d.selSwitch = ui.Oklch(0.30, 0.045, 266)
	if dark {
		d.bg = ui.Oklch(0.175, 0.012, 260)
		d.surface = ui.Oklch(0.205, 0.012, 260)
		d.surfaceAlt = ui.Oklch(0.245, 0.012, 260)
		d.border = ui.Oklch(0.305, 0.012, 260)
		d.text = ui.Oklch(0.965, 0.005, 260)
		d.muted = ui.Oklch(0.705, 0.014, 260)
		d.accent = ui.Oklch(0.705, 0.145, 265)
		d.accentHover = ui.Oklch(0.775, 0.13, 265)
		d.accentText = ui.Oklch(0.175, 0.012, 260)
		d.success = ui.Oklch(0.735, 0.16, 152)
		d.successBg = ui.Oklch(0.27, 0.06, 152)
		d.danger = ui.Oklch(0.705, 0.18, 27)
		d.dangerBg = ui.Oklch(0.28, 0.06, 27)
		d.warning = ui.Oklch(0.79, 0.15, 70)
		d.focus = ui.Oklch(0.705, 0.145, 265)
		d.soft = ui.Oklch(0.235, 0.012, 260)
		d.shadow = ui.RGBA(0, 0, 0, 0.32)
	} else {
		d.bg = ui.Oklch(0.985, 0.004, 260)
		d.surface = ui.Oklch(1, 0, 0)
		d.surfaceAlt = ui.Oklch(0.972, 0.005, 260)
		d.border = ui.Oklch(0.918, 0.006, 260)
		d.text = ui.Oklch(0.22, 0.018, 260)
		d.muted = ui.Oklch(0.55, 0.014, 260)
		d.accent = ui.Oklch(0.52, 0.17, 265)
		d.accentHover = ui.Oklch(0.455, 0.18, 265)
		d.accentText = ui.Oklch(0.99, 0, 0)
		d.success = ui.Oklch(0.60, 0.15, 152)
		d.successBg = ui.Oklch(0.95, 0.055, 152)
		d.danger = ui.Oklch(0.58, 0.20, 27)
		d.dangerBg = ui.Oklch(0.955, 0.04, 27)
		d.warning = ui.Oklch(0.68, 0.15, 70)
		d.focus = ui.Oklch(0.60, 0.16, 265)
		d.soft = ui.Oklch(0.963, 0.006, 260)
		d.shadow = ui.RGBA(24, 28, 45, 0.10)
	}

	var base ui.Theme
	if dark {
		base = *ui.DarkTheme()
	} else {
		base = *ui.LightTheme()
	}
	base.Dark = dark
	base.Background = d.bg
	base.Surface = d.surfaceAlt
	base.SurfaceHover = d.soft
	base.SurfacePressed = d.border
	base.Border = d.border
	base.Text = d.text
	base.TextMuted = d.muted
	base.Accent = d.accent
	base.AccentHover = d.accentHover
	base.AccentPressed = d.accentHover
	base.AccentText = d.accentText
	base.Danger = d.danger
	base.Warning = d.warning
	base.Success = d.success
	base.Selection = d.accent.Alpha(0.25)
	base.Focus = d.focus.Alpha(0.55)
	base.Scrollbar = d.muted.Alpha(0.35)
	base.Inverse = d.text
	base.InverseText = d.bg
	base.Radius = 10
	base.Spacing = 4
	base.FontSize = 13
	d.t = &base
	return d
}

// lucide wraps a Lucide path in the standard 24×24 stroke icon document.
func lucide(inner string) *ui.SVG {
	return ui.MustParseSVG([]byte(
		`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" ` +
			`stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` +
			inner + `</svg>`))
}

var (
	icoLogo = lucide(
		`<path d="M4 7h13"/><path d="m14 4 3 3-3 3"/><path d="M20 17H7"/><path d="m10 14-3 3 3 3"/>`,
	)
	icoPlus    = lucide(`<path d="M12 5v14"/><path d="M5 12h14"/>`)
	icoRefresh = lucide(`<path d="M21 12a9 9 0 1 1-2.64-6.36L21 8"/><path d="M21 3v5h-5"/>`)
	icoZap     = lucide(`<path d="M13 2 4 14h7l-1 8 9-12h-7l1-8Z"/>`)
	icoMore    = lucide(
		`<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
	)
	icoEdit = lucide(
		`<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/>`,
	)
	icoChevron = lucide(`<path d="m6 9 6 6 6-6"/>`)
	icoAlert   = lucide(
		`<path d="m21.7 18-8-14a2 2 0 0 0-3.4 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.7-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>`,
	)
	icoActivity = lucide(`<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>`)
	icoArrow    = lucide(`<path d="M5 12h14"/><path d="m13 6 6 6-6 6"/>`)
	icoLayers   = lucide(
		`<path d="m12 2 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5"/><path d="m3 17 9 5 9-5"/>`,
	)
	icoInbox = lucide(
		`<path d="M22 12h-6l-2 3h-4l-2-3H2"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>`,
	)
	icoFolder = lucide(
		`<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>`,
	)
	icoSearch  = lucide(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`)
	icoTrash   = lucide(`<path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="m19 6-1 14H6L5 6"/>`)
	icoClose   = lucide(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)
	icoSliders = lucide(
		`<path d="M4 21v-7"/><path d="M4 10V3"/><path d="M12 21v-9"/><path d="M12 8V3"/><path d="M20 21v-5"/><path d="M20 12V3"/><path d="M1 14h6"/><path d="M9 8h6"/><path d="M17 16h6"/>`,
	)
	icoChevronLeft  = lucide(`<path d="m15 18-6-6 6-6"/>`)
	icoChevronRight = lucide(`<path d="m9 18 6-6-6-6"/>`)
)

// pill is a small rounded status label.
func pill(c *ui.Context, text string, fg, bg ui.Color) ui.Element {
	return ui.Box(c).Padding(2, 8).Radius(999).Background(bg).Children(func() {
		ui.Text(c, text).FontSize(11).FontWeight(600).SingleLine().TextColor(fg)
	})
}
