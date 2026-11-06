package main

import (
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
	"portproxy/locales"
)

func (a *app) aboutModal(c *ui.Context, d *design) {
	ui.Modal(c, &a.about, func() {
		ui.Column(c).Width(360).Gap(16).Children(func() {
			ui.Column(c).Center().Gap(10).Children(func() {
				ui.Box(c).Size(56, 56).Radius(16).Background(d.accent).Center().Children(func() {
					ui.Icon(c, icoLogo).FontSize(30).TextColor(d.accentText)
				})
				ui.Text(c, appTitle).FontSize(20).FontWeight(600).SingleLine()
				pill(c, "v"+appVersion, d.accent, d.accent.Alpha(0.11))
			})

			ui.Divider(c)

			ui.Text(c, locales.AboutDesc1()).FontSize(12.5).TextColor(d.text).
				TextAlign(ui.Center).LineHeight(1.5)
			ui.Text(c, locales.AboutDesc2()).
				FontSize(11.5).TextColor(d.muted).TextAlign(ui.Center).LineHeight(1.5)

			ui.Row(c).Gap(8).Justify(ui.Center).Children(func() {
				if ui.Button(c, locales.CheckForUpdates()).Clicked() {
					updater.CheckForUpdates()
				}
				if ui.Button(c, locales.Close()).Clicked() {
					a.about = false
				}
			})
		})
	})
}
