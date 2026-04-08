package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

var accessLogMu sync.Mutex

func accessLogMiddleware(next http.Handler, plainLogPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: 0}
		next.ServeHTTP(rec, r)
		code := rec.code
		if code == 0 {
			code = http.StatusOK
		}
		dur := time.Since(start)
		logHTTPAccess(r.Method, r.URL.Path, code, dur)
		if plainLogPath != "" {
			appendPlainAccessLog(plainLogPath, r.Method, r.URL.Path, code, dur)
		}
	})
}

func logHTTPAccess(method, path string, code int, dur time.Duration) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	codeStr := fmt.Sprintf("%d", code)
	if supportsANSI() {
		switch {
		case code >= 500:
			codeStr = "\033[31m" + codeStr + ansiReset
		case code >= 400:
			codeStr = "\033[33m" + codeStr + ansiReset
		case code >= 200 && code < 300:
			codeStr = ansiGreen + codeStr + ansiReset
		}
	}
	line := fmt.Sprintf("  %s %6s %-24s %s %8s\n", ts, method, trimPath(path, 24), codeStr, dur.Round(time.Microsecond))
	fmt.Fprint(os.Stdout, line)
}

func trimPath(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return "…" + s[len(s)-(max-3):]
}

func appendPlainAccessLog(path, method, pathEsc string, code int, dur time.Duration) {
	line := fmt.Sprintf("%s %s %s %d %s\n",
		time.Now().Format(time.RFC3339Nano), method, pathEsc, code, dur.Round(time.Microsecond))
	accessLogMu.Lock()
	defer accessLogMu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}

// plain access log file name under log dir
func accessLogFile(logDir string) string {
	if logDir == "" {
		return ""
	}
	return filepath.Join(logDir, "server-access.log")
}

// logJobLifecycle prints a single line to stdout for task queue visibility.
func logJobLifecycle(phase string, jobID string, status JobStatus, workDir string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	jobShort := jobID
	if len(jobShort) > 8 {
		jobShort = jobShort[:8] + "…"
	}
	if supportsANSI() {
		fmt.Fprintf(os.Stdout, "  %s%s%s [job] %s%-9s%s id=%s%s%s status=%s dir=%s\n",
			ansiGray, ts, ansiReset,
			ansiBold, phase, ansiReset,
			ansiDim, jobShort, ansiReset,
			status, workDir)
	} else {
		fmt.Fprintf(os.Stdout, "  %s [job] %-9s %s status=%s dir=%s\n", ts, phase, jobShort, status, workDir)
	}
}
