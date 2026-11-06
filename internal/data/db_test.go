package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// referenceMigrations is the exact SQL of PortProxyGUI.Data.MigrationUtil,
// used to prove that a database made by the original app opens in ours.
var referenceMigrations = []string{
	`CREATE TABLE __history ( MigrationId text PRIMARY KEY, ProductVersion text);`,
	`INSERT INTO __history (MigrationId, ProductVersion) VALUES ('000000000000', '0.0');`,
	`CREATE TABLE rules
(
    Id text PRIMARY KEY,
    Type text,
    ListenOn text,
    ListenPort integer,
    ConnectTo text,
    ConnectPort integer
);`,
	`CREATE UNIQUE INDEX IX_Rules_Type_ListenOn_ListenPort ON Rules(Type, ListenOn, ListenPort);`,
	`INSERT INTO __history (MigrationId, ProductVersion) VALUES ('202103021542', '1.1.0');`,
	`ALTER TABLE rules ADD Note text;`,
	`ALTER TABLE rules ADD ` + "`Group`" + ` text;`,
	`INSERT INTO __history (MigrationId, ProductVersion) VALUES ('202201172103', '1.2.0');`,
	`ALTER TABLE rules RENAME TO rulesOld;`,
	`DROP INDEX IX_Rules_Type_ListenOn_ListenPort;`,
	`CREATE TABLE rules (
	Id text PRIMARY KEY,
	Type text,
	ListenOn text,
	ListenPort integer,
	ConnectTo text,
	ConnectPort integer,
	Comment text,
	` + "`Group`" + ` text 
);`,
	`CREATE UNIQUE INDEX IX_Rules_Type_ListenOn_ListenPort ON Rules ( Type, ListenOn, ListenPort );`,
	`INSERT INTO rules SELECT Id, Type, ListenOn, ListenPort, ConnectTo, ConnectPort, Note, ` + "`Group`" + ` FROM rulesOld;`,
	`DROP TABLE rulesOld;`,
	`INSERT INTO __history (MigrationId, ProductVersion) VALUES ('202202221635', '1.3.0');`,
	`CREATE TABLE configs (
	Item text,
	` + "`Key`" + ` text,
	Value text
);`,
	`CREATE UNIQUE INDEX IX_Configs_Key ON configs ( Item, ` + "`Key`" + ` );`,
	`INSERT INTO configs ( Item, ` + "`Key`" + `, Value ) VALUES ( 'MainWindow', 'Width', '720' );`,
	`INSERT INTO configs ( Item, ` + "`Key`" + `, Value ) VALUES ( 'MainWindow', 'Height', '500' );`,
	`INSERT INTO configs ( Item, ` + "`Key`" + `, Value ) VALUES ( 'PortProxy', 'ColumnWidths', '[24, 64, 140, 100, 140, 100, 100]' );`,
	`INSERT INTO __history (MigrationId, ProductVersion) VALUES ('202303092024', '1.4.0');`,
}

