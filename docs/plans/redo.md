I read the whole of `3cac56f` (the suggestion system) and compared it against `docs/plans/plan.md`, which is still untracked in the tree. Build and `go test ./internal/suggestions/... ./internal/services/...` both pass. Here's the retrospective.

## First, what I'd keep unchanged

These earned their place and I want to be explicit about them before the criticism:

- **One polymorphic table + indexed subject columns + JSON payload** (`domain/suggestion.go:41-48`). A new detector needs no migration, and every query the UI actually runs (`list suggestions about food X`) hits an index instead of parsing JSON. That's the correct split.
- **`DedupKey` as a deterministic unique index**, with the UUID pair ordered so A↔B and B↔A collapse (`merge_detect.go:79-80`). Idempotency is the hardest part of any detect loop and this got it right on the first try.
- **Watermark captured *before* the scan** (`services/suggestion.go:132`). The naive version — advance to `time.Now()` after scanning — silently drops every row written during the scan. Easy to get wrong, hard to notice.
- **Appliers re-validate against live data** rather than trusting the candidate (`food_default_unit.go:133`). The plan called this out as the reason the payload doesn't need FK enforcement, and the code actually honours it.
- **`detectMerges` as a generic config struct** — three detectors, one algorithm, abstracted *after* three concrete cases existed rather than before.

`★ Insight ─────────────────────────────────────`
The subject-columns-plus-payload pattern is worth naming: the *indexed* fields are the ones the system queries by (`subject_type`, `subject_id`, `target_id`, `status`), while the *unindexed* JSON holds what only the applier reads. Choosing `type:text` over Postgres `jsonb` is then a deliberate, correct trade — you gave up payload querying you'd already decided you never need.
`─────────────────────────────────────────────────`

---

## What I'd do differently, ranked

### 2. Detectors would depend on repository ports, not `*gorm.DB`

`internal/suggestions` is effectively a fourth layer that speaks SQL directly, which contradicts the repo's own boundary (AGENTS.md: repositories are the GORM layer) and undoes commit `4373a66` *"use Scopes and table names from structs"* — `food_default_unit.go:41-57` and `food_conversion.go:36-50` hardcode `food`, `units`, `recipe_ingredients`, `food_unit_conversions` as string literals. Three consequences:

- A table rename breaks detectors at runtime, not compile time.
- Detectors can't be tested without a real DB — which is why `testdb_test.go:16-18` exists and its own comment admits it's a copy of the repositories package's helper.
- Domain logic leaks and duplicates: `u.base_factor = 0` in `food_conversion.go:45` is `Unit.Convertible()` re-expressed as a magic literal in SQL.

I'd add the read side to the existing repositories (`FoodRepository.MergeCandidatesUpdatedSince`, `UnitRepository.NonConvertibleUsage`, …) or a single `domain.CatalogQueries` port, and have detectors take interfaces. The scan logic then becomes fakeable, and the SQL lives with the rest of the SQL.

### 3. Payloads would be typed

Every detector builds `map[string]any{…}` and every applier re-declares an anonymous struct to parse it back. The two sides can drift with zero compiler help. The clearest symptom is `food_conversion.go:97`: the applier requires `target_amount` and `target_unit_id`, which the *detector never writes* — by design, a human is supposed to fill them in. Nothing in the type system, the API, or Swagger says so; the user discovers it as an error string at accept time.

I'd write `domain/suggestion_payloads.go` with one struct per type and a `Validate() error`, so "needs human input before it can be accepted" becomes a declared status (`needs_input`) instead of a runtime surprise. Same for `SuggestionView.View map[string]any` (`domain/suggestion.go:93`) — it's the field the frontend renders and it's opaque to `swag`, so there's no generated contract for it.

### 4. Accept would claim the row before applying

`services/suggestion.go:71-80`: read → check `pending` → apply → write status. Two concurrent accepts both pass the check. The second `Merge` fails its `canonical_food_id IS NULL` guard, and the loser's write flips a **successfully applied** suggestion to `failed` with an error message. Under the plan's explicit "any authenticated user is a moderator" model, two people working the same queue is the expected case, not an edge case.

Fix is one repo method: `UPDATE suggestions SET status='applying' WHERE id=? AND status IN ('pending','failed')`, then use `RowsAffected` as the lock.

