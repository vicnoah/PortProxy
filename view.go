package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
	"portproxy/locales"
)

func (a *app) view(c *ui.Context) {
	d := a.applyTheme(c)
	a.normalizeFilters()
	a.syncRows()

	ui.Row(c).Fill().Children(func() {
		a.sidebar(c, d)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().
			Background(d.bg).Clip().Children(func() {
			a.topbar(c, d)
			a.statsRow(c, d)
			a.content(c, d)
			a.statusBar(c, d)
		})
	})

	a.setProxyModal(c, d)
	a.aboutModal(c, d)
	a.groupModal(c, d)
	a.confirmGroupModal(c, d)
	a.confirmDeleteModal(c, d)
	a.shortcuts(c)
	a.runPending(c)

	if a.toastMsg != "" {
		c.AddToast(ui.Toast{Title: a.toastMsg, Type: a.toastKind})
		a.toastMsg, a.toastKind = "", ""
	}
}

func (a *app) viewTitle() string {
	switch a.filterGroup {
	case filterAll:
		return locales.AllRules()
	case filterUngrouped:
		return locales.Ungrouped()
	default:
		return a.filterGroup
	}
}

func sideItem(c *ui.Context, d *design, icon *ui.SVG, label string, count int,
	active bool, onClick func()) ui.Element {
	b := ui.ButtonBase(c).Label(label)
	hover := b.Hovered()
	bg := ui.Transparent
	if active {
		bg = d.sideActive
	} else if hover {
		bg = d.sideHover
	}
	fg := d.sideMuted
	if active {
		fg = d.sideText
	}
	bar := ui.Transparent
	if active {
		bar = d.statFrom
	}
	b.Background(bg).Height(38).Radius(9).Gap(9).Padding(0, 9).MarginX(10).
		AlignItems(ui.Center).
		Transition(ui.ElementTransition{Duration: 140 * time.Millisecond, Colors: true})
	b.Children(func() {
		ui.Box(c).Size(3, 18).Radius(2).Background(bar).Shrink(0)
		ui.Icon(c, icon).FontSize(15).TextColor(fg)
		ui.Text(c, label).FontSize(12.5).SingleLine().TextColor(fg).
			Grow(1).Shrink(1).FontWeight(map[bool]int{true: 600, false: 400}[active])
		if count >= 0 {
			ui.Box(c).Padding(1, 7).Radius(999).Background(d.sideBadge).Children(func() {
				ui.Text(c, strconv.Itoa(count)).FontSize(10.5).
					FontWeight(600).TextColor(d.sideMuted).SingleLine()
			})
		}
	})
	if b.Clicked() {
		onClick()
	}
	return b
}

