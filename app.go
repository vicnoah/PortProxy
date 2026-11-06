package main

import (
	"fmt"
	"net"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"portproxy/internal/data"
	"portproxy/internal/win"
	"portproxy/locales"
)

const (
	appTitle        = "PortProxy"
	appVersion      = "1.4.2"
	filterAll       = ""
	filterUngrouped = "\x00ungrouped"
)

var columnIDs = []string{
	"status", "type", "listenOn", "listenPort", "connectTo", "connectPort", "comment",
}

var pageSizes = []int{10, 20, 50, 100}

const defaultPageSize = 20

func typeFilterOptions() []string {
	return append([]string{locales.FilterAllTypes()}, data.ProxyTypes...)
}

func typeOptions() []string {
	return append([]string{locales.AutoType()}, data.ProxyTypes...)
}

func columnTitles() []string {
	return []string{
		"", locales.ColType(), locales.ColListenOn(), locales.ColListenPort(),
		locales.ColConnectTo(), locales.ColConnectPort(), locales.ColComment(),
	}
}

// Platform is the Windows side of PortProxy. It is an interface so the view
// and the sync logic can be tested without a Windows registry.
type Platform interface {
	GetProxies() ([]win.Proxy, error)
	AddOrUpdateProxy(win.Proxy) error
	DeleteProxy(win.Proxy) error
	IsServiceRunning() (bool, error)
	StartService() error
	ParamChange() error
	FlushDNSCache() error
}

type osPlatform struct{}

func (osPlatform) GetProxies() ([]win.Proxy, error)   { return win.GetProxies() }
func (osPlatform) AddOrUpdateProxy(p win.Proxy) error { return win.AddOrUpdateProxy(p) }
func (osPlatform) DeleteProxy(p win.Proxy) error      { return win.DeleteProxy(p) }
func (osPlatform) IsServiceRunning() (bool, error)    { return win.IsServiceRunning() }
func (osPlatform) StartService() error                { return win.StartService() }
func (osPlatform) ParamChange() error                 { return win.ParamChange() }
func (osPlatform) FlushDNSCache() error               { return win.FlushDNSCache() }

type row struct {
	key      string
	header   bool
	title    string
	count    int
	hasRule  bool
	rule     data.Rule
	active   bool
	rowIndex int
}

type setProxyForm struct {
	open        bool
	update      bool
	original    data.Rule
	selectedID  string
	typ         string
	group       string
	listenOn    string
	listenPort  string
	connectTo   string
	connectPort string
	comment     string
	err         string
}

type groupForm struct {
	open    bool
	rename  bool
	oldName string
	name    string
	err     string
}

type deleteConfirmForm struct {
	open   bool
	ids    []string
	titles []string
}

type app struct {
	store *data.Store
	p     Platform
	cfg   data.AppConfig

	rules  []data.Rule
	active map[string]bool
	rows   []row
	dirty  bool

	filterGroup  string
	query        string
	groups       []string
	statusIndex  int
	typeChoice   string
	page         int
	pageSize     int
	appliedSig   string
	pending      []func()
	groupCounts  map[string]int
	sideGroups   []string
	totalRules   int
	activeRules  int
	ungrouped    int
	visibleRules int
	groupTotal   int

	list      ui.ListState
	selection ui.Selection[string]
	selected  int
	sort      ui.SortOrder

	status         string
	serviceRunning bool
	serviceKnown   bool
	langOverride   string

	set           setProxyForm
	about         bool
	group         groupForm
	confirmOpen   bool
	confirmName   string
	deleteConfirm deleteConfirmForm

	toastMsg  string
	toastKind string

	designCache *design
	designDark  bool

	win    *mygo.Window
	update func(func())
	async  func(func())
}

