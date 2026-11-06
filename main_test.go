package main

import (
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"portproxy/internal/data"
	"portproxy/internal/win"
	"portproxy/locales"
)

type fakePlatform struct {
	proxies      []win.Proxy
	running      bool
	flushed      bool
	started      bool
	paramChanges int
}

func (f *fakePlatform) GetProxies() ([]win.Proxy, error) { return f.proxies, nil }

func (f *fakePlatform) AddOrUpdateProxy(p win.Proxy) error {
	for i := range f.proxies {
		if f.proxies[i].Type == p.Type &&
			f.proxies[i].ListenOn == p.ListenOn &&
			f.proxies[i].ListenPort == p.ListenPort {
			f.proxies[i] = p
			return nil
		}
	}
	f.proxies = append(f.proxies, p)
	return nil
}

func (f *fakePlatform) DeleteProxy(p win.Proxy) error {
	out := f.proxies[:0]
	for _, x := range f.proxies {
		if x.Type == p.Type && x.ListenOn == p.ListenOn && x.ListenPort == p.ListenPort {
			continue
		}
		out = append(out, x)
	}
	f.proxies = out
	return nil
}

func (f *fakePlatform) IsServiceRunning() (bool, error) { return f.running, nil }
func (f *fakePlatform) StartService() error {
	f.started, f.running = true, true
	return nil
}
func (f *fakePlatform) ParamChange() error   { f.paramChanges++; return nil }
func (f *fakePlatform) FlushDNSCache() error { f.flushed = true; return nil }