func (a *app) sidebar(c *ui.Context, d *design) {
	ui.Column(c).Width(232).Shrink(0).FillHeight().
		Gradient(d.sideTop, d.sideBottom, 160).Children(func() {
		ui.Box(c).Height(3).Gradient(d.statFrom, d.statTo, 90)
		ui.Row(c).Height(86).Shrink(0).Padding(0, 18).Gap(11).
			AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(38, 38).Radius(11).Background(d.statFrom).Center().
				Shadow(0, 4, 14, 0, d.glow).Shrink(0).Children(func() {
				ui.Icon(c, icoLogo).FontSize(21).TextColor(d.statText)
			})
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, appTitle).FontSize(17).FontWeight(700).
					TextColor(d.sideText).SingleLine()
				ui.Text(c, locales.AppSubtitle()).FontSize(10.5).
					TextColor(d.sideMuted).SingleLine()
			})
		})
		ui.Box(c).Height(1).MarginX(18).MarginY(4).Background(d.sideBorder)

		ui.Box(c).Padding(0, 20).Height(30).Children(func() {
			ui.Text(c, locales.Workspace()).FontSize(10).FontWeight(700).
				LetterSpacing(1.2).TextColor(d.sideMuted).SingleLine()
		})
		sideItem(c, d, icoLayers, locales.AllRules(), a.totalRules,
			a.filterGroup == filterAll,
			func() { a.later(func() { a.setFilter(filterAll) }) })
		sideItem(c, d, icoInbox, locales.Ungrouped(), a.ungrouped,
			a.filterGroup == filterUngrouped,
			func() { a.later(func() { a.setFilter(filterUngrouped) }) })

		ui.Row(c).Height(36).Shrink(0).MarginY(8).Padding(0, 20).
			AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.GroupsSection()).FontSize(10).FontWeight(700).
				LetterSpacing(1.2).TextColor(d.sideMuted).SingleLine()
			ui.Spacer(c)
			plus := ui.ButtonBase(c).Label(locales.NewGroup()).
				Tooltip(locales.NewGroup())
			if plus.Hovered() {
				plus.Background(d.sideHover)
			}
			plus.Size(22, 22).Radius(6).Center().Children(func() {
				ui.Icon(c, icoPlus).FontSize(14).TextColor(d.sideMuted)
			})
			if plus.Clicked() {
				a.later(a.openNewGroup)
			}
		})

		for _, g := range a.sideGroups {
			name := g
			item := sideItem(c, d, icoFolder, name, a.groupCounts[name],
				a.filterGroup == name,
				func() { a.later(func() { a.setFilter(name) }) })
			item.ContextMenu(func(m *ui.Menu) {
				if m.Item(locales.NewRule()).Chosen() {
					a.later(func() { a.setFilter(name); a.openNewProxy() })
				}
				m.Separator()
				if m.Item(locales.RenameGroup()).Chosen() {
					a.later(func() { a.openRenameGroup(name) })
				}
				if m.Item(locales.DeleteGroup()).Chosen() {
					a.later(func() { a.openDeleteGroup(name) })
				}
			})
		}

		ui.Spacer(c)

		ui.Column(c).Padding(0, 18, 16, 18).Gap(9).Children(func() {
			ui.Box(c).Height(1).Background(d.sideBorder)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				dot := d.success
				text := locales.ServiceRunning()
				if !a.serviceRunning {
					dot, text = d.danger, locales.ServiceStopped()
				}
				ui.Box(c).Size(7, 7).Radius(999).Background(dot).
					Shadow(0, 0, 6, 0, dot.Alpha(0.8))
				ui.Text(c, text).FontSize(11).TextColor(d.sideMuted).SingleLine()
			})
			ui.Text(c, "v"+appVersion).FontSize(10).
				TextColor(d.sideMuted.Alpha(0.55)).SingleLine()
		})
	})
}

func iconButton(c *ui.Context, label string, icon *ui.SVG, primary bool) ui.Element {
	if primary {
		return ui.PrimaryButton(c, "").Label(label).Children(func() {
			ui.Icon(c, icon).FontSize(15)
			ui.Text(c, label).SingleLine().FontWeight(600)
		})
	}
	return ui.Button(c, "").Label(label).Children(func() {
		ui.Icon(c, icon).FontSize(15)
		ui.Text(c, label).SingleLine()
	})
}

func iconOnlyButton(c *ui.Context, d *design, icon *ui.SVG, label string, fn func()) {
	b := ui.ButtonBase(c).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(d.surfaceAlt)
	}
	b.Background(d.surface).Size(34, 34).Radius(9).Center().
		Border(1, d.border).Children(func() {
		ui.Icon(c, icon).FontSize(16).TextColor(d.muted)
	})
	if b.Clicked() {
		fn()
	}
}