func newApp(store *data.Store, p Platform) (*app, error) {
	cfg, err := store.DB().GetAppConfig()
	if err != nil {
		return nil, err
	}
	a := &app{
		store:          store,
		p:              p,
		cfg:            cfg,
		active:         map[string]bool{},
		statusIndex:    0,
		page:           0,
		pageSize:       data.LoadPageSize(),
		groupCounts:    map[string]int{},
		selected:       -1,
		dirty:          true,
		serviceRunning: true,
		langOverride:   data.LoadLanguagePref(),
		groups:         data.LoadCustomGroups(),
		update:         func(fn func()) { fn() },
		async:          func(fn func()) { fn() },
	}
	if len(cfg.ColumnWidths) != len(columnIDs) {
		a.cfg.ColumnWidths = append([]int(nil), data.DefaultColumnWidths...)
	}
	a.list.Selected = &a.selected
	a.list.Selection = &a.selection
	a.list.Sort = &a.sort
	a.list.RoundRows = true
	a.applyColumnWidths()
	return a, nil
}

func keyOf(typ, listenOn string, port int) string {
	return typ + "\x00" + listenOn + "\x00" + strconv.Itoa(port)
}

func ruleKey(r data.Rule) string { return keyOf(r.Type, r.ListenOn, r.ListenPort) }

func proxyToRule(p win.Proxy) data.Rule {
	return data.Rule{
		Type: p.Type, ListenOn: p.ListenOn, ListenPort: p.ListenPort,
		ConnectTo: p.ConnectTo, ConnectPort: p.ConnectPort,
	}
}

func ruleToProxy(r data.Rule) win.Proxy {
	return win.Proxy{
		Type: r.Type, ListenOn: r.ListenOn, ListenPort: r.ListenPort,
		ConnectTo: r.ConnectTo, ConnectPort: r.ConnectPort,
	}
}

// isIPv6 mirrors PortProxyGUI's SetProxy.IsIPv6 regex, extended with a
// net.ParseIP fallback so ordinary IPv6 literals such as "::1" are detected
// too. Both still yield one of the four valid netsh proxy types.
var ipv6RE = regexp.MustCompile(`^[0-9A-F]{2}(?::[0-9A-F]{2}){5}$`)

func isIPv6(s string) bool {
	if ipv6RE.MatchString(s) {
		return true
	}
	return strings.Contains(s, ":") && net.ParseIP(s) != nil
}

func getPassType(listenOn, connectTo string) string {
	from, to := "v4", "v4"
	if isIPv6(listenOn) {
		from = "v6"
	}
	if isIPv6(connectTo) {
		to = "v6"
	}
	return from + "to" + to
}

func (a *app) normalizeFilters() {
	opts := typeFilterOptions()
	if !slices.Contains(opts, a.typeChoice) {
		a.typeChoice = opts[0]
	}
	if !slices.Contains(pageSizes, a.pageSize) {
		a.pageSize = defaultPageSize
	}
}

func (a *app) currentTypeFilter() string {
	for _, t := range data.ProxyTypes {
		if a.typeChoice == t {
			return t
		}
	}
	return ""
}

func (a *app) hasFilters() bool {
	return a.filterGroup != filterAll || a.query != "" ||
		a.statusIndex != 0 || a.currentTypeFilter() != ""
}

// filterSig is the O(1) signature of everything that changes which rows
// are shown. Comparing it keeps per-frame work independent of the number of
// rules.
func (a *app) filterSig() string {
	return a.filterGroup + "\x00" + a.query + "\x00" +
		strconv.Itoa(a.statusIndex) + "\x00" + a.currentTypeFilter() + "\x00" +
		strconv.Itoa(a.page) + "\x00" + strconv.Itoa(a.pageSize) + "\x00" +
		a.sort.Column + "\x00" + strconv.FormatBool(a.sort.Descending)
}

func (a *app) pageCount(n int) int {
	if a.pageSize <= 0 {
		return 1
	}
	return max(1, (n+a.pageSize-1)/a.pageSize)
}

// later defers an action that changes the row collection until the view has
// finished building. Rebuilding a.rows while ui.Table is still iterating the
// previous row count would index past the end and crash.
func (a *app) later(fn func()) {
	a.pending = append(a.pending, fn)
}

