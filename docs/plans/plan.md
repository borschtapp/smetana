# Plan: Semi-Automated Catalog Maintenance & Suggestions

A backend system that **detects** data-quality problems and **proposes** fixes as a
queue of suggestions. A moderator (any authenticated user) reviews each one
Google-Photos style ("Merge *Tomatoes* and *Tomat*?") and accepts or rejects it.
High-confidence suggestions may auto-apply, with a revert path.

---

## 1. Decisions (confirmed)

| Topic | Decision | Consequence |
|-------|----------|-------------|
| Moderation access | **Any authenticated user** (community-wiki model) | No new role/migration on `User`. Global suggestions are visible & actionable by everyone; household suggestions filtered by token's `HouseholdID`. |
| LLM | **Interface now, OpenAI-compatible provider later** | Ship deterministic detectors first. Define `SuggestionProvider`; inject `nil` until implemented. Detectors degrade to algorithm-only. |
| Application | **Auto-apply above a per-type confidence threshold** | Needs a stored revert snapshot + `Undo`. Below-threshold → manual review queue. |
| Prices | **Framework now, source pluggable later** | `food_price` detector flags gaps; a `PriceSource` interface starts with manual/copy-from-similar; external scraping is a future provider. |

### Why these matter to the architecture
Because there is **no admin gate** and the catalog (`food`/`unit`/`taxonomy`) is a
single globally-shared dataset, auto-apply is only safe if every change is
**reversible**. The schema therefore stores a `RevertPayload` captured at apply
time, and every `Applier` implements `Undo`.

---

## 2. What already exists (reuse, don't rebuild)

The data model was clearly built with this feature in mind:

- **Alias/merge pointers**: `Food.CanonicalFoodID`, `Unit.CanonicalUnitID`,
  `Taxonomy.CanonicalID`.
- **Merge mechanics**: `FoodRepository.Merge` and `UnitRepository.Merge`
  (`../../internal/repositories/food.go`, `unit.go`) reassign
  `RecipeIngredient.FoodID`/`UnitID`, `ShoppingItem`, `FoodPrice`, conversions,
  and set the canonical pointer — **while leaving `RecipeIngredient.Name`/`RawText`
  untouched**. This is exactly "change the FoodID but keep the original text."
- **Auto-follow on import**: `resolveAlias` in both `FindOrCreate` methods means
  once an alias exists, future imports of "Tomat" resolve to canonical "Tomato".
- **Gap detection already written**: `RecipeService.EstimatePrice`
  (`internal/services/recipe.go:316`) classifies ingredients as `missing_price`
  and `incompatible_unit` — the source of truth for two detectors below.
- **Unit math**: `Unit.Convertible()`, `UnitService.Convert`,
  `ConvertWithConversions`, `BestUnit`.
- **Background jobs**: `scheduler.Job` interface + `jobs.FeedFetchJob` pattern +
  `SchedulerLog` for run tracking.
- **DI**: single wiring point `internal/routes/api.go:RegisterApiRoutes`.

**Gap**: `TaxonomyRepository` has **no `Merge`** — must be added for `taxonomy_merge`.

---

## 3. Domain model — `../../domain/suggestion.go`

One generic, polymorphic table. New detector = new `Type` + payload, **no migration**.
The trade-off (payload not FK-enforced) is acceptable because Appliers
**re-validate against live data at apply time** (the candidate may be stale by then).

