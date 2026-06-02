package agentless

// Remote script execution over WinRM — the core RMM "run a command everywhere"
// capability. This is the single most dangerous primitive in the product, so:
//   - EVERY run is persisted to an audit trail by the caller (api -> store).
//   - Output is captured (stdout/stderr/exit) and bounded so a runaway command
//     can't blow up memory or the response.
//   - We reuse the same credentialed WinRM transport as the inventory collector
//     (clean-room, no third-party agent), one fresh client per run.
//
// Shells:
//   - "powershell" (default): script is run as a PowerShell scriptblock via the
//     UTF-16LE+base64 -EncodedCommand path (handles quoting/newlines safely).
//   - "cmd": script is passed verbatim to the legacy cmd.exe processor.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/masterzen/winrm"
)

// maxOutput caps captured stdout/stderr (each) returned to the caller. RMM
// scripts occasionally dump a lot; we keep the tail bounded and flag truncation.
const maxOutput = 64 * 1024

// ScriptResult is the outcome of one remote execution.
type ScriptResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Truncated  bool   `json:"truncated"`
}

// RunScript executes script on a remote Windows host over WinRM and captures
// its output. shell is "powershell" (default) or "cmd". It returns a transport
// error only when the command could not be dispatched at all; a non-zero exit
// of the remote command is reported via ScriptResult.ExitCode (not an error),
// because "the command ran and failed" is a normal, auditable RMM outcome.
func RunScript(ctx context.Context, c Conn, shell, script string) (ScriptResult, error) {
	var res ScriptResult
	if strings.TrimSpace(script) == "" {
		return res, fmt.Errorf("empty script")
	}
	client, err := newClient(c)
	if err != nil {
		return res, fmt.Errorf("winrm client: %w", err)
	}

	var command string
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "", "powershell", "ps", "pwsh":
		// UTF-16LE + base64 -EncodedCommand: safe for arbitrary quoting/newlines.
		command = winrm.Powershell(script)
	case "cmd", "bat", "batch":
		command = script
	default:
		return res, fmt.Errorf("unsupported shell %q (use powershell|cmd)", shell)
	}

	start := time.Now()
	stdout, stderr, code, err := client.RunWithContextWithString(ctx, command, "")
	res.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		return res, classify(err, stderr)
	}
	res.Stdout, res.Truncated = clamp(stdout)
	res.Stderr, _ = clamp(stderr)
	res.ExitCode = code
	return res, nil
}

// clamp bounds a captured stream to maxOutput, keeping the tail (where errors
// and final results usually are) and prefixing a truncation marker.
func clamp(s string) (string, bool) {
	if len(s) <= maxOutput {
		return s, false
	}
	return "...[truncated]...\n" + s[len(s)-maxOutput:], true
}
