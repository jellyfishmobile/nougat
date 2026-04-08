# Nougat

HTTP API that queues **tasks** and runs your **LLM CLI** (Anthropic Claude by default, or Gemini and others via env) for each job. Captures stdout, stderr, and exit status in [BoltDB](https://github.com/etcd-io/bbolt) and optional JSON logs on disk.

## Requirements

- Go **1.22+**
- A working **`claude`** CLI (Claude Code) on `PATH`, or another CLI configured with `NOUGAT_LLM_*` (see help)

## Build

Recommended — sets **version** (git tag/describe), **build number** (commit count), and **build time** (UTC) into the binary:

```bash
make build
./nougat -version
```

Override when tagging releases, e.g. `make build VERSION=1.4.0 BUILD_NUMBER=140`.

Plain `go build -o nougat .` leaves defaults (`0.0.0-dev`, `0`, `unknown`). To stamp manually:

```bash
go build -trimpath -o nougat -ldflags "-X 'main.version=1.0.0' -X 'main.buildNumber=42' -X 'main.buildTime=2026-04-08T12:00:00Z'" .
```

`GET /health` includes `version`, `build_number`, and `build_time` in the JSON response.

## Run

```bash
./nougat
```

The binary **stays in the foreground** running an HTTP server (this is normal, not a hang). Stop with **Ctrl+C**. It listens on **`:8080`** by default, or set **`-addr`** / **`PORT`**. On startup, check **stderr** for the `curl …/health` hint if your terminal buffers banner output.

```bash
./nougat -addr :3000
PORT=3000 ./nougat
```

Help (ANSI sections in a capable terminal):

```bash
./nougat -help
./nougat help
```

## HTTP API

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/run-task` | Accept a task (JSON body, returns `job_id`) |
| `GET` | `/jobs/{id}` | Job status and output |
| `POST` | `/v1/chat/completions` | OpenAI-compatible chat (blocks until LLM exits) |
| `GET` | `/v1/models` | Model list |
| `GET` | `/dashboard` | HTML UI: paste your API key to view **usage** and **history** |
| `GET` | `/dashboard/api/summary` | JSON totals for the Bearer key |
| `GET` | `/dashboard/api/history` | JSON recent requests (optional `?limit=100`) |
| `GET` | `/health` | Liveness JSON |

### API keys & config

- Create **`nougat.config.json`** (see [`nougat.config.example.json`](nougat.config.example.json)) or set **`NOUGAT_CONFIG`**.
- Generate a key and append it to the config file:

  ```bash
  ./nougat -gen-key-label "my-laptop"
  # or: ./nougat -gen-api-key -gen-key-label "my-laptop"
  ```

  Prints a **`sk-nougat-…`** secret once; add the file to `.gitignore` (default).

- **`NOUGAT_API_KEY`** is still supported: merged as key id `key_env` so it validates like file-based keys.

**Auth behavior**

- If the config lists any `api_keys` or `NOUGAT_API_KEY` is set, **`/v1/*` requires** `Authorization: Bearer <secret>` on every request.
- Set **`"require_api_key": true`** under `auth` to force `/v1` auth even before you add keys (everything will 401 until keys exist).
- **`/dashboard/api/*` always requires** a Bearer token that matches a configured key (used to scope stats).

**Usage stats** are stored in the same BoltDB file as jobs (`UsageTotals` / `UsageHistory` buckets): totals plus the last **500** events per key (rough token estimates for chat completions).

### Example: enqueue a task

```bash
curl -sS -X POST http://127.0.0.1:8080/run-task \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Say hello in one sentence.","working_directory":".","timeout":300}'
```

Then poll `GET /jobs/{job_id}` until `status` is no longer `PENDING` / `RUNNING`.

### Request body (`POST /run-task`)

| Field | Required | Description |
| --- | --- | --- |
| `prompt` | yes | Passed to the LLM CLI |
| `working_directory` | no | Defaults to `.` |
| `task_type` | no | Stored on the job |
| `timeout` | no | Seconds; default **300** |

## LLM backend (Claude / Gemini)

Nougat runs: **`$NOUGAT_LLM_BIN` + extra args + prompt**.

| Variable | Meaning |
| --- | --- |
| `NOUGAT_LLM_BIN` | Executable (default: `claude`) |
| `NOUGAT_LLM_EXTRA` | Space-separated args **before** the prompt. **Unset** → adds `--prompt`. **Set but empty** → no extra args (prompt only). |

Examples:

```bash
# Gemini-style (-p before prompt)
export NOUGAT_LLM_BIN=gemini
export NOUGAT_LLM_EXTRA=-p
./nougat
```

Full detail: `./nougat -help`.

## Data on disk

- **`jobs.db`** — BoltDB: jobs plus per-API-key usage (`UsageTotals` / `UsageHistory`)
- **`logs/`** — Per-job JSON after completion; **`server-access.log`** for HTTP access lines
- **`nougat.config.json`** — API keys (gitignored by default); use **`-config`** to pick another path

These paths are ignored by git via `.gitignore` where applicable.