func (a *app) topbar(c *ui.Context, d *design) {
	ui.Row(c).Height(78).Shrink(0).Padding(0, 24).Gap(9).
		AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(3).Shrink(1).MinWidth(0).Children(func() {
			ui.Text(c, a.viewTitle()).FontSize(21).FontWeight(650).
				LetterSpacing(-0.3).SingleLine()
			ui.Text(c, locales.RulesInView(a.visibleRules, a.totalRules)).
				FontSize(11.5).TextColor(d.muted).SingleLine()
		})
		ui.Spacer(c)
		ui.SearchField(c, &a.query).Width(190).MinWidth(120).Shrink(1).
			Label(locales.SearchRules())
		iconOnlyButton(c, d, icoRefresh, locales.Refresh(),
			func() { a.later(a.doRefresh) })
		iconOnlyButton(c, d, icoZap, locales.FlushDNS(),
			func() { a.later(a.flushDNS) })
		if iconButton(c, locales.NewRule(), icoPlus, true).Clicked() {
			a.later(a.openNewProxy)
		}
		more := ui.Button(c, "").Label(locales.More()).Menu(a.buildMoreMenu).Children(func() {
			ui.Icon(c, icoMore).FontSize(16)
			ui.Text(c, locales.More()).SingleLine()
			ui.Icon(c, icoChevron).FontSize(13).TextColor(d.muted)
		})
		_ = more
	})
}

func statCard(c *ui.Context, d *design, icon *ui.SVG, value, label string, accent bool) {
	card := ui.Row(c).Grow(1).Height(80).Padding(0, 16).Gap(13).
		AlignItems(ui.Center).Radius(12).Border(1, d.border).
		Shadow(0, 2, 8, 0, d.shadow)
	fg, muted, tile := d.text, d.muted, d.soft
	if accent {
		card.Gradient(d.statFrom, d.statTo, 135)
		fg, muted, tile = d.statText, d.statText.Alpha(0.82), d.statText.Alpha(0.18)
	} else {
		card.Background(d.surface)
	}
	card.Children(func() {
		ui.Box(c).Size(42, 42).Radius(12).Background(tile).Center().Children(func() {
			ui.Icon(c, icon).FontSize(20).TextColor(fg)
		})
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, value).FontSize(24).FontWeight(700).
				FontFeatures("tnum").TextColor(fg).SingleLine()
			ui.Text(c, label).FontSize(11).TextColor(muted).SingleLine()
		})
	})
}

func (a *app) statsRow(c *ui.Context, d *design) {
	ui.Row(c).Height(96).Shrink(0).Padding(0, 24, 8, 24).Gap(12).Children(func() {
		statCard(c, d, icoLayers, strconv.Itoa(a.totalRules), locales.StatRules(), true)
		statCard(c, d, icoActivity, strconv.Itoa(a.activeRules), locales.StatActive(), false)
		statCard(c, d, icoFolder, strconv.Itoa(a.groupTotal), locales.StatGroups(), false)
	})
}

func pageSizeOptions() []string {
	out := make([]string, len(pageSizes))
	for i, n := range pageSizes {
		out[i] = strconv.Itoa(n)
	}
	return out
}

func (a *app) tableColumns() []ui.TableColumn {
	widths := a.displayWidths()
	titles := columnTitles()
	cols := make([]ui.TableColumn, 0, len(columnIDs)+1)
	for i, id := range columnIDs {
		col := ui.TableColumn{
			ID: id, Title: titles[i],
			Width:    float32(widths[i]),
			MinWidth: float32(minColumnWidths[i]),
			Sortable: id != "status",
		}
		if id == "comment" {
			col.Width = 0
			col.MinWidth = 160
		}
		if id == "listenPort" || id == "connectPort" {
			col.Align = ui.End
		}
		cols = append(cols, col)
	}
	// A dedicated fixed column keeps row actions aligned regardless of how
	// long (or empty) the comment in the previous column is. It is not part
	// of columnIDs, so config.db keeps its 7 configurable column widths.
	cols = append(cols, ui.TableColumn{
		ID: "actions", Title: locales.ColActions(),
		Width: 78, MinWidth: 78, Fixed: true, Sortable: false,
	})
	return cols
}

