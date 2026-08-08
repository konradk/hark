package dependencies

import (
	"errors"
	"reflect"
	"testing"
)

func TestCheckWithLookPath(t *testing.T) {
	tools := []Tool{
		{Name: "present", Purpose: "does work"},
		{Name: "missing", Purpose: "does other work", InstallHint: "install missing"},
	}

	statuses := checkWithLookPath(tools, func(name string) (string, error) {
		if name == "present" {
			return "/usr/bin/present", nil
		}
		return "", errors.New("not found")
	})

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if !statuses[0].Found || statuses[0].Path != "/usr/bin/present" {
		t.Fatalf("unexpected present status: %#v", statuses[0])
	}
	if statuses[1].Found || statuses[1].InstallHint != "install missing" {
		t.Fatalf("unexpected missing status: %#v", statuses[1])
	}
}

func TestMissingNames(t *testing.T) {
	statuses := []Status{
		{Name: "one", Found: true},
		{Name: "two", Found: false},
		{Name: "three", Found: false},
	}

	got := MissingNames(statuses)
	want := []string{"two", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingNames() = %#v, want %#v", got, want)
	}
	if summary := MissingSummary(statuses); summary != "two, three" {
		t.Fatalf("MissingSummary() = %q", summary)
	}
}