func newTestApp(t *testing.T) (*app, *fakePlatform, *data.Store) {
	t.Helper()
	dir := t.TempDir()
	// Never let a test touch the real user preferences directory.
	t.Setenv("PORTPROXY_PREFS_DIR", filepath.Join(dir, "prefs"))
	store, err := data.OpenStore(filepath.Join(dir, "own", "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	p := &fakePlatform{running: true}
	a, err := newApp(store, p)
	if err != nil {
		t.Fatal(err)
	}
	a.async = func(func()) {} // never open a native dialog in tests
	a.update = func(fn func()) { fn() }
	return a, p, store
}

func TestViewRenders(t *testing.T) {
	a, _, _ := newTestApp(t)
	tt := ui.NewTester(a.view, 940, 640)
	tt.Frame()
	for _, want := range []string{
		"PortProxy", "Port forwarding manager", "No forwarding rules",
		"New rule", "Refresh", "Flush DNS Cache",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestRefreshImportsRegistryProxies(t *testing.T) {
	a, p, store := newTestApp(t)
	p.proxies = []win.Proxy{
		{Type: "v4tov4", ListenOn: "0.0.0.0", ListenPort: 8080,
			ConnectTo: "127.0.0.1", ConnectPort: 80},
	}
	a.refresh()
	if len(a.rules) != 1 {
		t.Fatalf("rules = %+v", a.rules)
	}
	r := a.rules[0]
	if r.Type != "v4tov4" || r.ListenPort != 8080 || r.ConnectPort != 80 {
		t.Errorf("rule = %+v", r)
	}
	if !a.active[ruleKey(r)] {
		t.Error("rule should be active")
	}
	rules, _ := store.DB().Rules()
	if len(rules) != 1 || rules[0].Id != r.Id {
		t.Errorf("db rules = %+v", rules)
	}
	tt := ui.NewTester(a.view, 940, 640)
	tt.Frame()
	if !tt.HasText("v4tov4") || !tt.HasText("8080") {
		t.Errorf("texts = %q", tt.Texts())
	}
}

func TestEnableDisableDelete(t *testing.T) {
	a, p, store := newTestApp(t)
	r := data.Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
		ConnectTo: "10.0.0.1", ConnectPort: 4000, Comment: "c", Group: "G"}
	if err := store.DB().Add(&r); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	a.selection.Add(r.Id)
	a.selected = 0

	a.enableSelected()
	if len(p.proxies) != 1 || p.proxies[0].ListenPort != 3000 {
		t.Errorf("proxies after enable = %+v", p.proxies)
	}
	if !a.active[ruleKey(r)] {
		t.Error("should be active")
	}
	if p.paramChanges == 0 {
		t.Error("ParamChange was not called")
	}

	a.disableSelected()
	if len(p.proxies) != 0 {
		t.Errorf("proxies after disable = %+v", p.proxies)
	}

	a.selection.Add(r.Id)
	a.selected = 0
	a.deleteSelected()
	if !a.deleteConfirm.open {
		t.Fatal("delete must ask for confirmation first")
	}
	if rules, _ := store.DB().Rules(); len(rules) != 1 {
		t.Errorf("rule was deleted before confirmation: %+v", rules)
	}
	a.confirmDelete()
	if rules, _ := store.DB().Rules(); len(rules) != 0 {
		t.Errorf("rules after delete = %+v", rules)
	}
}

func TestNewAndModifyProxy(t *testing.T) {
	a, p, store := newTestApp(t)

	a.openNewProxy()
	if a.set.listenOn != "*" || a.set.typ != locales.AutoType() {
		t.Fatalf("new form = %+v", a.set)
	}
	a.set.listenOn = "0.0.0.0"
	a.set.listenPort = "8080"
	a.set.connectTo = "127.0.0.1"
	a.set.connectPort = "80"
	a.set.comment = "web"
	a.set.group = "Servers"
	a.applySetProxy()
	if a.set.open {
		t.Fatalf("dialog stayed open: %s", a.set.err)
	}
	if len(p.proxies) != 1 {
		t.Fatalf("proxies = %+v", p.proxies)
	}
	rules, _ := store.DB().Rules()
	if len(rules) != 1 || rules[0].Comment != "web" || rules[0].Group != "Servers" {
		t.Fatalf("rules = %+v", rules)
	}
	if rules[0].Type != "v4tov4" {
		t.Errorf("auto type = %q", rules[0].Type)
	}

	a.selection.Add(rules[0].Id)
	a.selected = 0
	a.openModifyProxy()
	if !a.set.update || a.set.comment != "web" {
		t.Fatalf("modify form = %+v", a.set)
	}
	a.set.connectTo = "127.0.0.2"
	a.set.connectPort = "8081"
	a.set.comment = "web v2"
	a.applySetProxy()
	if a.set.open {
		t.Fatalf("modify failed: %s", a.set.err)
	}
	rules, _ = store.DB().Rules()
	if len(rules) != 1 || rules[0].ConnectTo != "127.0.0.2" ||
		rules[0].ConnectPort != 8081 || rules[0].Comment != "web v2" {
		t.Errorf("rules after modify = %+v", rules)
	}
	if len(p.proxies) != 1 || p.proxies[0].ConnectTo != "127.0.0.2" {
		t.Errorf("proxies after modify = %+v", p.proxies)
	}
}

func TestInvalidPortShowsError(t *testing.T) {
	a, _, _ := newTestApp(t)
	a.openNewProxy()
	a.set.listenPort = "not-a-port"
	a.applySetProxy()
	if !a.set.open || a.set.err == "" {
		t.Fatalf("expected a form error, got %+v", a.set)
	}
}

func TestAutoType(t *testing.T) {
	for _, tc := range []struct{ listen, connect, want string }{
		{"0.0.0.0", "127.0.0.1", "v4tov4"},
		{"::", "127.0.0.1", "v6tov4"},
		{"0.0.0.0", "::1", "v4tov6"},
		{"::1", "::1", "v6tov6"},
		{"AA:BB:CC:DD:EE:FF", "10.0.0.1", "v6tov4"},
	} {
		if got := getPassType(tc.listen, tc.connect); got != tc.want {
			t.Errorf("getPassType(%q,%q) = %q, want %q", tc.listen, tc.connect, got, tc.want)
		}
	}
}

func TestGroupingAndSorting(t *testing.T) {
	a, _, store := newTestApp(t)
	for _, r := range []data.Rule{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
			ConnectTo: "a", ConnectPort: 1, Group: "Beta"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000,
			ConnectTo: "a", ConnectPort: 1, Group: "Alpha"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 2000,
			ConnectTo: "a", ConnectPort: 1},
	} {
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	a.refresh()
	a.rebuildRows()
	var headers []string
	for _, r := range a.rows {
		if r.header {
			headers = append(headers, r.title)
		}
	}
	if !reflect.DeepEqual(headers, []string{"Alpha", "Beta"}) {
		t.Errorf("group headers = %v", headers)
	}
	a.sort = ui.SortOrder{Column: "listenPort"}
	a.syncRows()
	var ports []int
	for _, r := range a.rows {
		if r.hasRule {
			ports = append(ports, r.rule.ListenPort)
		}
	}
	if !reflect.DeepEqual(ports, []int{1000, 3000, 2000}) {
		t.Errorf("sorted ports = %v", ports)
	}
}

func TestServiceAndFlush(t *testing.T) {
	a, p, _ := newTestApp(t)
	p.running = false
	a.refresh()
	if a.serviceRunning {
		t.Error("service warning should show")
	}
	a.startService()
	if !p.started || !a.serviceRunning {
		t.Error("service not started")
	}
	a.flushDNS()
	if !p.flushed {
		t.Error("DNS cache not flushed")
	}
}

func TestResetWindow(t *testing.T) {
	a, _, store := newTestApp(t)
	a.cfg.MainWindowWidth, a.cfg.MainWindowHeight = 1234, 567
	a.cfg.ColumnWidths = []int{1, 2, 3, 4, 5, 6, 7}
	a.resetWindow()
	cfg, _ := store.DB().GetAppConfig()
	if cfg.MainWindowWidth != 720 || cfg.MainWindowHeight != 500 {
		t.Errorf("config = %+v", cfg)
	}
}

func TestDeleteFromMenuAndKeyNoCrash(t *testing.T) {
	a, _, store := newTestApp(t)
	var ids []string
	for i := 0; i < 3; i++ {
		r := data.Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 1000 + i,
			ConnectTo: "a", ConnectPort: 1, Comment: "row"}
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.Id)
	}
	a.refresh()
	tt := ui.NewTester(a.view, 1280, 800)
	tt.Frame()

	a.selection.Add(ids[0])
	if err := tt.RightClick("v4tov4"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.deleteConfirm.open {
		t.Fatal("context-menu delete must ask for confirmation")
	}
	a.confirmDelete()
	tt.Frame()

	a.selection.Add(ids[1])
	tt.Key(0, ui.KeyDelete)
	tt.Frame()
	if !a.deleteConfirm.open {
		t.Fatal("Delete key must ask for confirmation")
	}
	a.confirmDelete()
	tt.Frame()

	rules, _ := store.DB().Rules()
	if len(rules) != 1 {
		t.Errorf("rules after two deletes = %+v", rules)
	}
}

func TestActionsColumnIsFixed(t *testing.T) {
	a, _, _ := newTestApp(t)
	if len(columnIDs) != 7 {
		t.Fatalf("columnIDs must stay at 7 for config.db compatibility: %v", columnIDs)
	}
	cols := a.tableColumns()
	if len(cols) != len(columnIDs)+1 {
		t.Fatalf("columns = %d, want %d", len(cols), len(columnIDs)+1)
	}
	last := cols[len(cols)-1]
	if last.ID != "actions" || !last.Fixed || last.Width != 78 {
		t.Errorf("actions column = %+v", last)
	}
	if cols[len(columnIDs)-1].ID != "comment" || cols[len(columnIDs)-1].Width != 0 {
		t.Errorf("comment column = %+v", cols[len(columnIDs)-1])
	}
}

func TestPagination(t *testing.T) {
	a, _, store := newTestApp(t)
	for i := 0; i < 25; i++ {
		r := data.Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 1000 + i,
			ConnectTo: "a", ConnectPort: 1}
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	a.refresh()
	a.setPageSize(10)
	if a.visibleRules != 25 || a.pageCount(25) != 3 {
		t.Fatalf("visible=%d pages=%d", a.visibleRules, a.pageCount(25))
	}
	if n := a.ruleRowCount(); n != 10 {
		t.Errorf("page 0 rows = %d, want 10", n)
	}
	a.gotoPage(1)
	if n := a.ruleRowCount(); n != 10 {
		t.Errorf("page 1 rows = %d, want 10", n)
	}
	a.gotoPage(2)
	if n := a.ruleRowCount(); n != 5 {
		t.Errorf("page 2 rows = %d, want 5", n)
	}
	a.setPageSize(50)
	if a.page != 0 || a.ruleRowCount() != 25 {
		t.Errorf("after 50/page: page=%d rows=%d", a.page, a.ruleRowCount())
	}
	a.setPageSize(10)
	a.gotoPage(9)
	if a.page != 2 {
		t.Errorf("page not clamped to last page: %d", a.page)
	}
}

func (a *app) ruleRowCount() int {
	n := 0
	for _, r := range a.rows {
		if r.hasRule {
			n++
		}
	}
	return n
}

func TestFilters(t *testing.T) {
	a, p, store := newTestApp(t)
	for _, r := range []data.Rule{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000, ConnectTo: "a", ConnectPort: 1},
		{Type: "v6tov4", ListenOn: "::", ListenPort: 2000, ConnectTo: "b", ConnectPort: 2},
	} {
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	p.proxies = []win.Proxy{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000,
			ConnectTo: "a", ConnectPort: 1},
	}
	a.refresh()
	if a.visibleRules != 2 || a.hasFilters() {
		t.Fatalf("initial visible=%d filters=%v", a.visibleRules, a.hasFilters())
	}

	a.statusIndex = 1
	a.syncRows()
	if a.visibleRules != 1 {
		t.Errorf("enabled filter visible=%d, want 1", a.visibleRules)
	}
	a.statusIndex = 2
	a.syncRows()
	if a.visibleRules != 1 {
		t.Errorf("disabled filter visible=%d, want 1", a.visibleRules)
	}

	a.statusIndex = 0
	a.typeChoice = "v6tov4"
	a.syncRows()
	if a.visibleRules != 1 || a.rows[0].rule.Type != "v6tov4" {
		t.Errorf("type filter visible=%d rows=%+v", a.visibleRules, a.rows)
	}
	if !a.hasFilters() {
		t.Error("hasFilters should be true")
	}

	a.query = "no-such-match"
	a.syncRows()
	if a.visibleRules != 0 {
		t.Errorf("query filter visible=%d, want 0", a.visibleRules)
	}
	a.clearFilters()
	if a.visibleRules != 2 || a.hasFilters() || a.page != 0 {
		t.Errorf("after clear: visible=%d filters=%v page=%d",
			a.visibleRules, a.hasFilters(), a.page)
	}
}

func TestDeleteRowButtonNoCrash(t *testing.T) {
	a, _, store := newTestApp(t)
	for _, r := range []data.Rule{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000,
			ConnectTo: "a", ConnectPort: 1, Comment: "first"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 2000,
			ConnectTo: "b", ConnectPort: 2, Comment: "a much longer comment here"},
	} {
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	a.refresh()
	tt := ui.NewTester(a.view, 1280, 800)
	tt.Frame()
	if err := tt.Click(locales.Delete()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.deleteConfirm.open {
		t.Fatal("row delete button must ask for confirmation")
	}
	if err := tt.Click(locales.ConfirmDeleteButton()); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	rules, _ := store.DB().Rules()
	if len(rules) != 1 {
		t.Errorf("rules after delete = %+v", rules)
	}
}

func TestGroupManagement(t *testing.T) {
	a, _, store := newTestApp(t)
	for _, r := range []data.Rule{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000,
			ConnectTo: "a", ConnectPort: 1, Group: "Dev"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 2000,
			ConnectTo: "b", ConnectPort: 2, Group: "Dev"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
			ConnectTo: "c", ConnectPort: 3},
	} {
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	a.refresh()

	a.openNewGroup()
	a.group.name = "Production"
	a.applyGroupForm()
	if a.group.open {
		t.Fatalf("group dialog stayed open: %s", a.group.err)
	}
	if !slices.Contains(a.groupNames(), "Production") {
		t.Errorf("groups = %v", a.groupNames())
	}
	if got := data.LoadCustomGroups(); !slices.Contains(got, "Production") {
		t.Errorf("persisted groups = %v", got)
	}

	a.setFilter("Dev")
	if a.visibleRules != 2 {
		t.Errorf("filtered to Dev: visible = %d", a.visibleRules)
	}
	a.setFilter(filterUngrouped)
	if a.visibleRules != 1 {
		t.Errorf("filtered to ungrouped: visible = %d", a.visibleRules)
	}
	a.setFilter(filterAll)

	a.query = "9000"
	a.syncRows()
	if a.visibleRules != 0 {
		t.Errorf("search 9000: visible = %d", a.visibleRules)
	}
	a.query = "3000"
	a.syncRows()
	if a.visibleRules != 1 {
		t.Errorf("search 3000: visible = %d", a.visibleRules)
	}
	a.query = ""
	a.syncRows()

	a.openRenameGroup("Dev")
	a.group.name = "Staging"
	a.applyGroupForm()
	if a.group.open {
		t.Fatalf("rename failed: %s", a.group.err)
	}
	rules, _ := store.DB().Rules()
	staging := 0
	for _, r := range rules {
		if r.Group == "Staging" {
			staging++
		}
	}
	if staging != 2 {
		t.Errorf("rules in Staging = %d", staging)
	}

	a.openDeleteGroup("Staging")
	a.deleteGroup("Staging")
	rules, _ = store.DB().Rules()
	if len(rules) != 3 {
		t.Fatalf("deleting a group removed rules: %+v", rules)
	}
	for _, r := range rules {
		if r.Group == "Staging" {
			t.Errorf("rule still in deleted group: %+v", r)
		}
	}
	if slices.Contains(a.groupNames(), "Staging") {
		t.Errorf("group still listed: %v", a.groupNames())
	}
}

func TestMoveToGroup(t *testing.T) {
	a, _, store := newTestApp(t)
	r := data.Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 1000,
		ConnectTo: "a", ConnectPort: 1}
	if err := store.DB().Add(&r); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	a.selection.Add(r.Id)
	a.moveToGroup("Servers")
	rules, _ := store.DB().Rules()
	if len(rules) != 1 || rules[0].Group != "Servers" {
		t.Errorf("rules after move = %+v", rules)
	}
}

func TestInlineSwitch(t *testing.T) {
	a, p, store := newTestApp(t)
	r := data.Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
		ConnectTo: "10.0.0.1", ConnectPort: 4000}
	if err := store.DB().Add(&r); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	tt := ui.NewTester(a.view, 1280, 800)
	tt.Frame()
	if err := tt.Click(locales.Enable()); err != nil {
		t.Fatal(err)
	}
	if len(p.proxies) != 1 || p.proxies[0].ListenPort != 3000 {
		t.Errorf("proxies after switch = %+v", p.proxies)
	}
	if !a.active[ruleKey(r)] {
		t.Error("rule should be active after switch")
	}
}

