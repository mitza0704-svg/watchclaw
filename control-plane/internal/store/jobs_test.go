package store

import (
	"context"
	"testing"
)

// TestJobLifecycle covers enqueue -> claim (pending->running, once) -> complete.
func TestJobLifecycle(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	id, err := s.EnqueueJob(ctx, Job{Hostname: "HOST-A", Kind: "script", Shell: "powershell", Command: "echo hi", CreatedBy: "tester"})
	if err != nil || id == 0 {
		t.Fatalf("enqueue: id=%d err=%v", id, err)
	}

	// A job for another host must not be claimed by HOST-A.
	if _, err := s.EnqueueJob(ctx, Job{Hostname: "HOST-B", Kind: "script", Shell: "sh", Command: "true"}); err != nil {
		t.Fatalf("enqueue B: %v", err)
	}

	claimed, err := s.ClaimJobs(ctx, "HOST-A")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != id || claimed[0].Command != "echo hi" {
		t.Fatalf("claim returned %+v", claimed)
	}

	// Re-claim must return nothing — a running job is not handed out twice.
	again, err := s.ClaimJobs(ctx, "HOST-A")
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("expected no re-claim, got %d", len(again))
	}

	if err := s.CompleteJob(ctx, id, JobResult{ExitCode: 0, Stdout: "hi", Stderr: "", Status: "done"}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	jobs, err := s.ListJobs(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found *Job
	for i := range jobs {
		if jobs[i].ID == id {
			found = &jobs[i]
		}
	}
	if found == nil || found.Status != "done" || found.Stdout != "hi" {
		t.Fatalf("completed job wrong: %+v", found)
	}

	// Completing an unknown job id must error.
	if err := s.CompleteJob(ctx, 99999, JobResult{Status: "done"}); err == nil {
		t.Fatal("expected error completing unknown job")
	}
}