func (a *app) runPending(c *ui.Context) {
	if len(a.pending) == 0 {
		return
	}
	fns := a.pending
	a.pending = nil
	for _, fn := range fns {
		fn()
	}
	c.Invalidate()
}

func (a *app) clearFilters() {
	a.filterGroup, a.query, a.statusIndex = filterAll, "", 0
	a.typeChoice = typeFilterOptions()[0]
	a.page = 0
	a.clearSelection()
	a.syncRows()
}

func (a *app) setPageSize(n int) {
	a.pageSize = n
	a.page = 0
	if err := data.SavePageSize(n); err != nil {
		a.reportError(err)
	}
	a.syncRows()
}

func (a *app) gotoPage(p int) {
	a.page = p
	a.syncRows()
}

func (a *app) applyTheme(c *ui.Context) *design {
	sys := c.Theme()
	dark := sys != nil && sys.Dark
	if a.designCache == nil || a.designDark != dark {
		a.designCache = designFor(dark)
		a.designDark = dark
	}
	c.SetTheme(a.designCache.t)
	return a.designCache
}

// The reference app's default column widths are sized for a 9pt font. Our
// 13px interface needs a little more room so headers and type tags do not
// clip; larger values the user (or the reference app) chose are preserved.
var minColumnWidths = []int{64, 90, 140, 120, 140, 120, 120}

func (a *app) displayWidths() []int {
	stored := a.cfg.ColumnWidths
	if len(stored) != len(columnIDs) {
		stored = data.DefaultColumnWidths
	}
	out := make([]int, len(columnIDs))
	for i := range out {
		out[i] = max(stored[i], minColumnWidths[i])
	}
	return out
}

func (a *app) applyColumnWidths() {
	if len(a.cfg.ColumnWidths) != len(columnIDs) {
		a.cfg.ColumnWidths = append([]int(nil), data.DefaultColumnWidths...)
	}
	widths := a.displayWidths()
	m := make(map[string]float32, len(columnIDs)-1)
	// The last column is left flexible so the table fills the card.
	for i, id := range columnIDs[:len(columnIDs)-1] {
		m[id] = float32(widths[i])
	}
	a.list.Columns = ui.TableLayout{
		Order:  append([]string(nil), columnIDs...),
		Widths: m,
	}
}

func (a *app) readColumnWidths() []int {
	out := append([]int(nil), a.cfg.ColumnWidths...)
	for i, id := range columnIDs {
		if w, ok := a.list.Columns.Widths[id]; ok && w > 0 {
			out[i] = int(w)
		}
	}
	return out
}

func (a *app) saveConfig() {
	a.cfg.ColumnWidths = a.readColumnWidths()
	if err := a.store.DB().SaveAppConfig(a.cfg); err != nil {
		a.reportError(err)
	}
}

func (a *app) reportError(err error) {
	if err == nil {
		return
	}
	a.status = err.Error()
	a.toastMsg = err.Error()
	a.toastKind = "error"
}

func (a *app) setStatus(msg string) {
	a.status = time.Now().Format("2006-01-02 15:04:05") + " : " + msg
}

func (a *app) notify(msg string) {
	a.toastMsg = msg
	a.toastKind = ""
}

