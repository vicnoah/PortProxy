package main

import (
	"github.com/egoist/mygo/ui"
	"portproxy/locales"
)

func (a *app) confirmDeleteModal(c *ui.Context, d *design) {
	f := &a.deleteConfirm
	modal := ui.Modal(c, &f.open, func() {
		ui.Column(c).Width(400).Gap(16).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(34, 34).Radius(10).Background(d.dangerBg).
					Center().Shrink(0).Children(func() {
					ui.Icon(c, icoAlert).FontSize(17).TextColor(d.danger)
				})
				ui.Text(c, locales.ConfirmDeleteTitle()).
					FontSize(16).FontWeight(600).SingleLine()
			})
			ui.Divider(c)
			ui.Text(c, locales.ConfirmDeleteBody(len(f.ids))).
				FontSize(12.5).TextColor(d.text).LineHeight(1.5).MaxLines(2)

			ui.Column(c).Gap(6).Padding(10, 12).Radius(8).
				Background(d.soft).Children(func() {
				shown := min(len(f.titles), 3)
				for _, title := range f.titles[:shown] {
					ui.Text(c, title).Font("monospace").FontSize(11).
						TextColor(d.muted).SingleLine()
				}
				if len(f.titles) > shown {
					ui.Text(c, locales.MoreRules(len(f.titles)-shown)).
						FontSize(11).TextColor(d.muted).SingleLine()
				}
			})

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					a.deleteConfirm = deleteConfirmForm{}
				}
				del := ui.PrimaryButton(c, locales.ConfirmDeleteButton())
				del.Background(d.danger).TextColor(d.statText)
				if del.Clicked() {
					a.later(a.confirmDelete)
				}
			})
		})
	})
	if modal.Dismissed() {
		a.deleteConfirm = deleteConfirmForm{}
	}
}