func (a *app) filterBar(c *ui.Context, d *design) {
	ui.Row(c).Height(48).Shrink(0).Padding(0, 14).Gap(9).
		AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icoSliders).FontSize(15).TextColor(d.muted)
		ui.Segmented(c, &a.statusIndex, locales.FilterAll(),
			locales.LegendEnabled(), locales.LegendDisabled()).
			Label(locales.FilterAll())
		ui.Select(c, &a.typeChoice, typeFilterOptions()).Width(132).
			Label(locales.FilterAllTypes())
		if a.filterGroup != filterAll {
			name := a.filterGroup
			if name == filterUngrouped {
				name = locales.Ungrouped()
			}
			chip := ui.Row(c).Gap(5).Padding(4, 8).Radius(999).
				Background(d.accent.Alpha(0.12)).AlignItems(ui.Center).Shrink(0)
			chip.Children(func() {
				ui.Icon(c, icoFolder).FontSize(12).TextColor(d.accent)
				ui.Text(c, name).FontSize(11).FontWeight(600).
					TextColor(d.accent).SingleLine().MaxLines(1)
				clear := ui.ButtonBase(c).Label(locales.ClearFilters()).
					Children(func() {
						ui.Icon(c, icoClose).FontSize(11).TextColor(d.accent)
					})
				clear.Size(16, 16).Center()
				if clear.Clicked() {
					a.later(func() { a.setFilter(filterAll) })
				}
			})
		}
		ui.Spacer(c)
		if a.hasFilters() {
			if ui.Button(c, locales.ClearFilters()).Clicked() {
				a.later(a.clearFilters)
			}
		}
		ui.Text(c, locales.FilterResults(a.visibleRules)).
			FontSize(11).TextColor(d.muted).SingleLine()
	})
}

func (a *app) content(c *ui.Context, d *design) {
	ui.Box(c).Grow(1).Shrink(1).MinWidth(0).Padding(0, 24, 16, 24).Children(func() {
		ui.Column(c).Fill().MinWidth(0).Radius(12).Background(d.surface).
			Border(1, d.border).Shadow(0, 2, 10, 0, d.shadow).Clip().Children(func() {
			a.filterBar(c, d)
			ui.Divider(c)
			if len(a.rows) == 0 {
				a.emptyState(c, d)
			} else {
				tbl := ui.Table(c, &a.list, a.tableColumns(), len(a.rows),
					func(r, col int) { a.cell(c, d, r, col) })
				tbl.Grow(1).Label(locales.TableLabel()).ContextMenu(a.buildContextMenu)
				if tbl.Submitted() {
					a.later(a.openModifyProxy)
				}
			}
			ui.Divider(c)
			a.tableFooter(c, d)
		})
	})
}

func legend(c *ui.Context, d *design, color ui.Color, text string) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(7, 7).Radius(999).Background(color)
		ui.Text(c, text).FontSize(11).TextColor(d.muted).SingleLine()
	})
}

func pagerButton(c *ui.Context, label string, icon *ui.SVG,
	disabled bool, fn func()) {
	b := ui.Button(c, "").Label(label).Tooltip(label).Disabled(disabled)
	b.Size(28, 28).Children(func() {
		ui.Icon(c, icon).FontSize(14)
	})
	if b.Clicked() {
		fn()
	}
}

func (a *app) tableFooter(c *ui.Context, d *design) {
	pages := a.pageCount(a.visibleRules)
	ui.Row(c).Height(44).Shrink(0).Padding(0, 14).Gap(14).
		AlignItems(ui.Center).Children(func() {
		legend(c, d, d.success, locales.LegendEnabled())
		legend(c, d, d.muted.Alpha(0.4), locales.LegendDisabled())
		ui.Spacer(c)
		ui.Text(c, locales.PageSizeLabel()).FontSize(11).
			TextColor(d.muted).SingleLine()
		choice := strconv.Itoa(a.pageSize)
		if ui.Select(c, &choice, pageSizeOptions()).Width(72).
			Label(locales.PageSizeLabel()).Changed() {
			n, err := strconv.Atoi(choice)
			if err == nil {
				size := n
				a.later(func() { a.setPageSize(size) })
			}
		}
		pagerButton(c, locales.PagePrev(), icoChevronLeft, a.page <= 0,
			func() { a.later(func() { a.gotoPage(a.page - 1) }) })
		ui.Text(c, locales.PageOf(a.page+1, pages)).FontSize(11).
			TextColor(d.text).SingleLine()
		pagerButton(c, locales.PageNext(), icoChevronRight, a.page >= pages-1,
			func() { a.later(func() { a.gotoPage(a.page + 1) }) })
	})
}

