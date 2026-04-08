package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

type server struct {
	db     *bolt.DB
	logDir string
}

func main() {
	addrFlag := flag.String("addr", "", `HTTP listen address (e.g. ":3000", "127.0.0.1:8080"). When set, overrides the PORT environment variable.`)
	configPathFlag := flag.String("config", "", `path to nougat.config.json (API keys). Default: NOUGAT_CONFIG or ./nougat.config.json`)
	genAPIKey := flag.Bool("gen-api-key", false, `generate sk-nougat-… key, append to config file, print secret, and exit`)
	genKeyLabel := flag.String("gen-key-label", "", `label for the new key (implies -gen-api-key when non-empty)`)
	showVersion := flag.Bool("version", false, `print version / build number / build time and exit`)
	flag.Usage = printCLIHelp
	if len(os.Args) > 1 && os.Args[1] == "help" {
		printCLIHelp()
		os.Exit(0)
	}
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	configPath := strings.TrimSpace(*configPathFlag)
	if configPath == "" {
		configPath = defaultConfigPath(wd)
	}
	genKeyLabelTrim := strings.TrimSpace(*genKeyLabel)
	doGenKey := *genAPIKey || genKeyLabelTrim != ""
	if doGenKey {
		if err := runGenAPIKey(configPath, genKeyLabelTrim); err != nil {
			log.Fatal(err)
		}
		return
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	cfg.mergeEnvAPIKey()

	dbPath := filepath.Join(wd, dbFile)
	logDir := filepath.Join(wd, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Fatal(err)
	}

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	addr := resolveListenAddr(*addrFlag)
	srv := &server{db: db, logDir: logDir}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /run-task", srv.handleRunTask)
	mux.HandleFunc("GET /jobs/{id}", srv.handleGetJob)
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /v1/chat/completions", srv.handleOpenAIChatCompletions)
	mux.HandleFunc("GET /v1/models", srv.handleOpenAIModels)
	mux.HandleFunc("GET /dashboard", srv.handleDashboardPage)
	mux.HandleFunc("GET /dashboard/api/summary", srv.handleDashboardSummary)
	mux.HandleFunc("GET /dashboard/api/history", srv.handleDashboardHistory)

	printWelcome()
	printServerStatus(db, addr, dbPath, logDir, configPath)
	printServerRunningHint(addr)

	handler := accessLogMiddleware(apiKeyMiddleware(cfg, mux), accessLogFile(logDir))
	log.Fatal(http.ListenAndServe(addr, handler))
}

// resolveListenAddr returns -addr when non-empty; otherwise PORT or default :8080.
func resolveListenAddr(addrFlag string) string {
	if a := strings.TrimSpace(addrFlag); a != "" {
		return normalizeListenAddr(a)
	}
	if p := os.Getenv("PORT"); p != "" {
		return normalizeListenAddr(p)
	}
	return ":8080"
}

// normalizeListenAddr accepts ":port", "host:port", or a numeric port only.
func normalizeListenAddr(p string) string {
	if len(p) > 0 && p[0] == ':' {
		return p
	}
	if strings.Contains(p, ":") {
		return p
	}
	return ":" + p
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":        "healthy",
		"version":       version,
		"build_number":  buildNumber,
		"build_time":    buildTime,
	})
}

func (s *server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}
	workDir := req.WorkingDirectory
	if workDir == "" {
		workDir = "."
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 300
	}

	jobID, err := newJobID()
	if err != nil {
		log.Printf("job id: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	job := &Job{
		JobResponse: JobResponse{
			JobID:            jobID,
			Status:           StatusPending,
			TaskType:         req.TaskType,
			WorkingDirectory: workDir,
			CreatedAt:        now,
		},
		Prompt: req.Prompt,
	}

	if err := putJob(s.db, job); err != nil {
		log.Printf("put job: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	runJobAsync(s.db, s.logDir, job, timeout)
	logJobLifecycle("QUEUED", jobID, job.Status, workDir)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

func (s *server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	job, err := getJob(s.db, id)
	if err != nil {
		if errors.Is(err, errJobNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		log.Printf("get job: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func newJobID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
