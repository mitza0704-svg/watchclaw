//! Command-channel: the agent polls the control plane for jobs targeted at this
//! host, executes them in the agent's machine context, and reports the result.
//!
//! This is the foundation for agent-side scripting, self-healing, and patch
//! deploy — actions that need the endpoint's own context (e.g. winget, which
//! cannot run over agentless WinRM). Stage 1 executes in the agent's context;
//! Stage 2 (Windows) launches into the active user session for packaged apps.

use crate::transport;
use serde::{Deserialize, Serialize};
use std::io::Write;
use std::process::{Command, Stdio};
use sysinfo::System;

const MAX_OUTPUT: usize = 64 * 1024;

#[derive(Deserialize)]
pub struct Job {
    pub id: i64,
    #[allow(dead_code)]
    pub kind: String,
    pub shell: String,
    pub command: String,
}

#[derive(Serialize)]
struct JobResult {
    exit_code: i32,
    stdout: String,
    stderr: String,
    status: String, // "done" = executed (any exit code); "failed" = could not dispatch
}

/// Poll for this host's pending jobs, run each, and post results back.
/// Errors are logged and swallowed — the agent loop must never die on a hiccup.
pub fn poll_and_run(base: &str) {
    let host = System::host_name().unwrap_or_else(|| "unknown".into());
    let base = base.trim_end_matches('/');
    let url = format!("{base}/v1/agent/jobs/{host}");
    let jobs: Vec<Job> = match transport::get_json(&url) {
        Ok(j) => j,
        Err(e) => {
            eprintln!("job poll failed: {e}");
            return;
        }
    };
    for job in jobs {
        let id = job.id;
        let res = execute(&job);
        let rurl = format!("{base}/v1/agent/jobs/{id}/result");
        match transport::post_json(&rurl, &res) {
            Ok(_) => println!("job {id} {} exit={}", res.status, res.exit_code),
            Err(e) => eprintln!("job {id} result post failed: {e}"),
        }
    }
}

fn execute(job: &Job) -> JobResult {
    let outcome = match job.shell.to_lowercase().as_str() {
        "powershell" | "ps" | "pwsh" => run_powershell(&job.command),
        "cmd" | "bat" | "batch" => run_simple("cmd", &["/C", &job.command]),
        "sh" | "bash" => run_simple("sh", &["-c", &job.command]),
        other => Err(format!("unsupported shell {other}")),
    };
    match outcome {
        Ok((code, out, err)) => JobResult {
            exit_code: code,
            stdout: clamp(out),
            stderr: clamp(err),
            status: "done".into(),
        },
        Err(e) => JobResult {
            exit_code: -1,
            stdout: String::new(),
            stderr: e,
            status: "failed".into(),
        },
    }
}

/// Run a PowerShell command by piping it to `-Command -` over stdin — avoids all
/// argument-quoting pitfalls for arbitrary multi-line scripts.
fn run_powershell(cmd: &str) -> Result<(i32, String, String), String> {
    let mut child = Command::new("powershell")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            "-",
        ])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|e| format!("spawn powershell: {e}"))?;
    {
        let mut stdin = child.stdin.take().ok_or("no stdin")?;
        stdin
            .write_all(cmd.as_bytes())
            .map_err(|e| format!("write stdin: {e}"))?;
    } // stdin dropped here -> EOF so PowerShell runs and exits
    let out = child
        .wait_with_output()
        .map_err(|e| format!("wait: {e}"))?;
    Ok((
        out.status.code().unwrap_or(-1),
        String::from_utf8_lossy(&out.stdout).into_owned(),
        String::from_utf8_lossy(&out.stderr).into_owned(),
    ))
}

fn run_simple(prog: &str, args: &[&str]) -> Result<(i32, String, String), String> {
    let out = Command::new(prog)
        .args(args)
        .output()
        .map_err(|e| format!("spawn {prog}: {e}"))?;
    Ok((
        out.status.code().unwrap_or(-1),
        String::from_utf8_lossy(&out.stdout).into_owned(),
        String::from_utf8_lossy(&out.stderr).into_owned(),
    ))
}

fn clamp(s: String) -> String {
    if s.len() <= MAX_OUTPUT {
        return s;
    }
    // Keep the tail (where results/errors usually are), advancing to a UTF-8
    // char boundary so slicing never panics.
    let mut i = s.len() - MAX_OUTPUT;
    while i < s.len() && !s.is_char_boundary(i) {
        i += 1;
    }
    format!("...[truncated]...\n{}", &s[i..])
}