func makeReferenceDB(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	for _, s := range referenceMigrations {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("reference migration %q: %v", s, err)
		}
	}
	if _, err := raw.Exec(
		"INSERT INTO Rules (Id, Type, ListenOn, ListenPort, ConnectTo, ConnectPort, Comment, `Group`) " +
			"VALUES ('3f2504e0-4f89-11d3-9a0c-0305e82c3301', 'v4tov4', '0.0.0.0', 8080, '127.0.0.1', 80, 'web', 'Servers');",
	); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaMatchesReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PortProxyGUI", "config.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, table := range []string{"rules", "configs", "__history"} {
		ok, err := d.hasTable(table)
		if err != nil || !ok {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	cols, err := d.tableColumns("rules")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Id",
		"Type",
		"ListenOn",
		"ListenPort",
		"ConnectTo",
		"ConnectPort",
		"Comment",
		"Group",
	}
	for _, c := range want {
		if !cols[c] {
			t.Errorf("rules is missing column %q: %v", c, cols)
		}
	}
	if len(cols) != len(want) {
		t.Errorf("rules has unexpected columns: %v", cols)
	}

	var indexNames []string
	rows, err := d.db.Query(
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name LIKE 'IX_%' ORDER BY name;",
	)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		indexNames = append(indexNames, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(
		indexNames,
		[]string{"IX_Configs_Key", "IX_Rules_Type_ListenOn_ListenPort"},
	) {
		t.Errorf("indexes = %v", indexNames)
	}

	var lastID, lastVersion string
	if err := d.db.QueryRow(
		"SELECT MigrationId, ProductVersion FROM __history ORDER BY MigrationId DESC LIMIT 1;",
	).Scan(&lastID, &lastVersion); err != nil {
		t.Fatal(err)
	}
	// The reference app's MigrateToLast only stays idle when this is the last
	// id in its History table, and EnsureUpdateVersion must not trigger.
	if lastID != "202303092024" || lastVersion != "1.4.0" {
		t.Errorf("last migration = %s / %s", lastID, lastVersion)
	}
	var n int
	if err := d.db.QueryRow("SELECT count(*) FROM __history;").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(migrations) {
		t.Errorf("__history rows = %d, want %d", n, len(migrations))
	}

	cfg, err := d.GetAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MainWindowWidth != 720 || cfg.MainWindowHeight != 500 ||
		!reflect.DeepEqual(cfg.ColumnWidths, DefaultColumnWidths) {
		t.Errorf("default config = %+v", cfg)
	}
}

func TestOpenReferenceDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PortProxyGUI", "config.db")
	makeReferenceDB(t, path)

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	rules, err := d.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %+v", rules)
	}
	r := rules[0]
	if r.Id != "3f2504e0-4f89-11d3-9a0c-0305e82c3301" || r.Type != "v4tov4" ||
		r.ListenOn != "0.0.0.0" || r.ListenPort != 8080 || r.ConnectTo != "127.0.0.1" ||
		r.ConnectPort != 80 || r.Comment != "web" || r.Group != "Servers" {
		t.Errorf("rule = %+v", r)
	}

	cfg, err := d.GetAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MainWindowWidth != 720 || !reflect.DeepEqual(cfg.ColumnWidths, DefaultColumnWidths) {
		t.Errorf("config = %+v", cfg)
	}

	// Our writes must land in the same schema the reference app reads.
	add := Rule{Type: "v6tov6", ListenOn: "::", ListenPort: 9000,
		ConnectTo: "::1", ConnectPort: 9001, Comment: "v6", Group: "Test"}
	if err := d.Add(&add); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(add.Id, "-") || len(add.Id) != 36 {
		t.Errorf("id %q is not a GUID", add.Id)
	}
	got, err := d.GetRule("v6tov6", "::", 9000)
	if err != nil || got == nil || got.Comment != "v6" {
		t.Fatalf("GetRule = %+v, %v", got, err)
	}
}

func TestLegacySchemaUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`CREATE TABLE __history ( MigrationId text PRIMARY KEY, ProductVersion text);`,
		`INSERT INTO __history (MigrationId, ProductVersion) VALUES ('000000000000', '0.0');`,
		`CREATE TABLE rules (Id text PRIMARY KEY, Type text, ListenOn text, ListenPort integer, ConnectTo text, ConnectPort integer, Note text, ` + "`Group`" + ` text);`,
		`INSERT INTO rules VALUES ('legacy-1','v4tov4','*',1234,'10.0.0.1',4321,'old note','Legacy');`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	raw.Close()

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	rules, err := d.Rules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Comment != "old note" || rules[0].Group != "Legacy" {
		t.Errorf("upgraded rules = %+v", rules)
	}
	if _, err := d.GetAppConfig(); err != nil {
		t.Fatal(err)
	}
}

func TestRuleCRUDAndConfig(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	r := Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
		ConnectTo: "192.168.1.2", ConnectPort: 4000, Comment: "c", Group: "g"}
	if err := d.Add(&r); err != nil {
		t.Fatal(err)
	}
	r.ConnectTo, r.ConnectPort = "192.168.1.9", 4001
	r.Comment, r.Group = "changed", "changed-group"
	if err := d.Update(r); err != nil {
		t.Fatal(err)
	}
	got, err := d.GetRule("v4tov4", "*", 3000)
	if err != nil || got == nil {
		t.Fatalf("%+v %v", got, err)
	}
	// Update deliberately leaves Comment and Group, like the reference app.
	if got.ConnectTo != "192.168.1.9" || got.ConnectPort != 4001 ||
		got.Comment != "c" || got.Group != "g" {
		t.Errorf("after update = %+v", got)
	}
	if err := d.Remove(r); err != nil {
		t.Fatal(err)
	}
	if rules, _ := d.Rules(); len(rules) != 0 {
		t.Errorf("rules after remove = %+v", rules)
	}

	cfg := AppConfig{MainWindowWidth: 1024, MainWindowHeight: 768,
		ColumnWidths: []int{30, 70, 150, 110, 150, 110, 110}}
	if err := d.SaveAppConfig(cfg); err != nil {
		t.Fatal(err)
	}
	read, err := d.GetAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, cfg) {
		t.Errorf("config round trip = %+v, want %+v", read, cfg)
	}
}

