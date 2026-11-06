package data

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestCustomGroups(t *testing.T) {
	t.Setenv("PORTPROXY_PREFS_DIR", t.TempDir())
	if got := LoadCustomGroups(); len(got) != 0 {
		t.Fatalf("initial groups = %v", got)
	}
	if got := LoadPageSize(); got != 20 {
		t.Fatalf("default page size = %d", got)
	}
	if err := SavePageSize(50); err != nil {
		t.Fatal(err)
	}
	if got := LoadPageSize(); got != 50 {
		t.Errorf("page size = %d", got)
	}
	if err := SaveCustomGroups([]string{" Beta ", "Alpha", "", "Beta"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"Alpha", "Beta"}
	if got := LoadCustomGroups(); !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %v, want %v", got, want)
	}
}

func TestGroupMembership(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, r := range []Rule{
		{Type: "v4tov4", ListenOn: "*", ListenPort: 1000,
			ConnectTo: "a", ConnectPort: 1, Group: "Old"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 2000,
			ConnectTo: "b", ConnectPort: 2, Group: "Old"},
		{Type: "v4tov4", ListenOn: "*", ListenPort: 3000,
			ConnectTo: "c", ConnectPort: 3},
	} {
		if err := d.Add(&r); err != nil {
			t.Fatal(err)
		}
	}

	if err := d.RenameGroup("Old", "New"); err != nil {
		t.Fatal(err)
	}
	rules, _ := d.Rules()
	counts := map[string]int{}
	for _, r := range rules {
		counts[r.Group]++
	}
	if counts["New"] != 2 || counts["Old"] != 0 || counts[""] != 1 {
		t.Errorf("after rename = %v", counts)
	}

	if err := d.SetRuleGroup(rules[2].Id, "Moved"); err != nil {
		t.Fatal(err)
	}
	rules, _ = d.Rules()
	if rules[2].Group != "Moved" {
		t.Errorf("SetRuleGroup = %q", rules[2].Group)
	}

	if err := d.ClearGroup("New"); err != nil {
		t.Fatal(err)
	}
	if err := d.ClearGroup("Moved"); err != nil {
		t.Fatal(err)
	}
	rules, _ = d.Rules()
	if len(rules) != 3 {
		t.Fatalf("ClearGroup deleted rules: %+v", rules)
	}
	for _, r := range rules {
		if r.Group != "" {
			t.Errorf("rule still grouped: %+v", r)
		}
	}
}
