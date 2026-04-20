package prompts

import (
	"strings"
	"testing"
)

func TestUsage_containsToolNames(t *testing.T) {
	for _, name := range []string{
		"create_checkpoint", "render", "diff_checkpoints", "diff_paths", "dependencies",
		"inventory", "trace",
	} {
		if !strings.Contains(Usage, name) {
			t.Fatalf("Usage should mention %q", name)
		}
	}
}

func TestExplainBody_includesQuery(t *testing.T) {
	body := ExplainBody("what is this overlay?")
	if !strings.Contains(body, "what is this overlay?") {
		t.Fatal(body)
	}
	if !strings.Contains(body, "create_checkpoint") {
		t.Fatal("expected Usage embedded in body")
	}
}

func TestRefactorBody_includesQuery(t *testing.T) {
	body := RefactorBody("merge bases")
	if !strings.Contains(body, "merge bases") {
		t.Fatal(body)
	}
}

func TestDiffDirsMessages(t *testing.T) {
	m := DiffDirsMessages("envs/dev", "envs/prod")
	if !strings.Contains(m[0], "envs/dev") || !strings.Contains(m[0], "envs/prod") {
		t.Fatal(m[0])
	}
	if !strings.Contains(m[1], "create_checkpoint") || !strings.Contains(m[1], "diff_paths") {
		t.Fatal(m[1])
	}
}

func TestTroubleshootBody(t *testing.T) {
	body := TroubleshootBody("overlays/prod", "Deployment", "frontend")
	if !strings.Contains(body, "Deployment/frontend") {
		t.Fatal("should contain resource identifier")
	}
	if !strings.Contains(body, "overlays/prod") {
		t.Fatal("should contain path")
	}
	if !strings.Contains(body, "trace") {
		t.Fatal("should mention trace tool")
	}
}
