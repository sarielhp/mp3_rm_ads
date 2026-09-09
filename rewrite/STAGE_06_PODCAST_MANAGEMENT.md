# Stage 06: Podcast Management and Feed Engine

## 1. Goal & Rationale

Stage 06 consolidates all podcast lifecycle logic into `pkg/podcast`:
- RSS XML feed fetching, parsing, and multi-tier disk caching.
- Unified ID registry and lookup engine mapping Audiobookshelf, PodFetch, and directory paths.
- Download queue management, priority sorting, and JSON persistence.
- Download policy evaluation (latest N, duration cutoffs, release frequency analysis).
- Episode status resolution (`AdR` vs `NeedAdR`, audio metadata probing).
- Library maintenance (orphan cleanup, transcript auditing).

Previously spread across 24 root files (`pm_*.go`, `feed_cache.go`, `id_registry.go`, `download_queue.go`), this stage unifies them into a cohesive domain manager while eliminating all global package state.

---

## 2. Package Details

### 2.1 `pkg/podcast`

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `manager.go` | `pm_utils.go`, `pm_types.go` | Central `PodcastManager` coordinating feeds, downloads, backend sync, and processing |
| `feed_cache.go` | `feed_cache.go` | RSS XML parsing (`encoding/xml`), HTTP feed fetching with ETag/Last-Modified headers, disk caching |
| `id_registry.go` | `id_registry.go`, `podcast_id.go` | Unified registry mapping IDs, titles, directory paths, and backend episode IDs |
| `status.go` | `episode_status.go`, `status_report.go` | Sidecar `.status.json` reader/writer, calculating `AdR` (Ad Removed) vs `NeedAdR` status |
| `queue.go` | `download_queue.go`, `queue_persist.go` | Download and ad-removal priority queue, thread-safe mutations, JSON state persistence |
| `download.go` | `pm_download*.go` | HTTP audio enclosure downloading, resume capability, rate limiting, and work dir staging |
| `policy.go` | `pm_download_policy.go` | Download sync policy enforcement (count limits, duration ranges, frequency analysis) |
| `frequency.go` | `pm_frequency.go` | Statistical publication frequency analyzer (daily, weekly, biweekly, monthly) |
| `orphans.go` | `pm_clean_orphans.go` | Auditing local audio files against backend database; identifying and pruning orphaned episodes |
| `audit.go` | `audit_transcripts.go` | Library-wide audit of transcript files, cuts validity, and audio integrity |
| `process.go` | `rm_ads_podcast.go` | Orchestrates ad removal workflow for podcast episodes and updates backend records |

**State Decoupling:**
- `FeedCacheManager`, `DownloadQueue`, and `IDRegistry` are transformed from package-global singletons into fields on `type PodcastManager struct`.
- Multiple managers can be instantiated concurrently for tests without cross-contamination.

**Dependencies:**
- Imports `pkg/types`, `pkg/util`, `pkg/config`, `pkg/backend`, `pkg/audio`, `pkg/format`, `pkg/pipeline`.

---

## 3. Step-by-Step Implementation Plan

### Step 6.1: Define Domain Models & Manager
1. Define `type PodcastManager struct` in `pkg/podcast/manager.go`.
2. Move RSS XML types and feed caching from `feed_cache.go` to `pkg/podcast/feed_cache.go`.
3. Move ID registry into `pkg/podcast/id_registry.go`.

### Step 6.2: Move Status and Queues
1. Move episode status operations to `pkg/podcast/status.go`.
2. Move queue management and JSON persistence into `pkg/podcast/queue.go`.
3. Ensure queue mutations use `pkg/util.SyncMutex`.

### Step 6.3: Move Download and Policy Engine
1. Move episode downloaders to `pkg/podcast/download.go`.
2. Move policy evaluation to `pkg/podcast/policy.go`.
3. Move frequency analysis to `pkg/podcast/frequency.go`.
4. Move orphan cleanup and transcript audit to `pkg/podcast/orphans.go` and `audit.go`.
5. Move podcast ad removal coordinator to `pkg/podcast/process.go`.

### Step 6.4: Migrate Tests
1. Move all associated test files (`feed_cache_test.go`, `id_registry_test.go`, `episode_status_test.go`, `download_queue_test.go`, `pm_frequency_test.go`, etc.) into `pkg/podcast/`.
2. Bridge root callers to import `pkg/podcast`.
3. Verify line counts ($\le 80$ lines per function).

---

## 4. Verification & Quality Gates

```bash
# Verify podcast package
go test -v ./pkg/podcast/...

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