// refresh mirrors PortProxyGUI.PortProxyGUI.RefreshProxyList: the registry
// is the source of truth for active proxies, config.db keeps the metadata.
func (a *app) refresh() {
	proxies, err := a.p.GetProxies()
	if err != nil {
		a.reportError(err)
		return
	}
	rules, err := a.store.DB().Rules()
	if err != nil {
		a.reportError(err)
		return
	}

	type match struct {
		win.Proxy
		id string
	}
	matched := make([]match, 0, len(proxies))
	active := make(map[string]bool, len(proxies))
	for _, p := range proxies {
		id := ""
		for j := range rules {
			if keyOf(p.Type, p.ListenOn, p.ListenPort) == ruleKey(rules[j]) {
				id = rules[j].Id
				break
			}
		}
		matched = append(matched, match{Proxy: p, id: id})
		active[keyOf(p.Type, p.ListenOn, p.ListenPort)] = true
	}

	for _, m := range matched {
		if m.ListenPort <= 0 || m.ConnectPort <= 0 || m.id != "" {
			continue
		}
		r := proxyToRule(m.Proxy)
		if err := a.store.DB().Add(&r); err != nil {
			a.reportError(err)
			return
		}
	}
	for _, m := range matched {
		if m.ListenPort <= 0 || m.ConnectPort <= 0 || m.id == "" {
			continue
		}
		for j := range rules {
			if rules[j].Id != m.id {
				continue
			}
			r := rules[j]
			r.Type, r.ListenOn, r.ListenPort = m.Type, m.ListenOn, m.ListenPort
			r.ConnectTo, r.ConnectPort = m.ConnectTo, m.ConnectPort
			if err := a.store.DB().Update(r); err != nil {
				a.reportError(err)
				return
			}
			break
		}
	}

	a.rules, err = a.store.DB().Rules()
	if err != nil {
		a.reportError(err)
		return
	}
	a.active = active
	a.dirty = true
	a.syncRows()

	if running, err := a.p.IsServiceRunning(); err == nil {
		a.serviceRunning, a.serviceKnown = running, true
	} else {
		a.serviceRunning, a.serviceKnown = true, false
	}
}

func cmpInt(x, y int) int {
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

func (a *app) comparator() func(x, y data.Rule) bool {
	col, desc := a.sort.Column, a.sort.Descending
	return func(x, y data.Rule) bool {
		var c int
		switch col {
		case "status":
			ax, ay := a.active[ruleKey(x)], a.active[ruleKey(y)]
			if ax == ay {
				c = 0
			} else if ax {
				c = -1
			} else {
				c = 1
			}
		case "type":
			c = strings.Compare(strings.ToLower(x.Type), strings.ToLower(y.Type))
		case "listenOn":
			c = strings.Compare(strings.ToLower(x.ListenOn), strings.ToLower(y.ListenOn))
		case "listenPort":
			c = cmpInt(x.ListenPort, y.ListenPort)
		case "connectTo":
			c = strings.Compare(strings.ToLower(x.ConnectTo), strings.ToLower(y.ConnectTo))
		case "connectPort":
			c = cmpInt(x.ConnectPort, y.ConnectPort)
		case "comment":
			c = strings.Compare(strings.ToLower(x.Comment), strings.ToLower(y.Comment))
		default:
			return false
		}
		if desc {
			return c > 0
		}
		return c < 0
	}
}

func (a *app) matchesFilter(r data.Rule) bool {
	g := strings.TrimSpace(r.Group)
	switch a.filterGroup {
	case filterAll:
	case filterUngrouped:
		if g != "" {
			return false
		}
	default:
		if g != a.filterGroup {
			return false
		}
	}
	on := a.active[ruleKey(r)]
	if a.statusIndex == 1 && !on {
		return false
	}
	if a.statusIndex == 2 && on {
		return false
	}
	if t := a.currentTypeFilter(); t != "" && r.Type != t {
		return false
	}
	return true
}

func (a *app) matchesQuery(r data.Rule, q string) bool {
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(r.Type), q) ||
		strings.Contains(strings.ToLower(r.ListenOn), q) ||
		strings.Contains(strconv.Itoa(r.ListenPort), q) ||
		strings.Contains(strings.ToLower(r.ConnectTo), q) ||
		strings.Contains(strconv.Itoa(r.ConnectPort), q) ||
		strings.Contains(strings.ToLower(r.Comment), q) ||
		strings.Contains(strings.ToLower(r.Group), q)
}

