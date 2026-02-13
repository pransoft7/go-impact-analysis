# Downstream Impact Analysis for Go Modules

A tool that measures the real-world impact of changes to a Go module by running the test suites of its downstream dependents before and after a change.

---

## Architecture

```
┌──────────────┐      ┌──────────────────┐
│  config.json │─────▶│   Config Loader  │
└──────────────┘      │   (config.go)    │
                      └────────┬─────────┘
                               │
                               ▼
                      ┌──────────────────┐
                      │   Impact Runner  │
                      │   (impact.go)    │
                      └────────┬─────────┘
                               │
           ┌───────────────────┼───────────────────┐
           ▼                   ▼                   ▼
   ┌───────────────┐   ┌───────────────┐   ┌───────────────┐
   │   Baseline    │   │   Released    │   │   Modified    │
   │  (dep as-is)  │   │ (dep + v1.x) │   │ (dep + local) │
   └───────┬───────┘   └───────┬───────┘   └───────┬───────┘
           │                   │                   │
           ▼                   ▼                   ▼
       go test             go test             go test
           │                   │                   │
           └───────────────────┼───────────────────┘
                               ▼
                      ┌──────────────────┐
                      │   Classification │
                      │  UNCHANGED /     │
                      │  REGRESSION /    │
                      │  IMPROVEMENT /   │
                      │  UNCHANGED-FAIL  │
                      └──────────────────┘
```

### Three-Way Comparison

For each downstream dependent the tool runs three test passes:

| Pass | Description |
|---:|:---|
| **Baseline** | The dependent's test suite in its original state — no module replacements. |
| **Released** | Test suite after replacing the target module with the latest **released** version (shallow-cloned from the tag). |
| **Modified** | Test suite after replacing the target module with the **locally-modified** version (your working copy). |

The *Released* vs *Modified* comparison then yields one of four outcomes:

| Released | Modified | Outcome |
|:---:|:---:|:---|
| ✅ PASS | ✅ PASS | **UNCHANGED** — Your change does not break this dependent. |
| ✅ PASS | ❌ FAIL | **REGRESSION** — Your change introduces a failure. |
| ❌ FAIL | ✅ PASS | **IMPROVEMENT** — Your change fixes a previously-failing dependent. |
| ❌ FAIL | ❌ FAIL | **UNCHANGED-FAIL** — The dependent was already failing; your change has no additional impact. |

---

## Getting Started

### Prerequisites

- **Go 1.24 or above+**

### Build

```bash
git clone https://github.com/<your-username>/lfx-otel-prototype.git
cd lfx-otel-prototype
go build -o impact .
```

### Run

```bash
./impact config.json
```

---

## Configuration

The tool is driven by a single JSON file. Here is the included example (`config.json`):

```json
{
    "target": {
        "repo_url": "https://github.com/open-telemetry/opentelemetry-go.git",
        "module_prefix": "go.opentelemetry.io/otel/sdk",
        "released_ref": "v1.40.0",
        "modified_local_path": "/absolute/path/to/your/modified/otel"
    },
    "dependents": [
        {
            "repo_url": "https://github.com/open-telemetry/opentelemetry-go-contrib.git",
            "module_path": "instrumentation/net/http/otelhttp",
            "ref": "main"
        }
    ]
}
```

### Reference

#### `target` — The module under test

| Field | Description |
|:---|:---|
| `repo_url` | Git URL of the target module's repository. |
| `module_prefix` | Go module path prefix used to identify which dependencies to replace (e.g. `go.opentelemetry.io/otel/sdk`). |
| `released_ref` | Git tag or branch representing the current stable/released version. |
| `modified_local_path` | Absolute path to the locally modified copy of the target repo. |

#### `dependents[]` — Downstream consumers to test against

| Field | Description |
|:---|:---|
| `repo_url` | Git URL of the dependent's repository. |
| `module_path` | Relative path within the repo to the Go module directory (e.g. `instrumentation/net/http/otelhttp`). |
| `ref` | Git branch or tag to check out. |

---

## Example Output

```
Workspace: /tmp/otel-impact-123456

================================================
Dependent: https://github.com/open-telemetry/opentelemetry-go-contrib.git
Module   : instrumentation/net/http/otelhttp
================================================

--- Baseline ---
✔ PASS
Duration: 12s

--- Released ---
  replace go.opentelemetry.io/otel/sdk => /tmp/otel-impact-123456/released/sdk
✔ PASS
Duration: 14s

--- Modified ---
  replace go.opentelemetry.io/otel/sdk => /home/user/otel/modified/sdk
✘ FAIL
Duration: 11s

Summary:
Baseline : PASS
Released : PASS
Modified : FAIL
Outcome  : REGRESSION
```

---

## Project Structure

```
.
├── main.go          # CLI entrypoint — parses args, loads config, runs impact analysis
├── config.go        # Config types & JSON loader
├── config.json      # Example configuration
├── impact.go        # Core logic — cloning, module replacement, test execution, classification
└── go.mod
```

---

## Roadmap

This prototype demonstrates the core workflow. The full LFX mentorship project aims to extend it with:

- [ ] **Automatic dependent discovery** — Fetch all publicly-listed reverse dependencies from the Go module proxy / `pkg.go.dev` instead of manually specifying them.
- [ ] **Parallel test execution** — Run dependents concurrently with configurable parallelism.
- [ ] **Structured result reporting** — Output machine-readable JSON/CSV reports with per-package pass/fail data and duration statistics.
- [ ] **Statistical summary & diff** — Compare two full runs and surface aggregated metrics (pass-rate delta, newly broken packages, duration regressions).
- [ ] **Sandboxed execution** — Run test suites inside containers to isolate network access, environment variables, system resources, and the Go build/module cache from the host and from each other.
- [ ] **CI/CD integration** — GitHub Action / CLI flag to run as part of a PR workflow and post results as a check or comment.
- [ ] **Caching & incremental runs** — Skip dependents whose relevant dependency graph hasn't changed between runs.

---

## Non-Goals

The following are explicitly **out of scope** for this tool:

- **Root-cause analysis** — The tool classifies outcomes (pass/fail) but does not diagnose *why* a test failed. Investigating failures remains a manual step for the maintainer.
- **Flaky test handling** — Each test run is a single atomic pass/fail. There is no retry logic, statistical flake detection, or quarantine mechanism.
- **Timeout & build-failure handling** — Dependents that fail to build or hang indefinitely are not yet handled gracefully; the tool currently blocks until completion.
- **Bounded execution** — After automatic dependent discovery, there is no mechanism to cap the run to the top-N most important dependents (e.g. prioritising CNCF projects) to avoid unbounded compute.
- **Non-Go dependents** — Only Go modules are supported. Dependents in other languages are out of scope.
- **External service dependencies** — Tests that depend on external infrastructure (databases, APIs, message brokers) may produce false negatives since those services are unavailable during the run. This can create noise in results and is not accounted for.
