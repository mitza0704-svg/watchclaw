package agentless

import (
	"context"
	"strings"
	"testing"
)

func TestClamp(t *testing.T) {
	small := "hello"
	if got, trunc := clamp(small); got != small || trunc {
		t.Fatalf("small clamp: got %q trunc=%v", got, trunc)
	}
	big := strings.Repeat("x", maxOutput+1000)
	got, trunc := clamp(big)
	if !trunc {
		t.Fatal("expected truncation flag for oversized output")
	}
	if !strings.HasPrefix(got, "...[truncated]...") {
		t.Fatalf("expected truncation marker, got prefix %q", got[:20])
	}
	// keeps the tail, bounded to maxOutput (+marker).
	if !strings.HasSuffix(got, strings.Repeat("x", 10)) {
		t.Fatal("expected tail of original to be preserved")
	}
}

func TestRunScriptValidation(t *testing.T) {
	ctx := context.Background()
	c := Conn{Host: "127.0.0.1", User: "u", Pass: "p"}

	if _, err := RunScript(ctx, c, "powershell", "   "); err == nil {
		t.Fatal("expected error for empty script")
	}
	if _, err := RunScript(ctx, c, "fish", "echo hi"); err == nil ||
		!strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("expected unsupported-shell error, got %v", err)
	}
}