func (a *app) rebuildRows() {
	groupCounts := map[string]int{}
	active := 0
	ungrouped := 0
	for i := range a.rules {
		g := strings.TrimSpace(a.rules[i].Group)
		if g == "" {
			ungrouped++
		} else {
			groupCounts[g]++
		}
		if a.active[ruleKey(a.rules[i])] {
			active++
		}
	}
	a.groupCounts = groupCounts
	a.sideGroups = a.groupNames()
	a.groupTotal = len(a.sideGroups)
	a.totalRules = len(a.rules)
	a.activeRules = active
	a.ungrouped = ungrouped

	q := strings.ToLower(strings.TrimSpace(a.query))
	grouped := map[string][]int{}
	var plain []int
	for i := range a.rules {
		if !a.matchesFilter(a.rules[i]) || !a.matchesQuery(a.rules[i], q) {
			continue
		}
		g := strings.TrimSpace(a.rules[i].Group)
		if g == "" {
			plain = append(plain, i)
		} else {
			grouped[g] = append(grouped[g], i)
		}
	}

	names := make([]string, 0, len(grouped))
	for n := range grouped {
		names = append(names, n)
	}
	sort.Strings(names)
	less := a.comparator()

	// All matching rule rows, ordered by group then sort column, without
	// group headers. Pagination slices this, then headers are inserted.
	view := make([]row, 0, len(a.rules))
	addRules := func(idxs []int) {
		sort.SliceStable(idxs, func(i, j int) bool {
			return less(a.rules[idxs[i]], a.rules[idxs[j]])
		})
		for _, i := range idxs {
			r := a.rules[i]
			view = append(view, row{
				key: r.Id, hasRule: true, rule: r,
				active: a.active[ruleKey(r)], rowIndex: i,
			})
		}
	}
	for _, n := range names {
		addRules(grouped[n])
	}
	addRules(plain)
	a.visibleRules = len(view)

	pages := a.pageCount(len(view))
	if a.page >= pages {
		a.page = pages - 1
	}
	if a.page < 0 {
		a.page = 0
	}
	start := a.page * a.pageSize
	end := min(start+a.pageSize, len(view))
	pageRows := view[start:end]

	rows := make([]row, 0, len(pageRows)+len(names))
	showHeaders := !a.hasFilters()
	prevGroup := ""
	for _, rw := range pageRows {
		g := strings.TrimSpace(rw.rule.Group)
		if showHeaders && g != "" && g != prevGroup {
			rows = append(rows, row{
				key: "group:" + rw.key, header: true, title: g,
				count: a.groupCounts[g],
			})
		}
		prevGroup = g
		rows = append(rows, rw)
	}

	a.rows = rows
	a.list.Key = func(i int) any {
		if i < 0 || i >= len(a.rows) {
			return i
		}
		return a.rows[i].key
	}
	a.list.Header = func(i int) bool {
		return i >= 0 && i < len(a.rows) && a.rows[i].header
	}
	a.appliedSig = a.filterSig()
	a.dirty = false
}

func (a *app) syncRows() {
	if a.dirty || a.appliedSig != a.filterSig() {
		a.rebuildRows()
	}
}

func (a *app) setFilter(group string) {
	a.filterGroup = group
	a.page = 0
	a.clearSelection()
	a.syncRows()
}

func (a *app) selectedIDs() []string {
	var ids []string
	for k := range a.selection.All() {
		ids = append(ids, k)
	}
	if len(ids) == 0 && a.selected >= 0 && a.selected < len(a.rows) {
		if r := a.rows[a.selected]; r.hasRule {
			ids = append(ids, r.key)
		}
	}
	sort.Strings(ids)
	return ids
}

func (a *app) ruleByID(id string) *data.Rule {
	for i := range a.rules {
		if a.rules[i].Id == id {
			return &a.rules[i]
		}
	}
	return nil
}

func (a *app) clearSelection() {
	a.selection.Clear()
	a.selected = -1
}

