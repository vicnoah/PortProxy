package main

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"portproxy/locales"
)

func formField(c *ui.Context, d *design, label string, width float32, build func()) {
	col := ui.Column(c).Gap(6)
	if width > 0 {
		col.Width(width).Shrink(0)
	} else {
		col.Grow(1)
	}
	col.Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(d.muted).SingleLine()
		build()
	})
}

func (a *app) setProxyModal(c *ui.Context, d *design) {
	f := &a.set
	opts := typeOptions()
	// A language switch while the dialog is open must not leave a stale
	// localized "(Auto)" value that Select cannot show.
	if !slices.Contains(opts, f.typ) {
		f.typ = opts[0]
	}
	modal := ui.Modal(c, &f.open, func() {
		ui.Column(c).Width(466).Gap(16).Children(func() {
			ui.Row(c).Gap(11).AlignItems(ui.Center).Children(func() {
				icon := icoPlus
				if f.update {
					icon = icoEdit
				}
				ui.Box(c).Size(34, 34).Radius(10).Background(d.accent.Alpha(0.12)).
					Center().Shrink(0).Children(func() {
					ui.Icon(c, icon).FontSize(17).TextColor(d.accent)
				})
				ui.Column(c).Gap(2).Children(func() {
					title, sub := locales.DlgNewTitle(), locales.DlgNewSub()
					if f.update {
						title, sub = locales.DlgEditTitle(), locales.DlgEditSub()
					}
					ui.Text(c, title).FontSize(16).FontWeight(600).SingleLine()
					ui.Text(c, sub).FontSize(11.5).TextColor(d.muted).SingleLine()
				})
			})
			ui.Divider(c)

			formField(c, d, locales.TypeLabel(), 0, func() {
				ui.Select(c, &f.typ, opts).Width(200).Label(locales.TypeLabel())
			})

			ui.Row(c).Gap(12).Children(func() {
				formField(c, d, locales.ListenOn(), 0, func() {
					ui.TextInput(c, &f.listenOn).Placeholder(locales.PlaceholderListenOn()).
						Label(locales.ListenOn()).FillWidth()
				})
				formField(c, d, locales.ListenPort(), 116, func() {
					ui.TextInput(c, &f.listenPort).Placeholder(locales.PlaceholderListenPort()).
						Label(locales.ListenPort()).Width(116)
				})
			})

			ui.Row(c).Justify(ui.Center).Children(func() {
				ui.Icon(c, icoArrow).FontSize(16).TextColor(d.muted.Alpha(0.7))
			})

			ui.Row(c).Gap(12).Children(func() {
				formField(c, d, locales.ConnectTo(), 0, func() {
					ui.TextInput(c, &f.connectTo).Placeholder(locales.PlaceholderConnectTo()).
						Label(locales.ConnectTo()).FillWidth()
				})
				formField(c, d, locales.ConnectPort(), 116, func() {
					ui.TextInput(c, &f.connectPort).Placeholder(locales.PlaceholderConnectPort()).
						Label(locales.ConnectPort()).Width(116)
				})
			})

			formField(c, d, locales.Comment(), 0, func() {
				ui.TextInput(c, &f.comment).Placeholder(locales.PlaceholderComment()).
					Label(locales.Comment()).FillWidth()
			})
			formField(c, d, locales.Group(), 0, func() {
				ui.Combobox(c, &f.group, a.groupNames()).
					Placeholder(locales.PlaceholderGroup()).Label(locales.Group()).FillWidth()
			})

			if f.err != "" {
				ui.Row(c).Gap(8).Padding(9, 11).Radius(8).Background(d.dangerBg).
					AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icoAlert).FontSize(14).TextColor(d.danger)
					ui.Text(c, f.err).FontSize(11.5).TextColor(d.danger).SingleLine().Shrink(1)
				})
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					f.open, f.err = false, ""
				}
				if ui.PrimaryButton(c, locales.Set()).Clicked() {
					a.later(a.applySetProxy)
				}
			})
		})
	})
	if modal.Dismissed() {
		f.err = ""
	}
}
