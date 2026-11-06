package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Custom groups live in our own preferences, never in config.db: the
// original PortProxyGUI derives groups only from rules and must keep reading
// the same schema. A group's membership is still stored in rules.Group, so
// everything remains visible to the original app.

func normalizeGroups(groups []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

func LoadCustomGroups() []string {
	dir, err := PrefsDir()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(dir, "groups.json"))
	if err != nil {
		return nil
	}
	var groups []string
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil
	}
	return normalizeGroups(groups)
}

func SaveCustomGroups(groups []string) error {
	dir, err := PrefsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(normalizeGroups(groups))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "groups.json"), b, 0o644)
}

func (d *DB) RenameGroup(oldName, newName string) error {
	_, err := d.db.Exec(
		"UPDATE Rules SET `Group` = ? WHERE `Group` = ?;", newName, oldName)
	return err
}

// ClearGroup dissolves a group: its rules stay, they just become ungrouped.
func (d *DB) ClearGroup(name string) error {
	_, err := d.db.Exec(
		"UPDATE Rules SET `Group` = '' WHERE `Group` = ?;", name)
	return err
}

func (d *DB) SetRuleGroup(id, group string) error {
	_, err := d.db.Exec(
		"UPDATE Rules SET `Group` = ? WHERE Id = ?;", group, id)
	return err
}