func (a *app) emptyState(c *ui.Context, d *design) {
	title, body := locales.EmptyTitle(), locales.EmptyBody()
	icon := icoActivity
	if a.totalRules > 0 {
		title, body = locales.EmptyFilterTitle(), locales.EmptyFilterBody()
		if a.query != "" {
			icon = icoSearch
		}
	}
	ui.Column(c).Fill().Center().Gap(12).Padding(24).Children(func() {
		ui.Box(c).Size(58, 58).Radius(18).Background(d.soft).Center().Children(func() {
			ui.Icon(c, icon).FontSize(28).TextColor(d.muted)
		})
		ui.Text(c, title).FontSize(17).FontWeight(600).TextAlign(ui.Center)
		ui.Text(c, body).FontSize(12).TextColor(d.muted).
			TextAlign(ui.Center).MaxLines(2).MaxWidth(380)
		ui.Row(c).Gap(8).Children(func() {
			if iconButton(c, locales.NewRule(), icoPlus, true).Clicked() {
				a.later(a.openNewProxy)
			}
			if a.hasFilters() {
				if ui.Button(c, locales.ClearFilters()).Clicked() {
					a.later(a.clearFilters)
				}
			}
		})
	})
}

func tag(c *ui.Context, text string, fg, bg ui.Color) {
	ui.Box(c).Padding(1, 5).Radius(5).Background(bg).Children(func() {
		ui.Text(c, text).FontSize(10.5).FontWeight(500).SingleLine().TextColor(fg)
	})
}

func mono(c *ui.Context, s string, color ui.Color) {
	ui.Text(c, s).Font("monospace").FontSize(12).SingleLine().TextColor(color)
}

func rowButton(c *ui.Context, d *design, icon *ui.SVG, label string,
	danger bool, chosen bool, fn func()) {
	b := ui.ButtonBase(c).Label(label).Tooltip(label)
	hover := b.Hovered()
	bg := ui.Transparent
	if hover {
		bg = d.soft
		if danger {
			bg = d.dangerBg
		}
		if chosen {
			bg = d.accentText.Alpha(0.22)
		}
	}
	fg := d.muted
	if chosen {
		fg = d.accentText.Alpha(0.9)
	}
	if hover && danger && !chosen {
		fg = d.danger
	}
	b.Background(bg).Size(24, 24).Radius(6).Center().Children(func() {
		ui.Icon(c, icon).FontSize(13).TextColor(fg)
	})
	if b.Clicked() {
		fn()
	}
}

