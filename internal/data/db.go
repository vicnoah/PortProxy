package data

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"
)

const AppDbFileName = "config.db"

// migrations is PortProxyGUI.Data.MigrationUtil.History, in order. The last
// entry must be present in __history or the reference app would try to run
// its CREATE TABLE migrations again on a database we already built.
var migrations = []struct{ ID, Version string }{
	{"000000000000", "0.0"},
	{"202103021542", "1.1.0"},
	{"202201172103", "1.2.0"},
	{"202202221635", "1.3.0"},
	{"202303092024", "1.4.0"},
}

const createRules = `CREATE TABLE rules (
	Id text PRIMARY KEY,
	Type text,
	ListenOn text,
	ListenPort integer,
	ConnectTo text,
	ConnectPort integer,
	Comment text,
	` + "`Group`" + ` text 
);`

const createRulesIndex = "CREATE UNIQUE INDEX IX_Rules_Type_ListenOn_ListenPort ON Rules ( Type, ListenOn, ListenPort );"

const createConfigs = "CREATE TABLE configs (\n\tItem text,\n\t`Key` text,\n\tValue text\n);"

const createConfigsIndex = "CREATE UNIQUE INDEX IF NOT EXISTS IX_Configs_Key ON configs ( Item, `Key` );"

type DB struct {
	db   *sql.DB
	path string
}

// DefaultDBPath is our own database, kept out of the install folder so an
// in-place (overwrite) upgrade never removes it. On Windows this is
// %APPDATA%\PortProxy\config.db. PORTPROXY_DB overrides it in tests.
func DefaultDBPath() (string, error) {
	if p := os.Getenv("PORTPROXY_DB"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PortProxy", AppDbFileName), nil
}

func Open(file string) (*DB, error) {
	if dir := filepath.Dir(file); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	// A single connection avoids locking surprises and keeps the file copy
	// of Export consistent with what the app has just written.
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		db.Close()
		return nil, err
	}
	d := &DB{db: db, path: file}
	if err = d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Path() string { return d.path }

func (d *DB) Close() error { return d.db.Close() }

func (d *DB) hasTable(name string) (bool, error) {
	var n int
	err := d.db.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?;",
		name,
	).Scan(&n)
	return n > 0, err
}

func (d *DB) tableColumns(name string) (map[string]bool, error) {
	rows, err := d.db.Query("PRAGMA table_info(" + name + ");")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var cname, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[cname] = true
	}
	return cols, rows.Err()
}

// migrate brings any config.db to the exact schema of PortProxyGUI 1.4.x.
// It is idempotent, so it also works on a database the reference app made.
func (d *DB) migrate() error {
	if _, err := d.db.Exec(
		"CREATE TABLE IF NOT EXISTS __history ( MigrationId text PRIMARY KEY, ProductVersion text);",
	); err != nil {
		return err
	}

	hasRules, err := d.hasTable("rules")
	if err != nil {
		return err
	}
	if !hasRules {
		if _, err := d.db.Exec(createRules); err != nil {
			return err
		}
	} else {
		cols, err := d.tableColumns("rules")
		if err != nil {
			return err
		}
		if !cols["Comment"] {
			if cols["Note"] {
				// The 1.3.0 migration of the reference app: Note -> Comment.
				stmts := []string{
					"ALTER TABLE rules RENAME TO rulesOld;",
					"DROP INDEX IF EXISTS IX_Rules_Type_ListenOn_ListenPort;",
					createRules,
					createRulesIndex,
					"INSERT INTO rules SELECT Id, Type, ListenOn, ListenPort, ConnectTo, ConnectPort, Note, `Group` FROM rulesOld;",
					"DROP TABLE rulesOld;",
				}
				for _, s := range stmts {
					if _, err := d.db.Exec(s); err != nil {
						return err
					}
				}
			} else if _, err := d.db.Exec("ALTER TABLE rules ADD Comment text;"); err != nil {
				return err
			}
		}
		cols, err = d.tableColumns("rules")
		if err != nil {
			return err
		}
		if !cols["Group"] {
			if _, err := d.db.Exec("ALTER TABLE rules ADD `Group` text;"); err != nil {
				return err
			}
		}
	}

	hasConfigs, err := d.hasTable("configs")
	if err != nil {
		return err
	}
	if !hasConfigs {
		if _, err := d.db.Exec(createConfigs); err != nil {
			return err
		}
	}

	stmts := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS IX_Rules_Type_ListenOn_ListenPort ON Rules ( Type, ListenOn, ListenPort );",
		createConfigsIndex,
		"INSERT OR IGNORE INTO configs ( Item, `Key`, Value ) VALUES ( 'MainWindow', 'Width', '720' );",
		"INSERT OR IGNORE INTO configs ( Item, `Key`, Value ) VALUES ( 'MainWindow', 'Height', '500' );",
		"INSERT OR IGNORE INTO configs ( Item, `Key`, Value ) VALUES ( 'PortProxy', 'ColumnWidths', '[24, 64, 140, 100, 140, 100, 100]' );",
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return err
		}
	}
	for _, m := range migrations {
		if _, err := d.db.Exec(
			"INSERT OR IGNORE INTO __history (MigrationId, ProductVersion) VALUES (?, ?);",
			m.ID, m.Version,
		); err != nil {
			return err
		}
	}
	return nil
}

const selectRules = "SELECT Id, Type, ListenOn, ListenPort, ConnectTo, ConnectPort, Comment, `Group` FROM Rules"

