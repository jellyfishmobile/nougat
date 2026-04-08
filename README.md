# Nougat

HTTP API that queues **tasks** and runs your **LLM CLI** (Anthropic Claude by default, or Gemini and others via env) for each job. Captures stdout, stderr, and exit status in [BoltDB](https://github.com/etcd-io/bbolt) and optional JSON logs on disk.

## Requirements

- Go **1.22+**
- A working **`claude`** CLI (Claude Code) on `PATH`, or another CLI configured with `NOUGAT_LLM_*` (see help)

## Build

```bash
go build -o nougat .
```

## Run

```bash
./nougat
```

Listens on **`:8080`** by default, or set **`-addr`** / **`PORT`**.

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
| `GET` | `/health` | Liveness JSON |

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

- **`jobs.db`** — BoltDB job store (working directory)
- **`logs/`** — Per-job JSON after completion; **`server-access.log`** for HTTP access lines

These paths are ignored by git via `.gitignore`.