func (a *app) enableSelected() {
	ids := a.selectedIDs()
	n := 0
	for _, id := range ids {
		r := a.ruleByID(id)
		if r == nil || a.active[ruleKey(*r)] {
			continue
		}
		if !r.Valid() {
			a.reportError(fmt.Errorf("invalid rule for (%s -> %s)", r.ListenOn, r.ConnectTo))
			return
		}
		if err := a.p.AddOrUpdateProxy(ruleToProxy(*r)); err != nil {
			a.reportError(err)
			return
		}
		a.active[ruleKey(*r)] = true
		n++
	}
	_ = a.p.ParamChange()
	a.dirty = true
	a.syncRows()
	a.setStatus(locales.EnabledMsg(n))
	a.notify(locales.EnabledMsg(n))
}

func (a *app) disableSelected() {
	ids := a.selectedIDs()
	n := 0
	for _, id := range ids {
		r := a.ruleByID(id)
		if r == nil || !a.active[ruleKey(*r)] {
			continue
		}
		if err := a.p.DeleteProxy(ruleToProxy(*r)); err != nil {
			a.reportError(err)
			return
		}
		delete(a.active, ruleKey(*r))
		n++
	}
	_ = a.p.ParamChange()
	a.dirty = true
	a.syncRows()
	a.setStatus(locales.DisabledMsg(n))
	a.notify(locales.DisabledMsg(n))
}

// askDelete opens the confirmation dialog. Every delete entry point (row
// button, context menu, Delete key) goes through here so a stray click can
// never remove a rule.
func (a *app) askDelete(ids []string) {
	if len(ids) == 0 {
		return
	}
	titles := make([]string, 0, len(ids))
	for _, id := range ids {
		if r := a.ruleByID(id); r != nil {
			titles = append(titles, fmt.Sprintf("%s  %s:%d → %s:%d",
				r.Type, r.ListenOn, r.ListenPort, r.ConnectTo, r.ConnectPort))
		}
	}
	if len(titles) == 0 {
		return
	}
	a.deleteConfirm = deleteConfirmForm{open: true, ids: ids, titles: titles}
}

func (a *app) confirmDelete() {
	ids := a.deleteConfirm.ids
	a.deleteConfirm = deleteConfirmForm{}
	a.deleteIDs(ids)
}

func (a *app) deleteIDs(ids []string) {
	for _, id := range ids {
		if r := a.ruleByID(id); r != nil && a.active[ruleKey(*r)] {
			if err := a.p.DeleteProxy(ruleToProxy(*r)); err != nil {
				a.reportError(err)
			}
			delete(a.active, ruleKey(*r))
		}
	}
	_ = a.p.ParamChange()
	for _, id := range ids {
		if r := a.ruleByID(id); r != nil {
			if err := a.store.DB().Remove(*r); err != nil {
				a.reportError(err)
			}
		}
	}
	a.clearSelection()
	a.refresh()
	a.setStatus(locales.DeletedMsg(len(ids)))
	a.notify(locales.DeletedMsg(len(ids)))
}

func (a *app) doRefresh() {
	a.refresh()
	a.setStatus(locales.RefreshedMsg())
	a.notify(locales.RefreshedMsg())
}

func (a *app) flushDNS() {
	if err := a.p.FlushDNSCache(); err != nil {
		a.reportError(err)
		return
	}
	a.setStatus(locales.DnsMsg())
	a.notify(locales.DnsMsg())
}

func (a *app) startService() {
	if err := a.p.StartService(); err != nil {
		a.reportError(err)
		return
	}
	a.serviceRunning, a.serviceKnown = true, true
	a.setStatus(locales.ServiceMsg())
	a.notify(locales.ServiceMsg())
}

func (a *app) resetWindow() {
	a.cfg = data.DefaultAppConfig()
	if a.win != nil {
		a.win.SetSize(a.cfg.MainWindowWidth, a.cfg.MainWindowHeight)
	}
	a.applyColumnWidths()
	a.dirty = true
	a.syncRows()
	a.saveConfig()
	a.setStatus(locales.ResetMsg())
	a.notify(locales.ResetMsg())
}

func (a *app) setLanguage(tag string) {
	a.langOverride = tag
	if err := data.SaveLanguagePref(tag); err != nil {
		a.reportError(err)
	}
	if a.win != nil {
		mygo.App.SetLocale(tag)
	}
}

