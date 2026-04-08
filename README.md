# Nougat

HTTP service that runs your **LLM CLI** (Anthropic **Claude Code** by default, or **Gemini** and others) behind two APIs:

- **Native** — async jobs with `POST /run-task` and polling.
- **OpenAI-compatible** — `POST /v1/chat/completions` and `GET /v1/models` for tools like OpenCode, Kilo Code, etc.

Jobs and per–API-key **usage stats** live in [BoltDB](https://github.com/etcd-io/bbolt); optional JSON logs and a plain **HTTP access log** are written under `logs/`.

## Requirements

- **Go 1.22+** (see `go.mod`)
- A **`claude`** binary on `PATH`, or configure **`NOUGAT_LLM_BIN`** / **`NOUGAT_LLM_EXTRA`** (see [LLM backend](#llm-backend-claude--gemini) and `./nougat -help`)

## Build

Stamp **version**, **build number**, and **build time** into the binary (recommended):

```bash
make build
./nougat -version
```

- **VERSION** — `git describe --tags --always --dirty` (tag `v*` for semver-style strings)
- **BUILD_NUMBER** — `git rev-list --count HEAD` (falls back to `0` without git)
- **BUILD_TIME** — UTC timestamp at link time

Overrides: `make build VERSION=1.4.0 BUILD_NUMBER=140`.

Plain `go build -o nougat .` leaves defaults (`0.0.0-dev`, `0`, `unknown`). Manual stamping:

```bash
go build -trimpath -o nougat \
  -ldflags "-X 'main.version=1.0.0' -X 'main.buildNumber=42' -X 'main.buildTime=2026-04-08T12:00:00Z'" .
```

`GET /health` returns `version`, `build_number`, and `build_time` as JSON.

## Run

```bash
./nougat
```

The process **blocks in the foreground** serving HTTP until **Ctrl+C**. That is expected, not a hang. Default listen address is **`:8080`**; override with **`-addr`** or **`PORT`**.

```bash
./nougat -addr :3000
PORT=3000 ./nougat
```

### Startup output

- Status and banner go to **stderr**; HTTP **access lines** go to **stdout** (when a TTY is detected).
- You should see a **plain first line** (`nougat version … · build … · built …`), then **`nougat: opening database …/jobs.db …`**, then the banner and tables. Colors are used only when stderr is a **terminal** and **`NO_COLOR`** is unset (see `golang.org/x/term`).
- If you see **nothing**, try an external terminal or set **`TERM=xterm-256color`**. If startup stops after “opening database…”, another process may hold **`jobs.db`** — exit the other `nougat` or release the file lock.

### CLI quick reference

| Flag / command | Purpose |
| --- | --- |
| `-help`, `help` | Full help text |
| `-version` | Print version, build number, build time |
| `-config <path>` | Config file (default `./nougat.config.json` or `NOUGAT_CONFIG`) |
| `-gen-key-label <name>` | Create **`sk-nougat-…`**, append to config, print secret, exit (implies key generation) |
| `-gen-api-key` | Same without a label |

## HTTP API

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/run-task` | Enqueue task; returns `job_id` |
| `GET` | `/jobs/{id}` | Job status, stdout, stderr |
| `POST` | `/v1/chat/completions` | OpenAI-style chat (blocks until the CLI finishes) |
| `GET` | `/v1/models` | Model list |
| `GET` | `/dashboard` | HTML UI: paste API key for usage + history |
| `GET` | `/dashboard/api/summary` | JSON totals (Bearer key required) |
| `GET` | `/dashboard/api/history` | JSON events (`?limit=100`) |
| `GET` | `/health` | Liveness + build metadata |

**OpenAI clients:** base URL should be `http://<host>:<port>/v1` (no path beyond `/v1`).

### API keys and config

- Copy [`nougat.config.example.json`](nougat.config.example.json) to **`nougat.config.json`** (gitignored by default) or set **`NOUGAT_CONFIG`** / **`-config`**.
- Generate a key:

  ```bash
  ./nougat -gen-key-label "my-laptop"
  ```

**Auth**

- If **`api_keys`** is non-empty or **`NOUGAT_API_KEY`** is set, **`/v1/*`** requires **`Authorization: Bearer <secret>`**.
- Optional **`"auth": { "require_api_key": true }`** forces `/v1` auth even when no keys are listed yet.
- **`/dashboard/api/*`** always requires a **valid** configured key (stats are scoped to that key).
- **`NOUGAT_API_KEY`** is merged in memory as **`key_env`**.

**Stats** — BoltDB buckets **`UsageTotals`** / **`UsageHistory`**: rolling totals and up to **500** recent events per key (token counts for chat are estimates).

### Example: native task

```bash
curl -sS -X POST http://127.0.0.1:8080/run-task \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Say hello in one sentence.","working_directory":".","timeout":300}'
```

Poll `GET /jobs/{job_id}` until `status` is not `PENDING` / `RUNNING`.

### `POST /run-task` body

| Field | Required | Description |
| --- | --- | --- |
| `prompt` | yes | Sent to the LLM CLI |
| `working_directory` | no | Default `.` |
| `task_type` | no | Stored on the job |
| `timeout` | no | Seconds; default **300** |

## LLM backend (Claude / Gemini)

Invocation: **`$NOUGAT_LLM_BIN`** + optional **`$NOUGAT_LLM_EXTRA`** (space-split) + **prompt**.

| Variable | Meaning |
| --- | --- |
| `NOUGAT_LLM_BIN` | Executable (default `claude`) |
| `NOUGAT_LLM_EXTRA` | Unset → append `--prompt`. Set but **empty** → prompt is the only extra argument. Non-empty → split with `strings.Fields` and insert before the prompt. |

```bash
export NOUGAT_LLM_BIN=gemini
export NOUGAT_LLM_EXTRA=-p
./nougat
```

OpenAI path: **`NOUGAT_OPENAI_WORKDIR`**, **`NOUGAT_OPENAI_TIMEOUT_SEC`**, **`NOUGAT_OPENAI_MODELS`**, **`NOUGAT_OPENAI_MODEL_STRICT`** — see `./nougat -help`.

## Data on disk

| Path | Contents |
| --- | --- |
| `jobs.db` | Jobs + API usage buckets |
| `logs/` | Per-job JSON; `server-access.log` |
| `nougat.config.json` | API keys (keep private; use `-config` for another path) |

Ignored by `.gitignore` where noted.