func (a *app) cell(c *ui.Context, d *design, r, col int) {
	// A deferred action may have shrunk a.rows after this frame's row count
	// was captured; guard every access so the table can never index past it.
	if r < 0 || r >= len(a.rows) {
		return
	}
	rw := a.rows[r]
	if rw.header {
		if col == 0 {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(3, 14).Radius(2).Background(d.statFrom)
				ui.Icon(c, icoFolder).FontSize(13).TextColor(d.accent)
				ui.Text(c, rw.title).FontWeight(600).FontSize(12).
					SingleLine().TextColor(d.text)
				ui.Box(c).Padding(0, 6).Radius(999).Background(d.soft).Children(func() {
					ui.Text(c, fmt.Sprintf("%d", rw.count)).
						FontSize(10).TextColor(d.muted).SingleLine()
				})
			})
		}
		return
	}
	rule := rw.rule
	// mygo paints a chosen row with theme.Accent and inherits AccentText,
	// but our cells set explicit colors, which would otherwise stay dark on
	// the dark-blue highlight. Switch to light text for selected rows.
	chosen := a.selected == r || a.selection.Has(rw.key)
	fg, muted := d.text, d.muted
	tagFg, tagBg := d.accent, d.accent.Alpha(0.11)
	if chosen {
		fg, muted = d.accentText, d.accentText.Alpha(0.85)
		tagFg, tagBg = d.accentText, d.accentText.Alpha(0.22)
	}
	switch col {
	case 0:
		on := rw.active
		sw := ui.Switch(c, &on).Label(locales.Enable())
		if chosen {
			sw.Background(d.selSwitch)
		}
		if sw.Changed() {
			a.later(func() { a.toggleRule(rule, on) })
		}
	case 1:
		tag(c, rule.Type, tagFg, tagBg)
	case 2:
		mono(c, rule.ListenOn, fg)
	case 3:
		mono(c, strconv.Itoa(rule.ListenPort), fg)
	case 4:
		mono(c, rule.ConnectTo, fg)
	case 5:
		mono(c, strconv.Itoa(rule.ConnectPort), fg)
	case 6:
		text, color := "—", muted.Alpha(0.65)
		if strings.TrimSpace(rule.Comment) != "" {
			text, color = rule.Comment, muted
			ui.Text(c, text).FontSize(12).TextColor(color).
				SingleLine().Tooltip(text)
		} else {
			ui.Text(c, text).FontSize(12).TextColor(color).SingleLine()
		}
	case 7:
		ui.Row(c).Gap(2).AlignItems(ui.Center).FillWidth().
			Justify(ui.Center).Children(func() {
			rowButton(c, d, icoEdit, locales.ModifyItem(), false, chosen,
				func() { a.later(func() { a.openModifyFor(rule) }) })
			rowButton(c, d, icoTrash, locales.Delete(), true, chosen,
				func() { a.later(func() { a.deleteRule(rule) }) })
		})
	}
}

func (a *app) statusBar(c *ui.Context, d *design) {
	message := a.status
	if message == "" {
		message = locales.Welcome()
	}
	ui.Row(c).Height(40).Shrink(0).Padding(0, 24).Gap(10).
		BorderWidth(1, 0, 0, 0).BorderColor(d.border).
		AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(d.success).
			Shadow(0, 0, 6, 0, d.success.Alpha(0.7)).Shrink(0)
		ui.Text(c, message).FontSize(11.5).TextColor(d.muted).
			SingleLine().Shrink(1)
		ui.Spacer(c)
		if !a.serviceRunning && a.serviceKnown {
			warn := ui.ButtonBase(c).Label(locales.ServiceWarning()).Children(func() {
				ui.Icon(c, icoAlert).FontSize(13)
				ui.Text(c, locales.ServiceWarning()).
					FontSize(11.5).FontWeight(600).SingleLine()
			})
			warn.Background(d.dangerBg).TextColor(d.danger).Radius(999).
				Padding(5, 10).Gap(6).AlignItems(ui.Center)
			if warn.Clicked() {
				a.later(a.startService)
			}
		} else {
			ui.Text(c, locales.NetshHint()).FontSize(11).
				TextColor(d.muted.Alpha(0.75)).SingleLine()
		}
	})
}