```go
type SuggestionType string
const (
    SuggestionFoodMerge       SuggestionType = "food_merge"
    SuggestionUnitMerge       SuggestionType = "unit_merge"
    SuggestionTaxonomyMerge   SuggestionType = "taxonomy_merge"
    SuggestionFoodDefaultUnit SuggestionType = "food_default_unit"
    SuggestionFoodConversion  SuggestionType = "food_conversion"
    SuggestionUnitBaseFactor  SuggestionType = "unit_base_factor"
    SuggestionFoodPantry      SuggestionType = "food_pantry"
    SuggestionFoodTaxonomy    SuggestionType = "food_taxonomy"
    SuggestionIngredientFood  SuggestionType = "ingredient_food_link"
    SuggestionFoodPrice       SuggestionType = "food_price"
    // later: taxonomy_parent, recipe_taxonomy, recipe_language, food_image
)

type SuggestionStatus string
const (
    StatusPending     SuggestionStatus = "pending"
    StatusAccepted    SuggestionStatus = "accepted"     // applied after manual approval
    StatusAutoApplied SuggestionStatus = "auto_applied" // applied automatically (>= threshold)
    StatusRejected    SuggestionStatus = "rejected"     // declined; kept for dedup
    StatusReverted    SuggestionStatus = "reverted"
    StatusSuperseded  SuggestionStatus = "superseded"   // entity changed before action
    StatusFailed      SuggestionStatus = "failed"       // applier errored; retryable
)

type Suggestion struct {
    ID          uuid.UUID        `gorm:"type:char(36);primaryKey" json:"id"`
    Type        SuggestionType   `gorm:"index" json:"type"`
    Status      SuggestionStatus `gorm:"index;default:'pending'" json:"status"`
    Source      string           `json:"source"`            // "algorithm" | "llm" | "external"
    Confidence  float64          `json:"confidence"`        // 0..1
    HouseholdID *uuid.UUID       `gorm:"type:char(36);index" json:"household_id,omitempty"` // nil = global

    // Subject columns: indexed so we can list "suggestions about food X" without parsing JSON.
    SubjectType string     `gorm:"index:idx_sugg_subject" json:"subject_type"` // food|unit|taxonomy|ingredient|recipe
    SubjectID   uuid.UUID  `gorm:"type:char(36);index:idx_sugg_subject" json:"subject_id"`
    TargetID    *uuid.UUID `gorm:"type:char(36);index" json:"target_id,omitempty"` // e.g. "keep" food in a merge

    Payload       datatypes.JSON `json:"payload"`                 // type-specific proposal (see below)
    RevertPayload datatypes.JSON `json:"-"`                       // snapshot captured at apply time
    Reason        string         `json:"reason"`                  // human-readable explanation
    DedupKey      string         `gorm:"uniqueIndex" json:"-"`    // deterministic; prevents re-suggesting

    ResolvedBy *uuid.UUID `gorm:"type:char(36)" json:"resolved_by,omitempty"`
    ResolvedAt *time.Time `json:"resolved_at,omitempty"`
    Error      *string    `json:"error,omitempty"`
    Updated    time.Time  `gorm:"autoUpdateTime" json:"-"`
    Created    time.Time  `gorm:"autoCreateTime" json:"created"`
}
```

**For display**, the list endpoint hydrates referenced entities (food names, unit
labels) via batch `ByIDs` lookups rather than GORM associations, since the
subject is polymorphic. Returned as a `view` object alongside each suggestion.

