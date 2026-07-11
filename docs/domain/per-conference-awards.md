# Per-Conference Awards — Design Plan

## Status

Not started. This document captures the design for future implementation.

## Background

smb-tools currently awards a single set of league-wide awards (MVP, Cy Young,
ROY, etc.) per season. In SMB4 franchises with multiple conferences, the playoff
bracket is structured so that each conference has its own side of the bracket
and the two conference winners meet in the championship series. This means
per-conference awards (e.g. "Soup Conference MVP") are meaningful — they
recognize the best player within a conference, analogous to MLB's AL/NL awards.

### Current state

- The `awards` table has a free-text `name` column with no conference/scope
  column. Award names like "MVP" and "Cy Young" are league-wide.
- The `player_season_awards` M2M junction has no scoping — an assignment is just
  (player_season_id, award_id).
- Conference data exists on `team_season_history.conference_name` but is not
  referenced by any award query or view.
- The auto-compute logic in `award.go` uses ~10 hardcoded award name strings as
  map keys. The auto-suggest logic in `award_candidates.go` uses ~19 name strings
  plus 3 package-level name slices. These are the primary coupling points.
- The frontend display layer is fully name-agnostic — it renders the raw `name`
  field verbatim. Custom award names would display correctly with zero changes.
- The `season_conference_champions` view identifies the championship series
  runner-up (which is a conference champion in SMB4's bracket structure), but
  the "Conference Champion" award name doesn't include the specific conference
  name (e.g. "Soup" vs "Sandwich").

### What already works

- Custom awards with arbitrary names (e.g. "Soup Conference MVP") can be created
  via `CreateCustomAward` and would display correctly in the UI.
- The summary grouping/sorting is dynamic via `parent_award_id` / `runner_up_rank`
  — new runner-up slots roll up under their parent automatically.
- The stat-leader qualification thresholds (`numGames * 3.1` PA, `numGames * 3`
  outs) are conference-agnostic and would work per-conference without change.

## Design

### Approach: name-encoding with conference-aware computation

Seed per-conference awards as distinct rows in the `awards` table with
conference-qualified names (e.g. "Soup Conference MVP", "Sandwich Conference
Cy Young"). This avoids a schema change to `player_season_awards` and leverages
the existing name-agnostic display layer.

A new migration seeds per-conference award rows for each existing user-assignable
award, parameterized by the franchise's conference names. Since migrations are
static SQL and conference names are franchise-specific, the seeding happens at
import time (when conference names are first known), not at migration time.

### Schema changes

No changes to `awards` or `player_season_awards` tables. Conference scoping is
encoded in the award `name` itself.

Two new columns on `awards` to support dynamic lookup:

```sql
ALTER TABLE awards ADD COLUMN is_conference_scoped INTEGER NOT NULL DEFAULT 0;
ALTER TABLE awards ADD COLUMN conference_name TEXT NOT NULL DEFAULT '';
```

- `is_conference_scoped = 1` marks awards that are per-conference (e.g.
  "Soup Conference MVP"). League-wide awards keep `is_conference_scoped = 0`.
- `conference_name` stores the specific conference (e.g. "Soup") for
  conference-scoped awards. Empty for league-wide awards.

These columns allow the auto-compute and auto-suggest logic to discover
conference-scoped awards dynamically rather than hardcoding name patterns.

### Award seeding

At import time, after conference names are known:

1. Query distinct `conference_name` values from `team_season_history` for the
   season.
2. For each conference, seed per-conference variants of the user-assignable
   awards: MVP, Cy Young, ROY, Silver Slugger, All-Star, Gold Glove, and their
   runner-up slots. Name them `"{Conference} {AwardName}"` (e.g. "Soup
   Conference MVP").
3. Set `is_conference_scoped = 1` and `conference_name = "{Conference}"` on
   these rows.
4. Wire up `parent_award_id` / `runner_up_rank` for runner-up variants, same
   as the league-wide versions.

League-wide awards (MVP, Cy Young, etc.) remain as-is. Both league-wide and
per-conference variants can coexist.

### Auto-compute refactoring (`award.go`)

The stat-leader queries (`queryQualifiedLeader`, `queryMaxLeaders`) currently
compute a single league-wide leader. They need to be partitioned by conference:

1. Add an optional `conferenceName` parameter to the leader queries.
2. When `conferenceName` is non-empty, filter to players whose
   `team_season_history.conference_name` matches.
3. For each conference-scoped stat title (e.g. "Soup Conference Batting Title"),
   compute the leader within that conference and assign the award.

The hardcoded name-string map keys (`awardIDs["Batting Title"]`, etc.) are
replaced with a lookup that finds conference-scoped award IDs by matching
`is_conference_scoped = 1 AND conference_name = ?` and deriving the base award
type from the name suffix or a new `base_award_id` column.

### Auto-suggest refactoring (`award_candidates.go`)

The hardcoded name slices (`mvpAwardNames`, `cyYoungAwardNames`, `royAwardNames`)
and the 19-name lookup list are replaced with dynamic discovery:

1. Query all user-assignable awards where `is_conference_scoped = 0` (league-wide)
   — these are the existing suggestions.
2. For each conference, query all user-assignable awards where
   `is_conference_scoped = 1 AND conference_name = ?` — these are the per-
   conference suggestions.
3. For each conference, run the candidate queries with a conference filter and
   assign awards using the conference-scoped award IDs.

The `applyAutoSuggest` function gains a loop over conferences, calling the
existing helper functions with conference-filtered candidate lists.

### Candidate queries

`queryBattingCandidates` and `queryPitchingCandidates` gain an optional
`conferenceName` parameter. When set, they filter on
`tsh.conference_name = ?` via the existing join to `team_season_history`.

The playoff candidate queries do not need conference filtering — playoff MVP
and Championship MVP remain league-wide awards.

### "Conference Champion" award name

The existing "Conference Champion" award should include the conference name in
its display. Two options:

1. **Rename at seed time**: Change the award name from "Conference Champion" to
   `"{Conference} Conference Champion"` (e.g. "Soup Conference Champion"). This
   requires updating the `assignChampionshipAwards` logic to look up the
   conference-scoped award by name.

2. **Compose at display time**: Keep the award name as "Conference Champion" but
   add the conference name in the frontend display. This requires passing
   conference context to the display layer, which is more invasive.

Option 1 is simpler and consistent with the name-encoding approach.

### Frontend changes

Minimal. The display layer already renders award names verbatim. The only
changes needed:

1. The "Suggest Awards" banner text may need updating to mention per-conference
   suggestions.
2. The awards MultiSelect in the delegation page will show more award options
   (one set per conference). No structural change needed — the dropdown already
   lists all user-assignable awards.
3. The summary view will show per-conference award groups alongside league-wide
   groups, ordered by importance. No structural change needed.

### Migration strategy

- New migration adds `is_conference_scoped` and `conference_name` columns to
  `awards` (additive, default 0 / ''). Existing awards are unaffected.
- Per-conference award rows are seeded at import time, not at migration time,
  because conference names are franchise-specific.
- A re-import or explicit "seed conference awards" action is needed for existing
  franchises. This could be triggered automatically the next time a season is
  imported, or via a button in the UI.

## Open questions

1. **Should league-wide awards coexist with per-conference awards?** E.g.,
   should there be both "MVP" (league-wide) and "Soup Conference MVP"
   (per-conference)? Or should the league-wide MVP be replaced by per-conference
   MVPs? MLB has both — AL MVP, NL MVP, and no "MLB MVP". The recommendation is
   to keep league-wide awards for things like Playoff MVP and Championship MVP
   (which are inherently league-wide), and add per-conference variants for
   MVP, Cy Young, ROY, Silver Slugger, and All-Star.

2. **How many conferences?** SMB4 typically has 2 conferences, but the design
   should handle N conferences. The seeding and computation loops should be
   conference-count-agnostic.

3. **Gold Glove per conference?** Gold Glove is currently not auto-suggested
   (insufficient fielding data). Per-conference Gold Gloves would have the same
   limitation. They can be seeded as user-assignable awards even if not
   auto-suggested.

4. **Runner-up depth per conference?** Should per-conference awards have the
   same 5-deep runner-up chain (MVP-2 through MVP-5), or a shorter chain (e.g.
   top 3)? With 2 conferences, each conference has fewer teams, so a 5-deep
   chain may be excessive. This is a UX decision.

5. **Stat titles per conference?** Should "Batting Title" etc. also be
   per-conference? MLB has separate AL/NL batting titles. The recommendation
   is yes — seed per-conference stat titles and compute them per-conference.

## Implementation order

1. Migration: add `is_conference_scoped` and `conference_name` columns.
2. Import-time seeding: seed per-conference award rows when conference names
   are first encountered.
3. Refactor auto-compute: conference-partitioned stat-leader queries.
4. Refactor auto-suggest: dynamic award discovery by conference scope.
5. Rename "Conference Champion" to include conference name.
6. Frontend: update banner text, verify MultiSelect and summary display.
7. Tests: per-conference auto-compute, auto-suggest, and display.
