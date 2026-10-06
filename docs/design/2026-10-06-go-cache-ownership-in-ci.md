# Who owns the Go caches in CI

**Date:** 2026-10-06
**Status:** in force. Implemented in `.github/workflows/go-tests.yml`; §4 is the
measurement that decided it.
**Related:** the pattern is ported from the dotfiles checkout —
`.github/workflows/verify.yml` there is the current implementation and
`docs/design/2026-09-24-go-build-cache-ownership.md` is its reasoning. Beware
that `template/ci/verify.yml` and `template/ci/README.md` in the same checkout
predate that work: the template contains no `setup-go` step at all, so its Go
example is not the pattern.

---

## 1. The measurement

Run [37541795601](https://github.com/nilbot/nextdns-ip-updater/actions/runs/37541795601)
(commit `7fbafa8`), the workflow as it stood before this change:

- `Run tests` (`go test -v -race -coverprofile=coverage.out ./...`) took **18s**
  as a step, while the `go test` binary itself reported **7.055s**. The missing
  ~11s was compilation — race-instrumented std and this module's packages,
  rebuilt on every run.
- `Set up Go` logged `##[warning]Failed to restore: Cache service responded with
  400` and then `Cache is not found`. setup-go's own cache covers the go-build
  cache as well as the module cache, and it was restoring neither.
- The only working cache was the manual `actions/cache@v4` step:
  `Cache hit for: Linux-go-771a8852…`, 156 MB into `~/go/pkg/mod`. That path is
  the module cache alone and does nothing for `GOCACHE`.
- `GO_VERSION: "1.23"` resolved to `1.23.12`, which `go.mod`'s
  `toolchain go1.24.4` then overrode at run time: `go: downloading go1.24.4
  (linux/amd64)`, `go version go1.24.4`,
  `GOROOT=/home/runner/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.24.4.linux-amd64`.
  The toolchain setup-go installed was not the one that compiled, so the cache
  key named a version `GOCACHE` never saw.

## 2. Why that key could not repair itself

`setup-go` keys its cache on the files named in `cache-dependency-path`, restores
with **no restore-key**, and returns early from its save when the primary key
already exists. An entry is therefore frozen at whatever the first run to hold
that key put in `GOCACHE`, and no later run can grow it. Here the key defaulted
to `go.sum` — a file exactly one commit in this repository's history has ever
touched — so the first snapshot would have been permanent.

## 3. What changed

- `45b19aa` — adopt the pattern: `actions/setup-go@b7ad1dad… # v7.0.0` with an
  explicit `cache-dependency-path` over `go.sum` and
  `.github/go-build-cache-epoch`; the fail-closed epoch guard (`test -s` on that
  file, named via `$GITHUB_WORKSPACE`); `GO_VERSION: '1.24.4'`, the exact patch
  and the version `go.mod`'s toolchain line names, so nothing is downloaded at
  run time; `actions/checkout@3d3c42e5… # v7.0.1`; `permissions: contents: read`;
  `timeout-minutes: 15`. The manual `actions/cache@v4` step is gone — setup-go
  supersedes it, and it was the only tag-pinned action in the repository.
- `6065340` — cache the Go toolchain at
  `${{ runner.tool_cache }}/go/${{ env.GO_VERSION }}`, keyed on OS, architecture
  and version, placed **before** setup-go. Deliberately not keyed on the epoch:
  the toolchain is identical for every run on a platform, so sharing one entry
  is the point, and there is no hash to rotate.
- `7fbafa8` — docs-only pushes skip the workflow. See §5 for the condition that
  would revert it.

Note on the apparent contradiction: one `actions/cache` step was removed and
another added. The removed one was `@v4` on a mutable tag caching
`~/go/pkg/mod`, which setup-go already duplicates; the added one is SHA-pinned to
`55cc8345… # v6.1.0` and caches the tool cache, which setup-go does not.

## 4. What it measured

Run [37542634165](https://github.com/nilbot/nextdns-ip-updater/actions/runs/37542634165)
(`45b19aa`) and run [37543472529](https://github.com/nilbot/nextdns-ip-updater/actions/runs/37543472529)
(`6065340`). Attempt 1 of each is cold; attempt 2 is that same commit re-run via
`gh run rerun`, which restores what attempt 1 saved. All spans are measured the
same way: first step started to last step completed. All runs green.

| variant | `setup-go` step | `Run tests` step | job span |
|---|---|---|---|
| before any port (`37541795601`) | — | 18s | 51s |
| cache ownership, cold (`37542634165` a1) | 16s | 20s | 62s |
| cache ownership, warm (`37542634165` a2) | 12s | 7s | 25s |
| + toolchain cache, cold (`37543472529` a1) | 10s | 8s | 25s |
| + toolchain cache, warm (`37543472529` a2) | **3s** | 8s | **19s** |

What the warm toolchain attempt logged, which is the whole claim:

```
cache the Go toolchain   Cache hit for: go-toolchain-Linux-X64-1.24.4
cache the Go toolchain   Cache restored from key: go-toolchain-Linux-X64-1.24.4
actions/setup-go         Found in cache @ /opt/hostedtoolcache/go/1.24.4/x64
actions/setup-go         go version go1.24.4 linux/amd64
```

There is no `Successfully cached go to /opt/hostedtoolcache/…` line on that
attempt: setup-go found the tree the cache step had just restored instead of
installing Go.

**The cache step is not free, and the 9s is not the gain.** `setup-go` fell from
12s to 3s, but the toolchain cache step itself costs 3s on a hit (22:55:05 →
22:55:08). The end-to-end gain is the 6s in the span, not the 9s in the step. The
entry's size was not recorded; if the storage it occupies ever matters, read it
off the step's own log rather than estimating it. The cold rows in the table are not comparable to each
other: by `37543472529` the go-build cache was already warm from `37542634165`,
which is why its cold span looks better than the earlier cold one. Only
warm-versus-warm isolates the toolchain change.

The `go test` binary's own reported time never moved — 7.055s before, 7.027s
warm after. Everything saved was compilation.

## 5. What would revert it

- **The toolchain cache**, if setup-go stops finding the restored tree and
  installs Go anyway. The revert condition in the dotfiles note, and it has been
  tested once: it found it. If a future run logs `Successfully cached go to …`
  on a warm attempt, the step is dead weight and comes out.
- **The epoch**, if a rotation does not produce a working cache. Bumping
  `.github/go-build-cache-epoch` is now the *only* way to refresh the go-build
  entry: on a warm attempt setup-go logs `Cache hit occurred on the primary key`
  and saves nothing. Bump it whenever the cache needs re-seeding; this is the
  lever, not `go.sum`.
- **The docs-only `paths-ignore` on push**, if either of the two conditions it
  rests on stops holding: the first test that reads a non-Go file (the dotfiles
  gate refuses path filters for exactly this reason — its `makefile_test.go`
  reads the Makefile), or branch protection that makes a skipped check report
  "Expected" forever. `pull_request` is deliberately left unfiltered, so the
  second cannot bite today: `main` has no protection and no rulesets. Every
  `*_test.go` here was checked for `os.ReadFile`, `ioutil.ReadFile`, `os.Open`,
  `os.Stat`, `filepath.`, `VERSION`, `.json`, `.yaml` and `.conf`: no matches.
- **setup-go's cache**, if `Cache service responded with 400` returns. That
  warning is the diagnostic; it means the go-build cache is not being restored
  and `Run tests` will drift back toward 18s.

Not ported from the dotfiles implementation, and why: `GOWORK: off` (one module,
no `go.work`), per-module cache keys and the multi-leg matrix (one module, one
OS), the `gate` aggregator (nothing to attach it to without branch protection),
and the `workflow_dispatch` / `workflow_call` triggers (nothing calls this
workflow).

## 6. What this is worth

Job span 51s → 19s, and the `Run tests` step 18s → 8s. The two changes are
independent: the cache-ownership port fixes *what is cached*, the toolchain cache
removes *an install that has nothing to do with this repository's code*. Neither
depends on the other, and each has its own revert condition above.

The numbers are single samples on shared runners, and runner variance is real —
the dotfiles note records the same macOS leg at 39–72s across runs with identical
steps and caches. Treat the direction as established and the individual seconds
as approximate.