func TestI18nCatalogs(t *testing.T) {
	en, zh := locales.In("en"), locales.In("zh-Hans")
	if en.NewRule() != "New rule" || en.ColType() != "Type" {
		t.Errorf("English catalog: %q / %q", en.NewRule(), en.ColType())
	}
	if zh.NewRule() != "新建规则" || zh.ColType() != "类型" ||
		zh.Welcome() != "欢迎使用 PortProxy！" {
		t.Errorf("Chinese catalog: %q / %q / %q", zh.NewRule(), zh.ColType(), zh.Welcome())
	}
	if en.RulesCount(1) != "1 Rule" || en.RulesCount(3) != "3 Rules" {
		t.Errorf("English plural: %q / %q", en.RulesCount(1), en.RulesCount(3))
	}
	if zh.RulesCount(3) != "3 条规则" {
		t.Errorf("Chinese plural: %q", zh.RulesCount(3))
	}
	if en.RulesActive(2, 3) != "2 of 3 rules active" ||
		zh.RulesActive(2, 3) != "共 3 条规则，2 条已启用" {
		t.Errorf("RulesActive: %q / %q", en.RulesActive(2, 3), zh.RulesActive(2, 3))
	}
	// Every message must exist in both catalogs, not fall back to English.
	for name, pair := range map[string][2]string{
		"flushDNS":    {en.FlushDNS(), zh.FlushDNS()},
		"serviceWarn": {en.ServiceWarning(), zh.ServiceWarning()},
		"emptyBody":   {en.EmptyBody(), zh.EmptyBody()},
		"aboutDesc":   {en.AboutDesc1(), zh.AboutDesc1()},
		"newGroup":    {en.NewGroup(), zh.NewGroup()},
		"groups":      {en.GroupsSection(), zh.GroupsSection()},
		"statRules":   {en.StatRules(), zh.StatRules()},
		"confirm":     {en.ConfirmDeleteGroupTitle(), zh.ConfirmDeleteGroupTitle()},
		"filterAll":   {en.FilterAll(), zh.FilterAll()},
		"clear":       {en.ClearFilters(), zh.ClearFilters()},
		"colActions":  {en.ColActions(), zh.ColActions()},
		"pageOf":      {en.PageOf(1, 3), zh.PageOf(1, 3)},
	} {
		if pair[0] == "" || pair[1] == "" || pair[0] == pair[1] {
			t.Errorf("%s not translated: %q / %q", name, pair[0], pair[1])
		}
	}
}

