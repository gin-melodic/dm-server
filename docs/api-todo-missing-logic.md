# API TODO Missing Logic

This document tracks the missing or underspecified business logic called out by TODO comments in `api/`.

## History API

### `GET /v1/dream/detail`

Source: `api/history/v1/history.go`, `GetDreamReq`

Current logic:
- Loads the current user from auth context.
- Returns one non-deleted dream owned by the user.
- Joins the latest non-deleted `analysis_sessions` row only for `result_summary`.

Missing logic to supply:
- Decide whether only `completed` dreams can be returned, or whether `pending`, `processing`, and `error` dreams should also be visible.
- Define the canonical source for `interpretation`, `keywords`, and `confidenceScore`; current code maps `interpretation` from `analysis_sessions.result_summary`, `keywords` from `dreams.tags`, and uses fallback confidence when the stored value is zero.
- Add consistent not-found and unauthorized response semantics for frontend handling.
- Add coverage for deleted dreams, dreams owned by another user, and dreams without an analysis session.

### `PUT /v1/dream/update`

Source: `api/history/v1/history.go`, `UpdateDreamReq`

Current logic:
- Partially updates `title`, `content`, `emotion`, and `is_favorite` for the authenticated owner.
- Returns the updated dream record.

Missing logic to supply:
- Define validation rules for user-editable fields: max title length, content length, allowed emotion values, and whether empty strings are valid updates or should mean "leave unchanged".
- Decide whether changing `content` should invalidate or regenerate the existing dream analysis.
- Decide whether changes should update `dream_date`, `tags`, `status`, or analysis session data.
- Detect update attempts for missing or non-owned dreams before returning the fetched record.
- Add audit/versioning behavior if edited dream content must be preserved.

### `POST /v1/dream/analyze`

Source: `api/history/v1/history.go`, `CreateDreamAnalysisReq`

Current logic:
- Calls `service.Dream().StreamDream(ctx, req.Content)`.
- Reads the streamed analysis text.
- Searches for the latest dream row matching the same user and content.
- Updates fixed confidence metadata and returns hard-coded completed steps.

Missing logic to supply:
- Define the actual creation flow for a dream and analysis session instead of relying on `StreamDream` side effects and a follow-up content lookup.
- Persist analysis session lifecycle: `pending`, `processing`, `completed`, `error`, progress, result, error message, and timestamps.
- Generate structured analysis fields from model output: summary, interpretation, keywords, confidence, locale, title, tags, and emotion.
- Decide duplicate-content behavior: create a new dream every time, reuse existing dream, or deduplicate within a time window.
- Wrap dream creation, model result persistence, and metadata updates in a transaction where possible.
- Define failure behavior when model streaming fails after a dream has been inserted.
- Validate `locale` and define fallback language behavior.
- Add tests for success, model failure, duplicate content, and persistence of both `dreams` and `analysis_sessions`.

### `PATCH /v1/dream/favorite`

Source: `api/history/v1/history.go`, `SetDreamFavoriteReq`

Current logic:
- Confirms the dream belongs to the current user.
- Updates `dreams.is_favorite`.
- Returns the updated dream record.

Missing logic to supply:
- Normalize the JSON field name. The request uses `json:"is_favorite"` while the response uses `isFavorite`; confirm the frontend contract.
- Decide whether favorite can be set on non-completed or soft-deleted dreams.
- Add tests for idempotent favorite updates, unauthorized access, and missing dreams.

### `GET /v1/dream/home`

Source: `api/history/v1/history.go`, `GetDreamHomeReq`

Current logic:
- Counts all non-deleted dreams for the current user.
- Returns the five most recent dreams.
- Builds emotion waves from those recent dreams only.
- Uses the most recent dream as the recommendation.
- Calculates streak as the count of distinct dates in the recent list.

Missing logic to supply:
- Define real home dashboard requirements: date range, completed-only filtering, total count semantics, and sort order.
- Implement true current streak logic based on consecutive dream dates, not distinct dates in the latest five records.
- Aggregate emotion waves by date and emotion over the required period instead of echoing recent records with count `1`.
- Define recommendation selection logic and scoring tiers.
- Decide empty-state response shape when the user has no dreams.
- Add indexes or query tuning if the dashboard will scan larger dream histories.

### `GET /v1/dream/recommendation/today`

Source: `api/history/v1/history.go`, `GetTodayDreamRecommendationReq`

Current logic:
- Returns the most recent dream as a `standard` recommendation.

Missing logic to supply:
- Define what "today" means: user's timezone, `dream_date` vs `created_at`, and fallback behavior when there is no dream today.
- Implement recommendation ranking, score calculation, and tier rules.
- Decide whether the endpoint should return `null`, a fallback recommendation, or a not-found error when no candidate exists.
- Add tests for same-day, previous-day, empty-state, and multi-timezone behavior.

## User API

### `GET /v1/user/settings`

Source: `api/user/v1/user.go`, `GetUserSettingsReq`

Current logic:
- Loads `user_settings` by `user_id`.
- Returns defaults when no settings row exists.

Missing logic to supply:
- Confirm default values with product requirements: language, privacy mode, reminder behavior, reminder time, and storage mode.
- Validate whether returning defaults without inserting a row is acceptable.
- Define allowed enum values for `privacyMode` and `storageMode`.
- Add tests for first-time users, existing settings, and unauthorized access.

### `PUT /v1/user/settings`

Source: `api/user/v1/user.go`, `UpdateUserSettingsReq`

Current logic:
- Loads existing settings or defaults.
- Applies non-empty values and saves with insert-or-update logic.

Missing logic to supply:
- Add validation for language tags, privacy mode, storage mode, reminder time format, and reminder combinations.
- Decide whether empty strings can clear fields, especially `dreamReminderTime`.
- Replace count-then-insert/update with an upsert if concurrent updates are expected.
- Ensure `created_at` behavior is correct for inserted rows.
- Add tests for partial updates, clearing reminder time, invalid enum values, invalid times, and concurrent update behavior.

### `GET /v1/user/psyche-profile`

Source: `api/user/v1/user.go`, `GetPsycheProfileReq`

Current logic:
- Counts the user's dreams.
- Returns placeholder integration score bands and static archetype values.

Missing logic to supply:
- Define the real psyche profile model: inputs, scoring algorithm, archetype taxonomy, dominant archetype rules, and update cadence.
- Persist computed profile data if the score should be stable between requests.
- Decide whether the profile uses all dreams, completed dreams only, recent dreams, favorite dreams, or analysis sessions.
- Replace static archetype scores and descriptions with computed values.
- Define localization behavior for descriptions and archetype names.
- Add tests for no dreams, low/moderate/high data volume, and deterministic scoring.

## Cross-Cutting Gaps

- Regenerate model/entity files after schema changes. `internal/model/entity/dreams.go` does not currently include `emotion`, `is_favorite`, or `confidence_score`, even though the new API logic uses those fields through custom rows.
- Confirm that `resource/database/2-dreamnest-api-schema-changes.sql` is applied in all environments before enabling these endpoints.
- Standardize request and response casing between snake_case and camelCase.
- Add integration tests for the new TODO endpoints beyond the existing list/detail/delete/result coverage.
- Define a single error contract for validation failures, unauthorized access, missing records, and model-analysis failures.
