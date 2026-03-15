package main

import "testing"

func TestBuildPrintArgsForNewSession(t *testing.T) {
	client := &claudeSignInClient{model: "sonnet"}

	got := client.buildPrintArgs("")
	want := []string{
		"-p",
		"--output-format", "json",
		"--input-format", "text",
		"--model", "sonnet",
	}

	assertArgsEqual(t, got, want)
}

func TestBuildPrintArgsForResumeSession(t *testing.T) {
	client := &claudeSignInClient{model: "claude-sonnet-4-6"}

	got := client.buildPrintArgs("11111111-1111-1111-1111-111111111111")
	want := []string{
		"-p",
		"--output-format", "json",
		"--input-format", "text",
		"--model", "claude-sonnet-4-6",
		"-r", "11111111-1111-1111-1111-111111111111",
	}

	assertArgsEqual(t, got, want)
}

func assertArgsEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unexpected args length: got %d want %d\n got: %#v\nwant: %#v", len(got), len(want), got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected arg at index %d: got %q want %q\n got: %#v\nwant: %#v", i, got[i], want[i], got, want)
		}
	}
}