func (a *app) buildContextMenu(m *ui.Menu) {
	ids := a.selectedIDs()
	anySel, oneSel := len(ids) > 0, len(ids) == 1
	anyDisabled, anyEnabled := false, false
	for _, id := range ids {
		if r := a.ruleByID(id); r != nil {
			if a.active[ruleKey(*r)] {
				anyEnabled = true
			} else {
				anyDisabled = true
			}
		}
	}
	if m.Item(locales.Enable()).Disabled(!anyDisabled).
		Shortcut(ui.Ctrl, ui.KeyE).Chosen() {
		a.later(a.enableSelected)
	}
	if m.Item(locales.Disable()).Disabled(!anyEnabled).
		Shortcut(ui.Ctrl, ui.KeyI).Chosen() {
		a.later(a.disableSelected)
	}
	m.Separator()
	m.Submenu(locales.MoveToGroup(), func(m *ui.Menu) {
		if m.Item(locales.Ungrouped()).Chosen() {
			a.later(func() { a.moveToGroup("") })
		}
		for _, g := range a.sideGroups {
			name := g
			if m.Item(name).Chosen() {
				a.later(func() { a.moveToGroup(name) })
			}
		}
		m.Separator()
		if m.Item(locales.NewGroup()).Chosen() {
			a.later(a.openNewGroup)
		}
	}).Disabled(!anySel)
	m.Separator()
	if m.Item(locales.Refresh()).Shortcut(0, ui.KeyF5).Chosen() {
		a.later(a.doRefresh)
	}
	if m.Item(locales.FlushDNS()).Chosen() {
		a.later(a.flushDNS)
	}
	m.Separator()
	if m.Item(locales.NewRuleItem()).Shortcut(ui.Cmd, ui.KeyN).Chosen() {
		a.later(a.openNewProxy)
	}
	if m.Item(locales.ModifyItem()).Disabled(!oneSel).
		Shortcut(ui.Cmd, ui.KeyM).Chosen() {
		a.later(a.openModifyProxy)
	}
	if m.Item(locales.Delete()).Disabled(!anySel).
		Shortcut(0, ui.KeyDelete).Chosen() {
		a.later(a.deleteSelected)
	}
	m.Separator()
	m.Submenu(locales.More(), a.buildMoreMenu)
	m.Separator()
	if m.Item(locales.AboutItem()).Chosen() {
		a.about = true
	}
}

func (a *app) buildLanguageMenu(m *ui.Menu) {
	if m.Item(locales.FollowSystem()).Checked(a.langOverride == "").Chosen() {
		a.later(func() { a.setLanguage("") })
	}
	m.Separator()
	for _, tag := range mygo.App.Languages() {
		if i18n.IsPseudo(tag) {
			continue
		}
		label := i18n.Autonym(tag)
		if m.Item(label).Checked(a.langOverride == tag).Chosen() {
			a.later(func() { a.setLanguage(tag) })
		}
	}
}

func (a *app) buildMoreMenu(m *ui.Menu) {
	if m.Item(locales.NewGroup()).Chosen() {
		a.later(a.openNewGroup)
	}
	if a.filterGroup != filterAll && a.filterGroup != filterUngrouped {
		if m.Item(locales.RenameGroup()).Chosen() {
			a.later(func() { a.openRenameGroup(a.filterGroup) })
		}
		if m.Item(locales.DeleteGroup()).Chosen() {
			a.later(func() { a.openDeleteGroup(a.filterGroup) })
		}
	}
	m.Separator()
	if m.Item(locales.ImportConfig()).Chosen() {
		a.later(a.importDB)
	}
	if m.Item(locales.ExportConfig()).Chosen() {
		a.later(a.exportDB)
	}
	m.Separator()
	if m.Item(locales.ResetWindow()).Chosen() {
		a.later(a.resetWindow)
	}
	m.Separator()
	m.Submenu(locales.Language(), a.buildLanguageMenu)
	m.Separator()
	if m.Item(locales.CheckForUpdates()).Chosen() {
		updater.CheckForUpdates()
	}
	if m.Item(locales.AboutItem()).Chosen() {
		a.about = true
	}
}

func (a *app) shortcuts(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		a.later(a.openNewProxy)
	}
	if c.Shortcut(ui.Cmd, ui.KeyG) {
		a.later(a.openNewGroup)
	}
	if c.Shortcut(0, ui.KeyF5) || c.Shortcut(ui.Cmd, ui.KeyR) {
		a.later(a.doRefresh)
	}
	if c.Shortcut(0, ui.KeyDelete) && !a.set.open && !a.about &&
		!a.group.open && !a.confirmOpen && !a.deleteConfirm.open {
		a.later(a.deleteSelected)
	}
}
