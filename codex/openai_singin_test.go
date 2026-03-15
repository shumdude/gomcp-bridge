package main

import "testing"

func TestBuildExecArgsForNewSession(t *testing.T) {
	client := &openAISignInClient{model: "gpt-5"}

	got := client.buildExecArgs(`C:\repo`, false, `C:\tmp\out.txt`)
	want := []string{
		"-C", `C:\repo`,
		"exec",
		"--color", "never",
		"-s", "read-only",
		"--skip-git-repo-check",
		"--output-last-message", `C:\tmp\out.txt`,
		"-m", "gpt-5",
		"-",
	}

	assertArgsEqual(t, got, want)
}

func TestBuildExecArgsForResumePlacesExecFlagsBeforeSubcommand(t *testing.T) {
	client := &openAISignInClient{model: "gpt-5"}

	got := client.buildExecArgs(`C:\repo`, true, `C:\tmp\out.txt`)
	want := []string{
		"-C", `C:\repo`,
		"exec",
		"--color", "never",
		"resume",
		"--last",
		"--skip-git-repo-check",
		"--output-last-message", `C:\tmp\out.txt`,
		"-m", "gpt-5",
		"-",
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