func TestImportExport(t *testing.T) {
	dir := t.TempDir()
	a, err := Open(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(filepath.Join(dir, "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	for _, r := range []Rule{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000, ConnectTo: "a", ConnectPort: 1},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 2000, ConnectTo: "b", ConnectPort: 2},
	} {
		if err := a.Add(&r); err != nil {
			t.Fatal(err)
		}
	}
	// A duplicate key must not be imported twice.
	dup := Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 1000, ConnectTo: "z", ConnectPort: 9}
	if err := b.Add(&dup); err != nil {
		t.Fatal(err)
	}

	exported := filepath.Join(dir, "export.db")
	if err := a.ExportTo(exported); err != nil {
		t.Fatal(err)
	}
	n, err := b.ImportFrom(exported)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("imported %d rules, want 1", n)
	}
	rules, _ := b.Rules()
	if len(rules) != 2 {
		t.Errorf("rules after import = %+v", rules)
	}
}

func TestStoreIsIndependent(t *testing.T) {
	dir := t.TempDir()
	refPath := filepath.Join(dir, "ref", "PortProxyGUI", "config.db")
	ownPath := filepath.Join(dir, "own", "config.db")
	makeReferenceDB(t, refPath)

	s, err := OpenStore(ownPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// No first-run copy: our database starts empty even when a config.db
	// from the original app is right there.
	if rules, err := s.DB().Rules(); err != nil || len(rules) != 0 {
		t.Fatalf("first run must not copy reference data: %+v, %v", rules, err)
	}

	// Our edits are never mirrored back into the reference database.
	ours := Rule{Type: "v4tov4", ListenOn: "*", ListenPort: 7777,
		ConnectTo: "our-host", ConnectPort: 77, Comment: "ours"}
	if err := s.DB().Add(&ours); err != nil {
		t.Fatal(err)
	}
	refDB, err := Open(refPath)
	if err != nil {
		t.Fatal(err)
	}
	defer refDB.Close()
	if got, _ := refDB.GetRule("v4tov4", "*", 7777); got != nil {
		t.Errorf("our write leaked into the reference database: %+v", got)
	}

	// Compatibility stays available manually through Import / Export.
	n, err := s.DB().ImportFrom(refPath)
	if err != nil || n != 1 {
		t.Fatalf("ImportFrom = %d, %v", n, err)
	}
	exported := filepath.Join(dir, "export.db")
	if err := s.DB().ExportTo(exported); err != nil {
		t.Fatal(err)
	}
	ext, err := Open(exported)
	if err != nil {
		t.Fatal(err)
	}
	defer ext.Close()
	if got, _ := ext.GetRule("v4tov4", "0.0.0.0", 8080); got == nil {
		t.Error("exported config.db lost the imported rule")
	}
}

// TestDumpForExternalCheck writes a populated database to
// PORTPROXY_DUMP_DB so a standard SQLite tool can inspect it from outside
// the Go process (proof that the file is a normal config.db).
func TestDumpForExternalCheck(t *testing.T) {
	out := os.Getenv("PORTPROXY_DUMP_DB")
	if out == "" {
		t.Skip("PORTPROXY_DUMP_DB is not set")
	}
	_ = os.Remove(out)
	d, err := Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Add(&Rule{Type: "v4tov4", ListenOn: "0.0.0.0", ListenPort: 8080,
		ConnectTo: "127.0.0.1", ConnectPort: 80, Comment: "hello", Group: "Demo"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveAppConfig(AppConfig{MainWindowWidth: 900, MainWindowHeight: 620,
		ColumnWidths: []int{24, 64, 140, 100, 140, 100, 100}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", out)
}

func TestParsePort(t *testing.T) {
	for _, s := range []string{"1", "80", "65535", " 8080 "} {
		if _, err := ParsePort(s); err != nil {
			t.Errorf("ParsePort(%q): %v", s, err)
		}
	}
	for _, s := range []string{"", "0", "65536", "-1", "abc", "80.5"} {
		if _, err := ParsePort(s); err == nil {
			t.Errorf("ParsePort(%q) should fail", s)
		}
	}
}

func TestAppConfigFromRows(t *testing.T) {
	rows := []ConfigRow{
		{Item: "MainWindow", Key: "Width", Value: "900"},
		{Item: "MainWindow", Key: "Height", Value: "600"},
		{Item: "PortProxy", Key: "ColumnWidths", Value: "[10, 20, 30]"},
	}
	cfg := AppConfigFromRows(rows)
	if cfg.MainWindowWidth != 900 || cfg.MainWindowHeight != 600 ||
		!reflect.DeepEqual(cfg.ColumnWidths, []int{10, 20, 30}) {
		t.Errorf("cfg = %+v", cfg)
	}
	if ColumnWidthsString(cfg.ColumnWidths) != "[10, 20, 30]" {
		t.Errorf("ColumnWidthsString = %q", ColumnWidthsString(cfg.ColumnWidths))
	}
	fallback := AppConfigFromRows(nil)
	if fallback.MainWindowWidth != 720 || len(fallback.ColumnWidths) != 0 {
		t.Errorf("fallback = %+v", fallback)
	}
}