### DedupKey (idempotent re-scans)
A deterministic string per logical proposal, e.g.:
- merges: `food_merge:<minID>:<maxID>` (order-independent so A↔B isn't suggested twice)
- field fills: `food_default_unit:<foodID>`
- household price: `food_price:<householdID>:<foodID>`

`uniqueIndex` is portable across SQLite/MySQL/Postgres (partial indexes are not).
Re-scan logic: if a row with the key **already exists in any state, skip** — this
naturally stops re-suggesting something already **rejected**. Exception: a `failed`
row is reset to `pending` for retry. (Household `food_price` may want a
time-bucketed key so a gap can re-surface after N days — noted, not in v1.)

### Migration
Add `&domain.Suggestion{}` to `internal/database/connect.go:Migrate`.
Requires `gorm.io/datatypes` (already transitively present via GORM; add to `../../go.mod` if not).

---

## 4. Detector / Applier architecture

New package `../../internal/suggestions` (detectors + registry); service in
`../../internal/services/suggestion.go`; repo in `../../internal/repositories/suggestion.go`.

```go
type Scope int
const (ScopeGlobal Scope = iota; ScopeHousehold)

// Detector mines the DB for candidate problems.
// `since` is the reconciliation watermark: a detector only needs to (re)examine
// entities whose `Updated > since`. It still compares those against the *full*
// catalog (e.g. a new food vs. all existing foods for merge detection).
type Detector interface {
    Type()  domain.SuggestionType
    Scope() Scope
    Detect(ctx context.Context, since time.Time, householdID *uuid.UUID) ([]*domain.Suggestion, error)
}

// Applier enacts an accepted suggestion and can reverse it.
type Applier interface {
    Type() domain.SuggestionType
    // Apply mutates the DB and returns a revert snapshot.
    Apply(ctx context.Context, s *domain.Suggestion) (revert []byte, err error)
    Undo(ctx context.Context, s *domain.Suggestion) error
    // AutoApplyThreshold: confidence >= this auto-applies; return >1 to disable.
    AutoApplyThreshold() float64
}
```

A `Registry` maps `Type → {Detector, Applier}`. Detectors and appliers are thin —
they delegate to **existing** services/repos (`FoodService.Merge`,
`FoodService.CreateConversion`, `FoodService.Update`, etc.).

### `SuggestionService` (orchestrator)
- `Reconcile(ctx, scope, householdID, types...)` — for each registered detector in
  scope: read its `DetectorCursor.Watermark` → `Detect(ctx, watermark, householdID)`
  → upsert results by `DedupKey` (skip existing per rules above) → if
  `confidence >= AutoApplyThreshold()` then `apply()` immediately with
  `Status=auto_applied`, else persist `Status=pending` → on success, advance the
  cursor (skip advancing on partial failure). Cheap detectors run inline; detectors
  needing external calls emit an `ExternalTask` instead of calling out synchronously.
- `Accept(ctx, id, userID)` — load applier → re-validate → `Apply` → store
  `RevertPayload`, `Status=accepted`, `ResolvedBy/At`. On error → `Status=failed`, `Error`.
- `Reject(ctx, id, userID)` — `Status=rejected` (kept for dedup).
- `Revert(ctx, id, userID)` — `Undo` → `Status=reverted`.
- `List/Get/Stats`.

All `Apply`/`Undo` run inside a DB transaction (reuse `RecipeRepository.Transaction`
pattern; add equivalents where needed).

### Processing model — watermark reconciliation (not an event queue)

Detection is **level-triggered**, not edge-triggered. The import path does **not**
enqueue analysis work; it just writes entities (every entity already has
`Updated time.Time` with `autoUpdateTime`). A periodic reconcile loop then compares
*actual* state against *desired* state and fixes the gap. Missed writes self-heal
on the next sweep — there is no event to lose, and no enqueue hook to maintain.

State is a tiny **cursor per detector** (N rows total), not a task table:

```go
type DetectorCursor struct {
    DetectorType string    `gorm:"primaryKey"` // e.g. "food_merge"
    HouseholdID  *uuid.UUID `gorm:"primaryKey"` // nil for global detectors; per-household otherwise
    Watermark    time.Time  // max Updated successfully processed so far
}
```

Each sweep of a detector:
1. Read its `Watermark` (zero-value on first run / when a new detector is added → full backfill once).
2. `SELECT ... WHERE updated > :watermark` to get changed entities (the *work set*).
3. Run the detector over the work set (comparing against the full catalog where needed); upsert resulting suggestions by `DedupKey`.
4. On full success, advance `Watermark` to the max `Updated` seen. **On partial failure, do not advance** — the next sweep reprocesses safely because suggestion writes are idempotent (dedup'd).

Why this beats an event/task queue for detection:
- **Import stays untouched** — no transactional enqueue, no coupling. A burst feed
  sync that touches "tomato" 200× still yields one food row processed once.
- **Structural dedup + free debounce** — the sweep sees only each row's final state.
- **Self-healing** — a food created by any future code path is caught with no extra wiring.
- **Minimal state** — N cursor rows vs. a queue table with its own lifecycle.

Runs as a `SuggestionReconcileJob` (mirrors `jobs.FeedFetchJob`, registered in
`api.go` with `RECONCILE_INTERVAL`, logs to `SchedulerLog`). Global detectors run
once per sweep; household detectors loop over households (each with its own cursor row).

**Refinement (later, if needed):** add a `content_hash` column so a detector skips a
row whose relevant fields are unchanged, avoiding re-work on irrelevant edits.

### External-call queue (the *only* edge-triggered piece)

Cheap, deterministic detectors run inline in the reconcile sweep. But detectors that
make **external, failure-prone, rate-limited calls** — LLM ranking and supermarket
price fetches — must not run synchronously in the sweep. The reconcile loop instead
emits a small durable task per such call, drained by a worker with retry/backoff/throttle:

```go
type ExternalTask struct {
    ID        uuid.UUID
    Kind      string     // "llm_rank_merge" | "fetch_price"
    Payload   datatypes.JSON
    DedupKey  string     `gorm:"uniqueIndex"`
    Status    string     // pending | running | done | failed
    Attempts  int
    RunAfter  time.Time  // backoff / future scheduling
    Created, Updated time.Time
}
```

This is the one place queue semantics (attempts, backoff, rate-limit) genuinely pay
for themselves. Everything else stays in the level-triggered reconcile loop.

### On-demand
`POST /suggestions/scan` triggers a reconcile sweep immediately (optionally a single
`?type=`), e.g. right after a manual import — same code path, just not waiting for the timer.

---

## 5. Detector catalog (the maintenance tasks)

Grouped by phase. Each lists: signal → payload → applier (existing method) → auto-apply stance.

### Phase 1 — deterministic, high value
| Type | Signal (algorithm) | Payload | Applier | Auto-apply |
|------|--------------------|---------|---------|-----------|
| `food_default_unit` | Most-frequent `UnitID` across a food's `RecipeIngredient`s where `Food.DefaultUnitID IS NULL` (pure SQL `GROUP BY`) | `{food_id, unit_id, usage_count}` | `FoodService.Update` | Yes — strong signal (e.g. ≥0.9 when one unit dominates) |
| `food_pantry` | Food name matches curated pantry set (salt, water, oil, pepper…) and `Pantry=false` | `{food_id, keyword}` | `FoodService.Update` | Yes (curated list = high confidence) |
| `unit_merge` | Trigram/Levenshtein on `Unit.Name`+`Slug`; same `imperial` & base dimension | `{keep_id, merge_id, similarity}` | `UnitService.Merge` | Threshold (e.g. exact-normalized = auto; fuzzy = review) |
| `food_merge` | Normalized-name similarity (lowercase, singular/plural, trigram) on canonical foods | `{keep_id, merge_id, keep_name, merge_name, similarity}` | `FoodService.Merge` | Review by default (most user-visible); auto only on exact normalized match |
| `ingredient_food_link` | `RecipeIngredient.FoodID IS NULL` & `Name` parseable (kapusta) | `{ingredient_id, food_id?, food_name}` | `RecipeRepository.UpdateIngredient` | Review |

`keep` vs `merge` choice: prefer the food/unit with more references, an image, a
default unit, or older `Created` — encode as the higher-quality target.

### Phase 2 — conversions, prices, taxonomy merge
| Type | Signal | Payload | Applier | Notes |
|------|--------|---------|---------|-------|
| `food_conversion` | Foods whose cost calc yields `incompatible_unit` (reuse `EstimatePrice` logic) — e.g. ingredient in *piece*, price in *kg* | `{food_id, unit_id, target_amount, target_unit_id}` | `FoodService.CreateConversion` | Seed from a typical-weight table / sibling foods; LLM later for "1 medium onion ≈150 g". Review. |
| `unit_base_factor` | `Unit.Convertible()==false` (`BaseFactor==0`) | `{unit_id, base_unit_id, base_factor}` | extend `UnitService.Update` to set base fields | Seed common units (tbsp=15 ml, cup=240 ml). Review. |
| `food_price` (household) | Food used by household recipes/shopping lists with no recent `FoodPrice` | `{food_id, suggested_price?, currency, unit_id, amount, source}` | `FoodService.RecordPrice` | `PriceSource` interface; v1 source = copy-from-similar-food / manual confirm. Review. |
| `taxonomy_merge` | Similarity within same `Taxonomy.Type` | `{keep_id, merge_id}` | **NEW** `TaxonomyRepository.Merge` | Must add Merge: reassign `recipe_taxonomies`/`food_taxonomies`/`unit_taxonomies`, set `CanonicalID`. |

### Phase 3 — classification (LLM-leaning, lower priority)
`food_taxonomy` (categorize uncategorized foods), `taxonomy_parent` (hierarchy),
`recipe_taxonomy` (cuisine/category), `recipe_language` (detect from text),
`food_image` (source an image). All ship as algorithm/keyword stubs and improve
once the LLM provider lands.

---

## 6. LLM provider (designed, implemented later)

```go
// domain/suggestion.go
type SuggestionProvider interface {
    // RankMergeCandidates returns yes/no + confidence + rationale for fuzzy pairs.
    RankMergeCandidates(ctx, pairs []MergeCandidate) ([]MergeVerdict, error)
    // Classify assigns a label from a fixed set (taxonomy, language, ...).
    Classify(ctx, input string, labels []string) (label string, confidence float64, err error)
}
```

- Implementation in `internal/services/llm/` against an **OpenAI-compatible**
  `/chat/completions` endpoint (works with OpenAI, Azure, Ollama, vLLM, etc.).
  Config via env: `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`.
- Prompts request **structured JSON** (verdict + confidence) for safe parsing.
- Injected into detectors as an **optional** dependency; `nil` → detectors run
  algorithm-only. This keeps the provider fully isolated and the core shippable.
- Cost control: only borderline algorithmic candidates (e.g. similarity in a
  middle band) are sent to the LLM; obvious matches/non-matches stay deterministic.

---

## 7. API (`../../internal/handlers/api/suggestion.go`)

Follows existing conventions (`bindBody`, `types.UuidParam`, `types.GetSearchOptions`,
`types.ListResponse`, `middlewares.Protected()`). Wired in `api.go` after `foodGroup`.

```
GET    /suggestions            list; filters: type, status (default pending),
                               subject_type, subject_id, scope. Paginated.
                               Household-scoped rows filtered by token HouseholdID;
                               global rows visible to all.
GET    /suggestions/stats      counts by type & status (dashboard badges)
GET    /suggestions/:id        single, with hydrated subject view
POST   /suggestions/:id/accept apply now
POST   /suggestions/:id/reject decline (kept for dedup)
POST   /suggestions/:id/revert undo an applied/auto-applied suggestion
POST   /suggestions/accept     batch: { "ids": [...] }  ("accept all on screen")
POST   /suggestions/reject     batch
POST   /suggestions/scan       trigger detectors now; optional ?type=
```

All annotated with Swagger comments; run `swag init` after.

---

## 8. Files to add / change

**Add**
- `../../domain/suggestion.go` — `Suggestion` + `DetectorCursor` + `ExternalTask` models; `SuggestionRepository`/`SuggestionService` + `SuggestionProvider` interfaces.
- `../../internal/repositories/suggestion.go` — GORM impl (`db *gorm.DB`), upsert-by-dedupkey, list/filter, stats, cursor read/advance, external-task enqueue/claim.
- `../../internal/services/suggestion.go` — orchestrator (Reconcile/Accept/Reject/Revert).
- `../../internal/suggestions` — `registry.go`, `detector.go`/`applier.go` interfaces, one file per detector+applier.
- `../../internal/handlers/api/suggestion.go` — handlers.
- `../../internal/jobs/suggestion_reconcile.go` — watermark reconcile job (mirrors `feed_fetch.go`).
- `internal/jobs/external_task_worker.go` — drains `ExternalTask` with retry/backoff/throttle (Phase 6/8, when LLM/price sources land).
- (later) `internal/services/llm/` — OpenAI-compatible provider.

**Change**
- `../../internal/database/connect.go` — register `&domain.Suggestion{}`, `&domain.DetectorCursor{}`, `&domain.ExternalTask{}`.
- `../../internal/routes/api.go` — construct repo/service/detectors/handler, mount routes, register scan job.
- `../../domain/taxonomy.go` + `../../internal/repositories/taxonomy.go` — add `Merge(keepID, mergeID)`.
- `../../domain/unit.go` + `../../internal/services/unit.go` + repo — allow `Update` to set `base_unit_id`/`base_factor` (for `unit_base_factor` applier).
- `../../go.mod` — `gorm.io/datatypes` if not already present.
- `..` — regenerate via `swag init`.

---

## 9. Testing
- **Detectors**: table-driven tests over seeded fixtures asserting the candidates
  (and `DedupKey` idempotency: a second scan creates nothing new).
- **Appliers**: apply → assert DB state (e.g. ingredient `FoodID` reassigned, text
  preserved) → `Undo` → assert original state restored.
- **Service**: auto-apply threshold routing; reject keeps dedup; failed→retry.
- Follow existing `_test.go` + `mocks_test.go` patterns in `../../internal/services`.

---

## 10. Build order
1. Domain model (`Suggestion` + `DetectorCursor`) + migration + repo + bare service +
   API (list/accept/reject) with **one** detector (`food_default_unit`, pure SQL,
   safe, auto-appliable) driven by a watermark reconcile sweep — end-to-end.
2. Revert path + `RevertPayload` + auto-apply threshold routing.
3. `SuggestionReconcileJob` (scheduled) + `POST /suggestions/scan` (on-demand sweep).
4. Merge detectors (`unit_merge`, `food_merge`) reusing existing `Merge`.
5. `TaxonomyRepository.Merge` + `taxonomy_merge`.
6. `food_conversion`, `unit_base_factor`, `ingredient_food_link`.
7. `ExternalTask` queue + worker; `food_price` + `PriceSource` interface (manual/copy source).
8. LLM provider (`SuggestionProvider`, OpenAI-compatible) + classification detectors (Phase 3), routed through the `ExternalTask` queue.

---

## 11. Open questions for later
- **Household price re-surfacing**: should resolved `food_price` gaps re-suggest
  after N days? (time-bucketed dedup key) — deferred to v1 feedback.
- **Merge undo cost**: reverting a merge restores reassigned rows from the
  snapshot; if the canonical food was edited post-merge, undo restores
  associations only, not field edits. Acceptable? Confirm during Phase 2.
- **Abuse**: with open moderation, do we want an audit trail of who
  accepted/reverted what? (`ResolvedBy` covers accept; consider a small audit log
  if churn becomes a problem.)
