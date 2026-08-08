package hyprland

import "testing"

func TestParseWindow(t *testing.T) {
	win, err := ParseWindow([]byte(`{
    "address": "0x5560ac9ebc30",
    "workspace": {"id": 3, "name": "3"},
    "class": "dev.zed.Zed",
    "title": "hark",
    "at": [12, 38],
    "size": [1704, 1030],
    "pid": 99113,
    "mapped": true,
    "hidden": false
  }`))
	if err != nil {
		t.Fatalf("ParseWindow returned error: %v", err)
	}
	if win.Address != "0x5560ac9ebc30" {
		t.Fatalf("unexpected address: %q", win.Address)
	}
	if win.Class != "dev.zed.Zed" {
		t.Fatalf("unexpected class: %q", win.Class)
	}
	if win.Workspace.Name != "3" {
		t.Fatalf("unexpected workspace: %q", win.Workspace.Name)
	}
	if win.At != [2]int{12, 38} || win.Size != [2]int{1704, 1030} {
		t.Fatalf("unexpected window geometry: at=%v size=%v", win.At, win.Size)
	}
}

func TestParseWindowRejectsMissingAddress(t *testing.T) {
	if _, err := ParseWindow([]byte(`{"address":"0x0"}`)); err == nil {
		t.Fatal("expected missing address error")
	}
}