func (a *app) exportDB() {
	a.async(func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      a.win,
			Title:       locales.ExportDialogTitle(),
			DefaultPath: "config.db",
			ButtonLabel: locales.ExportButton(),
			Filters: []mygo.FileFilter{
				{Name: locales.DatabaseFile(), Extensions: []string{"db"}},
			},
		})
		if err != nil || path == "" {
			return
		}
		a.update(func() {
			if err := a.store.DB().ExportTo(path); err != nil {
				a.reportError(err)
				return
			}
			a.setStatus(locales.ExportMsg())
			a.notify(locales.ExportMsg())
		})
	})
}

func (a *app) importDB() {
	a.async(func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      a.win,
			Title:       locales.ImportDialogTitle(),
			ButtonLabel: locales.ImportButton(),
			Filters: []mygo.FileFilter{
				{Name: locales.DatabaseFile(), Extensions: []string{"db"}},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.update(func() {
			n, err := a.store.DB().ImportFrom(paths[0])
			if err != nil {
				a.reportError(err)
				return
			}
			a.refresh()
			a.setStatus(locales.ImportedMsg(n))
			a.notify(locales.ImportedMsg(n))
		})
	})
}

func (a *app) openNewProxy() {
	group := ""
	if a.filterGroup != filterAll && a.filterGroup != filterUngrouped {
		group = a.filterGroup
	}
	a.set = setProxyForm{
		open: true, update: false,
		typ: typeOptions()[0], group: group,
		listenOn: "*", listenPort: "", connectTo: "", connectPort: "", comment: "",
	}
}

func (a *app) openModifyProxy() {
	ids := a.selectedIDs()
	if len(ids) != 1 {
		return
	}
	if r := a.ruleByID(ids[0]); r != nil {
		a.openModifyFor(*r)
	}
}

