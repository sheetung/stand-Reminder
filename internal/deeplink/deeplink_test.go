package deeplink

import "testing"

func TestParseActionArg(t *testing.T) {
	action, ok := ParseActionArg([]string{"standreminder://action?action=break"})
	if !ok {
		t.Fatal("expected deeplink action to be detected")
	}
	if action != "break" {
		t.Fatalf("expected action break, got %q", action)
	}
}

func TestParseActionArgIgnoresNormalArgs(t *testing.T) {
	if action, ok := ParseActionArg([]string{"--version"}); ok || action != "" {
		t.Fatalf("expected no action, got %q", action)
	}
}