func TestChineseView(t *testing.T) {
	t.Setenv("MYGO_LOCALE", "zh-Hans")
	a, _, _ := newTestApp(t)
	tt := ui.NewTester(a.view, 940, 640)
	tt.Frame()
	for _, want := range []string{
		"端口转发管理器", "新建规则", "刷新", "清理 DNS 缓存", "暂无转发规则",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestLanguagePreference(t *testing.T) {
	t.Setenv("PORTPROXY_PREFS_DIR", t.TempDir())
	if got := data.LoadLanguagePref(); got != "" {
		t.Fatalf("default pref = %q, want follow system", got)
	}
	if err := data.SaveLanguagePref("zh-Hans"); err != nil {
		t.Fatal(err)
	}
	if got := data.LoadLanguagePref(); got != "zh-Hans" {
		t.Errorf("pref = %q", got)
	}
	if err := data.SaveLanguagePref(""); err != nil {
		t.Fatal(err)
	}
	if got := data.LoadLanguagePref(); got != "" {
		t.Errorf("pref after reset = %q", got)
	}
}

func TestContextMenuNewRule(t *testing.T) {
	t.Setenv("PORTPROXY_PREFS_DIR", t.TempDir())
	a, _, store := newTestApp(t)
	if err := store.DB().Add(&data.Rule{
		Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
		ConnectTo: "10.0.0.1", ConnectPort: 4000,
	}); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	tt := ui.NewTester(a.view, 940, 640)
	tt.Frame()
	if err := tt.RightClick("v4tov4"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	if len(menu) == 0 {
		t.Fatal("no context menu opened")
	}
	if err := tt.ChooseMenuItem(locales.NewRuleItem()); err != nil {
		t.Fatalf("menu items %q: %v", menu, err)
	}
	if !a.set.open {
		t.Error("New rule dialog did not open from the context menu")
	}
}

func TestLanguageMenu(t *testing.T) {
	t.Setenv("PORTPROXY_PREFS_DIR", t.TempDir())
	a, _, _ := newTestApp(t)
	tt := ui.NewTester(a.view, 940, 640)
	tt.Frame()
	if err := tt.Click(locales.More()); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(locales.Language(), "简体中文"); err != nil {
		t.Fatalf("menu %q: %v", tt.Menu(), err)
	}
	if a.langOverride != "zh-Hans" || data.LoadLanguagePref() != "zh-Hans" {
		t.Errorf("language not switched: %q / %q",
			a.langOverride, data.LoadLanguagePref())
	}
}

func snapshot(t *testing.T, name string, view func(c *ui.Context), w, h int, dark bool) {
	t.Helper()
	dir := os.Getenv("SNAPSHOT")
	if dir == "" {
		return
	}
	tt := ui.NewTester(view, w, h)
	tt.SetDark(dark)
	if tag := os.Getenv("MYGO_LOCALE"); tag != "" {
		tt.SetLocale(tag)
	}
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshot(t *testing.T) {
	if os.Getenv("SNAPSHOT") == "" {
		t.Skip("SNAPSHOT is not set")
	}
	a, p, store := newTestApp(t)
	for _, r := range []data.Rule{
		{Type: "v4tov4", ListenOn: "0.0.0.0", ListenPort: 8080,
			ConnectTo: "127.0.0.1", ConnectPort: 80, Comment: "Local web", Group: "Development"},
		{Type: "v4tov4", ListenOn: "0.0.0.0", ListenPort: 3389,
			ConnectTo: "10.0.0.5", ConnectPort: 3389, Comment: "RDP", Group: "Development"},
		{Type: "v6tov4", ListenOn: "::", ListenPort: 9000,
			ConnectTo: "192.168.1.10", ConnectPort: 9000, Comment: "IPv6 relay"},
	} {
		if err := store.DB().Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	p.proxies = []win.Proxy{
		{Type: "v4tov4", ListenOn: "0.0.0.0", ListenPort: 8080,
			ConnectTo: "127.0.0.1", ConnectPort: 80},
		{Type: "v6tov4", ListenOn: "::", ListenPort: 9000,
			ConnectTo: "192.168.1.10", ConnectPort: 9000},
	}
	a.refresh()
	snapshot(t, "portproxy-light.png", a.view, 1280, 800, false)
	snapshot(t, "portproxy-dark.png", a.view, 1280, 800, true)
	snapshot(t, "portproxy-narrow.png", a.view, 1000, 720, false)

	for _, rw := range a.rows {
		if rw.hasRule {
			a.selection.Add(rw.key)
			break
		}
	}
	snapshot(t, "portproxy-selected.png", a.view, 1280, 800, false)
	a.selection.Clear()

	a.openNewProxy()
	snapshot(t, "portproxy-dialog.png", a.view, 1280, 800, false)
	a.set.open = false
	a.openNewGroup()
	snapshot(t, "portproxy-group.png", a.view, 1280, 800, false)
	a.group.open = false
	a.openDeleteGroup("Development")
	snapshot(t, "portproxy-confirm.png", a.view, 1280, 800, false)
	a.confirmOpen = false
	a.selection.Add(a.rows[len(a.rows)-1].key)
	a.askDelete([]string{a.rows[len(a.rows)-1].key})
	snapshot(t, "portproxy-delete-confirm.png", a.view, 1280, 800, false)
	a.deleteConfirm = deleteConfirmForm{}
	a.about = true
	snapshot(t, "portproxy-about.png", a.view, 1280, 800, false)

	paged, _, pagedStore := newTestApp(t)
	for i := 0; i < 25; i++ {
		if err := pagedStore.DB().Add(&data.Rule{
			Type: "v4tov4", ListenOn: "0.0.0.0", ListenPort: 1000 + i,
			ConnectTo: "10.0.0.1", ConnectPort: 80,
			Comment: "Rule number " + strconv.Itoa(i+1),
		}); err != nil {
			t.Fatal(err)
		}
	}
	paged.refresh()
	paged.setPageSize(10)
	snapshot(t, "portproxy-paged.png", paged.view, 1280, 800, false)
}