func scanRule(rows *sql.Rows) (Rule, error) {
	var r Rule
	var id, typ, listenOn, connectTo, comment, group sql.NullString
	var listenPort, connectPort sql.NullInt64
	if err := rows.Scan(&id, &typ, &listenOn, &listenPort, &connectTo,
		&connectPort, &comment, &group); err != nil {
		return r, err
	}
	r.Id = id.String
	r.Type = typ.String
	r.ListenOn = listenOn.String
	r.ListenPort = int(listenPort.Int64)
	r.ConnectTo = connectTo.String
	r.ConnectPort = int(connectPort.Int64)
	r.Comment = comment.String
	r.Group = group.String
	return r, nil
}

func (d *DB) Rules() ([]Rule, error) {
	rows, err := d.db.Query(selectRules + ";")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []Rule
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (d *DB) GetRule(typ, listenOn string, listenPort int) (*Rule, error) {
	rows, err := d.db.Query(selectRules+
		" WHERE Type = ? AND ListenOn = ? AND ListenPort = ? LIMIT 1;",
		typ, listenOn, listenPort)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	r, err := scanRule(rows)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *DB) Add(r *Rule) error {
	id := NewId()
	_, err := d.db.Exec(
		"INSERT INTO Rules (Id, Type, ListenOn, ListenPort, ConnectTo, ConnectPort, Comment, `Group`) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?);",
		id,
		r.Type,
		r.ListenOn,
		r.ListenPort,
		r.ConnectTo,
		r.ConnectPort,
		r.Comment,
		r.Group,
	)
	if err != nil {
		return err
	}
	r.Id = id
	return nil
}

// Update mirrors the reference app: Comment and Group are deliberately left
// alone, they are metadata that survives a registry-driven refresh.
func (d *DB) Update(r Rule) error {
	_, err := d.db.Exec(
		"UPDATE Rules SET Type = ?, ListenOn = ?, ListenPort = ?, ConnectTo = ?, ConnectPort = ? WHERE Id = ?;",
		r.Type,
		r.ListenOn,
		r.ListenPort,
		r.ConnectTo,
		r.ConnectPort,
		r.Id,
	)
	return err
}

func (d *DB) Remove(r Rule) error {
	_, err := d.db.Exec("DELETE FROM Rules WHERE Id = ?;", r.Id)
	return err
}

func (d *DB) GetAppConfig() (AppConfig, error) {
	rows, err := d.db.Query("SELECT Item, `Key`, Value FROM Configs;")
	if err != nil {
		return DefaultAppConfig(), err
	}
	defer rows.Close()
	var cr []ConfigRow
	for rows.Next() {
		var item, key, value sql.NullString
		if err := rows.Scan(&item, &key, &value); err != nil {
			return DefaultAppConfig(), err
		}
		cr = append(cr, ConfigRow{Item: item.String, Key: key.String, Value: value.String})
	}
	if err := rows.Err(); err != nil {
		return DefaultAppConfig(), err
	}
	return AppConfigFromRows(cr), nil
}

func (d *DB) SaveAppConfig(cfg AppConfig) error {
	values := [][3]string{
		{"MainWindow", "Width", strconv.Itoa(cfg.MainWindowWidth)},
		{"MainWindow", "Height", strconv.Itoa(cfg.MainWindowHeight)},
		{"PortProxy", "ColumnWidths", ColumnWidthsString(cfg.ColumnWidths)},
	}
	for _, v := range values {
		if _, err := d.db.Exec(
			"UPDATE Configs SET Value = ? WHERE Item = ? AND `Key` = ?;",
			v[2], v[0], v[1],
		); err != nil {
			return err
		}
	}
	return nil
}

// ExportTo copies config.db to path, exactly as the reference app's
// Export menu does (SaveFileDialog + File.Copy).
func (d *DB) ExportTo(path string) error {
	if path == d.path {
		return nil
	}
	_, _ = d.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	src, err := os.Open(d.path)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

// ImportFrom merges the rules of another config.db into ours, skipping the
// ones whose natural key already exists. The source is migrated first, like
// ApplicationDbScope.FromFile in the reference app.
func (d *DB) ImportFrom(path string) (int, error) {
	if path == d.path {
		return 0, nil
	}
	other, err := Open(path)
	if err != nil {
		return 0, err
	}
	defer other.Close()
	rules, err := other.Rules()
	if err != nil {
		return 0, err
	}
	added := 0
	for i := range rules {
		exists, err := d.GetRule(rules[i].Type, rules[i].ListenOn, rules[i].ListenPort)
		if err != nil {
			return added, err
		}
		if exists == nil {
			if err := d.Add(&rules[i]); err != nil {
				return added, err
			}
			added++
		}
	}
	return added, nil
}

// SetConfigValue is used by tests to emulate the reference app writing a
// config row directly.
func (d *DB) SetConfigValue(item, key, value string) error {
	_, err := d.db.Exec(
		"UPDATE Configs SET Value = ? WHERE Item = ? AND `Key` = ?;", value, item, key)
	return err
}

func (d *DB) RawRules() ([]Rule, error) { return d.Rules() }

func (d *DB) DebugSchema() (string, error) {
	rows, err := d.db.Query(
		"SELECT type, name, sql FROM sqlite_master WHERE name IN ('rules','configs','__history','IX_Rules_Type_ListenOn_ListenPort','IX_Configs_Key') ORDER BY name;",
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	out := ""
	for rows.Next() {
		var typ, name string
		var sqlText sql.NullString
		if err := rows.Scan(&typ, &name, &sqlText); err != nil {
			return "", err
		}
		out += fmt.Sprintf("%s %s: %s\n", typ, name, sqlText.String)
	}
	return out, rows.Err()
}