func (a *app) groupNames() []string {
	seen := map[string]bool{}
	for _, g := range a.groups {
		if g != "" {
			seen[g] = true
		}
	}
	for i := range a.rules {
		g := strings.TrimSpace(a.rules[i].Group)
		if g != "" {
			seen[g] = true
		}
	}
	out := make([]string, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

func (a *app) openNewGroup() {
	a.group = groupForm{open: true}
}

func (a *app) openRenameGroup(name string) {
	a.group = groupForm{open: true, rename: true, oldName: name, name: name}
}

func (a *app) openDeleteGroup(name string) {
	a.confirmOpen, a.confirmName = true, name
}

func (a *app) applyGroupForm() {
	f := &a.group
	name := strings.TrimSpace(f.name)
	if name == "" {
		f.err = locales.GroupNameRequired()
		return
	}
	for _, g := range a.groupNames() {
		if g == name && (!f.rename || g != f.oldName) {
			f.err = locales.GroupExists()
			return
		}
	}
	groups := append([]string(nil), a.groups...)
	msg := locales.GroupCreatedMsg()
	if f.rename {
		if err := a.store.DB().RenameGroup(f.oldName, name); err != nil {
			f.err = err.Error()
			return
		}
		for i := range groups {
			if groups[i] == f.oldName {
				groups[i] = name
			}
		}
		if a.filterGroup == f.oldName {
			a.filterGroup = name
		}
		msg = locales.GroupRenamedMsg()
	} else {
		groups = append(groups, name)
	}
	if err := data.SaveCustomGroups(groups); err != nil {
		f.err = err.Error()
		return
	}
	a.groups = groups
	f.open, f.err = false, ""
	a.refresh()
	a.setStatus(msg)
	a.notify(msg)
}

func (a *app) deleteGroup(name string) {
	if err := a.store.DB().ClearGroup(name); err != nil {
		a.reportError(err)
		return
	}
	groups := make([]string, 0, len(a.groups))
	for _, g := range a.groups {
		if g != name {
			groups = append(groups, g)
		}
	}
	if err := data.SaveCustomGroups(groups); err != nil {
		a.reportError(err)
	}
	a.groups = groups
	a.confirmOpen, a.confirmName = false, ""
	if a.filterGroup == name {
		a.filterGroup = filterAll
	}
	a.refresh()
	a.setStatus(locales.GroupDeletedMsg())
	a.notify(locales.GroupDeletedMsg())
}

func (a *app) moveToGroup(group string) {
	for _, id := range a.selectedIDs() {
		if err := a.store.DB().SetRuleGroup(id, group); err != nil {
			a.reportError(err)
		}
	}
	a.clearSelection()
	a.refresh()
}

func (a *app) toggleRule(r data.Rule, on bool) {
	if on {
		if !r.Valid() {
			a.reportError(fmt.Errorf("invalid rule for (%s -> %s)", r.ListenOn, r.ConnectTo))
			return
		}
		if err := a.p.AddOrUpdateProxy(ruleToProxy(r)); err != nil {
			a.reportError(err)
			return
		}
		a.active[ruleKey(r)] = true
	} else {
		if err := a.p.DeleteProxy(ruleToProxy(r)); err != nil {
			a.reportError(err)
			return
		}
		delete(a.active, ruleKey(r))
	}
	_ = a.p.ParamChange()
	a.dirty = true
	a.syncRows()
}

func (a *app) openModifyFor(r data.Rule) {
	a.set = setProxyForm{
		open: true, update: true,
		original: r, selectedID: r.Id,
		typ: r.Type, group: r.Group,
		listenOn: r.ListenOn, listenPort: strconv.Itoa(r.ListenPort),
		connectTo: r.ConnectTo, connectPort: strconv.Itoa(r.ConnectPort),
		comment: r.Comment,
	}
}

func (a *app) deleteRule(r data.Rule) {
	a.askDelete([]string{r.Id})
}

func (a *app) deleteSelected() {
	a.askDelete(a.selectedIDs())
}

func (a *app) applySetProxy() {
	f := &a.set
	lp, err := data.ParsePort(f.listenPort)
	if err != nil {
		f.err = err.Error()
		return
	}
	cp, err := data.ParsePort(f.connectPort)
	if err != nil {
		f.err = err.Error()
		return
	}
	r := data.Rule{
		Type:        strings.TrimSpace(f.typ),
		ListenOn:    strings.TrimSpace(f.listenOn),
		ListenPort:  lp,
		ConnectTo:   strings.TrimSpace(f.connectTo),
		ConnectPort: cp,
		Comment:     strings.TrimSpace(f.comment),
		Group:       strings.TrimSpace(f.group),
	}
	if r.Type == locales.AutoType() {
		r.Type = getPassType(r.ListenOn, r.ConnectTo)
	}
	if !data.IsProxyType(r.Type) {
		f.err = fmt.Sprintf("Unknow type for (%s -> %s).", r.ListenOn, r.ConnectTo)
		return
	}
	if f.update {
		old, err := a.store.DB().
			GetRule(f.original.Type, f.original.ListenOn, f.original.ListenPort)
		if err != nil {
			f.err = err.Error()
			return
		}
		if old != nil {
			if err := a.p.DeleteProxy(ruleToProxy(*old)); err != nil {
				f.err = err.Error()
				return
			}
			if err := a.store.DB().Remove(*old); err != nil {
				f.err = err.Error()
				return
			}
		}
	}
	if err := a.p.AddOrUpdateProxy(ruleToProxy(r)); err != nil {
		f.err = err.Error()
		return
	}
	if err := a.store.DB().Add(&r); err != nil {
		f.err = err.Error()
		return
	}
	if r.Group != "" {
		known := false
		for _, g := range a.groups {
			if g == r.Group {
				known = true
				break
			}
		}
		if !known {
			a.groups = append(a.groups, r.Group)
			if err := data.SaveCustomGroups(a.groups); err != nil {
				a.reportError(err)
			}
		}
	}
	_ = a.p.ParamChange()
	a.set.open = false
	a.set.err = ""
	a.refresh()
	a.setStatus(locales.SavedMsg())
	a.notify(locales.SavedMsg())
}
