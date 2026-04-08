package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

func runJobAsync(db *bolt.DB, logDir string, job *Job, timeoutSec int) {
	go executeJob(db, logDir, job, timeoutSec)
}

func executeJob(db *bolt.DB, logDir string, job *Job, timeoutSec int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	job.Status = StatusRunning
	_ = putJob(db, job)
	logJobLifecycle("RUNNING", job.JobID, job.Status, job.WorkingDirectory)

	stdout, stderr, exitCode, status := runLLM(ctx, job.WorkingDirectory, job.Prompt)

	job.Stdout = stdout
	job.Stderr = stderr
	job.ExitCode = exitCode
	job.Status = status
	now := time.Now().UTC()
	job.CompletedAt = &now

	_ = putJob(db, job)
	logJobLifecycle("DONE", job.JobID, job.Status, job.WorkingDirectory)

	logPath := filepath.Join(logDir, job.JobID+".json")
	logEntry := JobLog{Job: *job}
	data, err := json.MarshalIndent(logEntry, "", "  ")
	if err == nil {
		_ = os.WriteFile(logPath, data, 0o644)
	}
}

func runLLM(ctx context.Context, workDir, prompt string) (stdout, stderr string, exitCode int, status JobStatus) {
	bin := os.Getenv("NOUGAT_LLM_BIN")
	if bin == "" {
		bin = "claude"
	}

	var argv []string
	argv = append(argv, bin)
	if extra, ok := os.LookupEnv("NOUGAT_LLM_EXTRA"); ok {
		if strings.TrimSpace(extra) != "" {
			argv = append(argv, strings.Fields(extra)...)
		}
	} else {
		argv = append(argv, "--prompt")
	}
	argv = append(argv, prompt)

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = workDir

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return stdout, stderr, exitCode, StatusTimeout
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout, stderr, exitErr.ExitCode(), StatusFailed
		}
		return stdout, stderr, -1, StatusFailed
	}

	return stdout, stderr, 0, StatusCompleted
}