There's a smaller related bug in the same area: `repositories/suggestion.go:49` copies `existing.Status` (`failed`) onto the candidate, then line 57-65 resets the DB row to `pending` without updating the struct. So `runDetector`'s guard `c.Status == domain.StatusPending` (`services/suggestion.go:144`) never fires for a reset row — a previously-failed auto-apply candidate silently never retries.

### 5. The similarity scan needs a pre-filter, and `/scan` shouldn't be synchronous

`detectMerges` loads the entire non-aliased catalog and runs Levenshtein against every work-set row. On first run the watermark is zero, so the work set *is* the catalog — N² pairs. And `POST /suggestions/scan` (`handlers/api/suggestion.go:169`) runs that inline in an HTTP request, with no role gate, no timeout, and no interlock against the 6-hour scheduled sweep or against other app instances (the cursor table has no lease).

Three cheap things I'd have built in: a length pre-filter before computing distance (if `|len(a)-len(b)| > (1-threshold)·maxLen`, the score cannot clear the bar — that alone eliminates most pairs for free); bucketing by length band or leading rune; and `/scan` enqueueing onto the scheduler that already exists, returning 202.

### 6. `singularize` would not be a language-blind multilingual stemmer

`similarity.go:103-161` runs English, German, French, Italian and Ukrainian rules against every name in priority order, with no idea what language it's looking at — because `Food` has no language column. Concretely, `similarity.go:139` (`n>4`, ends in `e` → `a`) maps "orange"→"oranga" and "sauce"→"sauca"; `similarity.go:118` maps "raisin"→"raisi". It's *mostly* harmless because the distortion is symmetric across both sides of a comparison, but it means the function invents cross-language neighbours it can't justify, and the tests can only pin the cases someone thought of.

Two better routes: add `Language` to Food/Unit (this app imports Ukrainian and German recipes — it arguably wants that field regardless) and gate the rules on it; or drop stemming for token/trigram Jaccard, which absorbs plural drift without per-language rules and is cheaper to reason about. Note that `nameKey` already has the right shape — it compares *both* the raw and singular forms and takes the max. I'd extend that to a **set** of keys, so adding a rule is additive rather than destructive.

### 7. I wouldn't have built the household dimension yet

`Scope`, `CursorKey(type, householdID)`, `Suggestion.HouseholdID`, and the `?scope=` param all exist. Zero `ScopeHousehold` detectors are registered — the plan's `food_price` was the one that would have used it, and it didn't ship. Meanwhile the single place that *would* break is the place that wasn't parameterized: `DedupKey` has a global `uniqueIndex` with no household component, so two households producing the same key would collide.

Generalizing against an absent second case is exactly how seams end up in the wrong place. I'd either build one household detector in the same commit to keep the abstraction honest, or defer `Scope` until it has a caller. If keeping it, `DedupKey` should be constructed by a helper that takes `householdID`, so no detector can forget it.

### 8. Confidence is one field carrying five different meanings

`food_default_unit` computes a real usage ratio; the merge detectors emit name similarity; `ingredient_food_link.go:87` hardcodes `0.85`; `food_conversion.go:63` hardcodes `0.7`. Per-type thresholds make this workable internally, but it's exposed as one API field that a UI will inevitably sort across types. I'd name it `score`, return the type's threshold alongside it, and treat the hardcoded constants for what they are — placeholders shaped like confidence.

### Smaller things I'd fix at t=0

- `merge_detect.go:98` — `p, _ := json.Marshal(payload)` swallows the error.
- `food_default_unit.go:133` compares a `time.Time` round-tripped through JSON against a DB value. Fine on SQLite; on MySQL `DATETIME` truncation it will never match, and this project supports MySQL/Postgres. A version counter, or just re-checking "still NULL", is portable.

`★ Insight ─────────────────────────────────────`
Items 1, 2 and 3 share a shape: they're all *signature* decisions (`Apply`'s return type, what detectors take in their constructor, what `Payload` is). Signatures are nearly free to get right before there are implementations and expensive after — six appliers and six detectors now have to change together. Items 5–8 are tuning; they can land incrementally at any time. That asymmetry is the real answer to "what would I do differently" — not "which code is wrong," but "which decisions had a short window."
`─────────────────────────────────────────────────`
