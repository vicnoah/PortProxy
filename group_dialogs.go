package main

import (
	"github.com/egoist/mygo/ui"
	"portproxy/locales"
)

func (a *app) groupModal(c *ui.Context, d *design) {
	f := &a.group
	modal := ui.Modal(c, &f.open, func() {
		title, action := locales.NewGroup(), locales.Create()
		if f.rename {
			title, action = locales.RenameGroup(), locales.Rename()
		}
		ui.Column(c).Width(360).Gap(16).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(34, 34).Radius(10).
					Background(d.accent.Alpha(0.12)).Center().Shrink(0).Children(func() {
					ui.Icon(c, icoFolder).FontSize(18).TextColor(d.accent)
				})
				ui.Text(c, title).FontSize(16).FontWeight(600).SingleLine()
			})
			ui.Divider(c)
			ui.Text(c, locales.GroupNameLabel()).FontSize(11.5).
				FontWeight(600).TextColor(d.muted).SingleLine()
			ui.TextInput(c, &f.name).Placeholder(locales.GroupNamePlaceholder()).
				Label(locales.GroupNameLabel()).AutoFocus().FillWidth()
			if f.err != "" {
				ui.Row(c).Gap(8).Padding(9, 11).Radius(8).Background(d.dangerBg).
					AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icoAlert).FontSize(14).TextColor(d.danger)
					ui.Text(c, f.err).FontSize(11.5).TextColor(d.danger).
						SingleLine().Shrink(1)
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					f.open, f.err = false, ""
				}
				if ui.PrimaryButton(c, action).Clicked() {
					a.later(a.applyGroupForm)
				}
			})
		})
	})
	if modal.Dismissed() {
		f.err = ""
	}
}

func (a *app) confirmGroupModal(c *ui.Context, d *design) {
	modal := ui.Modal(c, &a.confirmOpen, func() {
		ui.Column(c).Width(360).Gap(16).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(34, 34).Radius(10).Background(d.dangerBg).
					Center().Shrink(0).Children(func() {
					ui.Icon(c, icoAlert).FontSize(17).TextColor(d.danger)
				})
				ui.Text(c, locales.ConfirmDeleteGroupTitle()).
					FontSize(16).FontWeight(600).SingleLine()
			})
			ui.Divider(c)
			ui.Text(c, locales.ConfirmDeleteGroupBody(a.confirmName)).
				FontSize(12.5).TextColor(d.text).LineHeight(1.55).MaxLines(3)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					a.confirmOpen = false
				}
				del := ui.PrimaryButton(c, locales.DeleteGroupButton())
				del.Background(d.danger).TextColor(d.statText)
				if del.Clicked() {
					name := a.confirmName
					a.later(func() { a.deleteGroup(name) })
				}
			})
		})
	})
	if modal.Dismissed() {
		a.confirmOpen = false
	}
}
