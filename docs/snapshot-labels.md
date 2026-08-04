# Snapshot Labels

## Status

Research complete enough for implementation planning. This document records the confirmed save-game
signals, the intended Franchise-versus-Season behavior, and the decisions that remain before code
changes begin.

## Goal

Snapshot labels should identify the state represented by a save, not merely its season number. A useful
label should answer as much of the following as the game mode allows:

- Which season or season transition does the snapshot belong to?
- Is the league in the regular season, playoffs, offseason draft, or pre-season?
- For Franchise mode, what was the player team's latest game or current playoff matchup?
- For Season mode, what is the league-wide progress or playoff round?

Franchise mode has a fixed player team. Season mode does not, so Season labels must remain league-relative
or bracket-relative rather than inventing a player-team perspective.

## Current Implementation

Detailed snapshot metadata was added after Season-mode support. Commit `cd15108` added Season-mode
tracking, while the later commit `a1ccd20` added the phase classifier and detailed labels. The current
classifier therefore inherited Franchise-only assumptions even though the application already supported
both modes.

The relevant implementation is split across:

- [`internal/service/snapshot_phase.go`](../internal/service/snapshot_phase.go) — save-state
  classification.
- [`internal/store/sqlite_savegame_reader.go`](../internal/store/sqlite_savegame_reader.go) — raw save
  queries.
- [`internal/store/snapshot_store.go`](../internal/store/snapshot_store.go) — persisted snapshot metadata.
- [`frontend/src/components/SnapshotPicker.vue`](../frontend/src/components/SnapshotPicker.vue) — label
  rendering.

Persisted metadata is currently team-relative:

- `phase`
- `game_number`
- `opponent_team_name`
- `is_home`

This is sufficient for Franchise regular-season labels but not for league-relative Season labels,
playoff-round names, or the offseason draft.

## Confirmed Save-Game Evidence

### Franchise playoff structure

The Hambone League Season 2 save has a three-round, best-of-five playoff with two conferences. Its
created series use this numbering:

| Series numbers | Game round value | Display label |
|---|---:|---|
| 0–3 | 3 | Conference Semifinals |
| 4–5 | 2 | Conference Finals |
| 6 | 1 | League Championship |

The Bards advanced through these series:

- Series 1: Metalskins vs Bards — Conference Semifinals.
- Series 4: Bottlenoses vs Bards — Conference Finals.
- Series 6: Bards vs Tramplers — League Championship.

Series 6 existed before its first completed game. At that point the current classifier selected the
newest Bards series, found no completed game in it, and fell back to the generic `Playoffs` label. This
proves that playoff phase and current matchup must be derived from `t_playoff_series`, not only from
completed `t_playoff_games` rows.

Franchise news independently recorded the completed games with `round = 3` for Conference Semifinals
and `round = 2` for Conference Finals. The news data validates the mapping but must not be the source of
snapshot labels because non-Franchise modes do not have a Franchise news feed.

### Game playoff vocabulary

Targeted executable inspection found a shared Persistent Seasons label path for ordinary Season and
Elimination modes. The Season vocabulary is:

| Conferences | Round value | Label |
|---:|---:|---|
| Any | 1 | League Championship |
| 2 | 2 | Conference Finals |
| 2 | 3 | Conference Semifinals |
| 2 | 4 | Conference Quarterfinals |
| 1 | 2 | League Semifinals |
| 1 | 3 | League Quarterfinals |

The executable also contains Elimination labels (`Finals`, `SemiFinals`, and `QuarterFinals`), but
Elimination tracking is outside the current application scope.

The generic Season label selector is separate from the Franchise Hub news renderer. Franchise news uses
`t_franchise_news_game_result` and `FranchiseNews/GameResult/*` localization objects; it should not be
treated as the renderer for Season-mode labels.

### Season mode

A live Season-mode save for the Massive Bullpen League confirmed:

- No `t_franchise` row and therefore no `playerTeamGUID`.
- 7 of 480 league games completed.
- The latest completed result was Cobras 2–1 Runners.
- Two conferences.
- A three-round, best-of-five `t_playoffs` configuration already existed.
- No `t_playoff_series` rows existed because the league was still in the regular season.

This establishes two important rules:

1. Season mode can support detailed labels without a player team by using league-wide progress and
   neutral matchups.
2. The existence of `t_playoffs` does not mean the playoffs have started. Created
   `t_playoff_series` rows are the reliable transition signal.

### End of season, offseason draft, and pre-season

Consecutive Hambone League snapshots captured all three states around the Season 2 to Season 3
transition:

| State | Latest season | `completionDate` | `inOffSeason` | `offSeasonTicksCompleted` | Next-season schedule |
|---|---:|---|---:|---:|---|
| End of Season 2 | 2 | Set | 0 | 0 | Not created |
| Preseason Draft, Round 1 | 2 | Set | 1 | 0 | Not created |
| Preseason Draft, Round 3 | 2 | Set | 1 | 12 | Not created |
| Preseason Draft, Round 4 | 2 | Set | 1 | 18 | Not created |
| Season 3 pre-season | 3 | Not set | 0 | 0 | 384 scheduled, 0 completed |

The game UI has 32 draft rounds. Two independently observed rounds establish six internal offseason
ticks per completed displayed round:

```text
completed draft rounds = offSeasonTicksCompleted / 6
current draft round     = completed draft rounds + 1
```

Round 32 should therefore begin at tick 186. The exact stored value immediately after completing Round
32 was not observed because the game had already transitioned to Season 3. The post-draft reset itself
was observed: `inOffSeason` and `offSeasonTicksCompleted` returned to zero and the new season row and
schedule appeared.

The current classifier checks `t_seasons.completionDate` first, so it labeled both genuine end-of-season
snapshots and active draft snapshots as `End of season`. For Franchise mode, `t_franchise.inOffSeason`
must take precedence over the completed-season test.

## Intended Classification

### Shared states

| State | Reliable signal |
|---|---|
| End of season | Latest season is complete and Franchise is not in offseason, or completed Season-mode season has no successor yet |
| Pre-season | Latest season exists, is incomplete, has a schedule, and has zero completed games |
| Regular season | Latest season has completed regular-season games and the playoffs have no created series |
| End of regular season | Regular schedule is complete and the playoffs have no created series |
| Playoffs | One or more `t_playoff_series` rows have been created |

### Franchise-only states

| State | Reliable signal |
|---|---|
| Preseason Draft | `t_franchise.inOffSeason = 1` |
| Player team eliminated | Player team has no active later series, or has lost the deciding game in its latest series |

The Franchise lifecycle precedence should be:

1. Preseason Draft
2. End of season
3. Playoffs
4. End of regular season
5. Regular season
6. Pre-season

The draft check must occur before the completed-season check because the just-finished season remains
the latest `t_seasons` row throughout the draft.

### Season-mode differences

Season mode must not use:

- `GetUserTeamGUID`
- opponent-only matchup fields
- `vs`/`@` relative to a player team
- `Team eliminated`

Instead, it should use:

- League-wide completed and scheduled game counts during the regular season.
- Neutral home/away team names when a latest-result matchup is useful.
- The furthest created playoff round represented in `t_playoff_series`.
- A neutral playoff-round label when multiple series are active.

## Proposed Label Shapes

These examples establish the information hierarchy. Final punctuation and capitalization can be settled
during implementation.

### Franchise mode

```text
Season 2 - After Game 12 (@ Crocodons)
Season 2 - After Conference Semifinals Game 3 (vs Metalskins)
Season 2 - After Conference Finals Game 3 (vs Bottlenoses)
Season 2 - League Championship (vs Tramplers)
Season 2 - Playoffs (Team eliminated)
Season 2 → 3 - Preseason Draft, Round 4 of 32
Season 3 - Pre-season
```

The transition form keeps the snapshot associated with Season 2 for reimport purposes while making it
clear that the draft is building the Season 3 roster.

### Season mode

```text
Season 1 - After league game 7 of 480
Season 1 - After league game 7 (Runners @ Cobras)
Season 1 - Conference Semifinals
Season 1 - After Conference Semifinals Game 2 (Runners @ Cobras)
Season 1 - League Championship
Season 2 - Pre-season
```

The first Season regular-season form is sufficient for an initial implementation. The neutral matchup
form requires persisting both teams and identifying the latest result globally.

## Structural Metadata Needed

Snapshot metadata should persist facts rather than rendered English labels. Candidate additions are:

- League mode or label perspective (`franchise` versus `season`), unless supplied by the immutable
  tracker record at render time.
- Completed and scheduled regular-season game counts.
- Neutral home and away team names for Season-mode result labels.
- Playoff series number.
- Playoff round value.
- Total playoff rounds.
- Conference count.
- Series length.
- Draft/offseason state and completed offseason ticks, or a derived draft round.

The exact minimal set should be chosen in the implementation plan. Existing `game_number`,
`opponent_team_name`, and `is_home` fields should remain readable for older snapshots.

## Implementation Boundaries

The feature crosses these layers:

1. Save-game reader
   - Read Franchise lifecycle state from `t_franchise`.
   - Include conference count in playoff configuration.
   - Preserve enough result ordering to identify a latest league-wide result when needed.
2. Classifier
   - Accept or resolve the league mode.
   - Branch between team-relative Franchise logic and league-relative Season logic.
   - Detect playoffs from created series.
   - Derive round values from bracket configuration and series numbering.
3. Companion persistence
   - Add structural metadata with a new, unused migration version.
   - If a distinct draft phase is added, account for the current `phase` column check constraint.
4. DTO and UI
   - Carry the new structural fields to `SnapshotPicker`.
   - Render mode-appropriate labels and safe generic fallbacks.
5. Tests
   - Extend the synthetic Season-mode save with conference and playoff scenarios.
   - Correct synthetic series numbering to match the observed zero-based game data.

## Existing Snapshot Metadata

Labels are persisted at capture time. Improving the classifier does not automatically repair old rows.

There are two related compatibility cases:

1. **Identical live save:** `SnapshotService` currently skips an identical hash without updating the
   latest snapshot's derived metadata. It should refresh metadata on a hash match so resyncing after a
   classifier improvement corrects the latest label without creating a duplicate file.
2. **Older snapshot files:** correcting all historical labels requires reopening each snapshot and
   reclassifying it. This should be an explicit backfill decision rather than an incidental UI behavior,
   especially because snapshots may be compressed or missing.

Any companion migration must use a genuinely unused numeric version. The migration runner compares the
integer prefix only; reusing a version under a different filename silently skips the later file on an
existing database.

## Test Cases Required Before Completion

- Franchise regular-season home and away labels remain unchanged.
- Franchise playoff round names for one- and two-conference leagues.
- Franchise series created before Game 1.
- Franchise player team eliminated.
- Genuine end-of-season state before entering the draft.
- Draft Rounds 1, 3, 4, and 32 with safe handling for unexpected tick values.
- Transition from draft to a newly created pre-season.
- Season-mode regular-season progress without calling `GetUserTeamGUID`.
- Season-mode playoff round with multiple active series.
- Season-mode championship with one remaining series.
- Identical-hash sync refreshes metadata without creating a second snapshot.
- Older metadata rows still render with their existing fields.
- Unexpected conference counts or series numbering fall back to a generic `Playoffs` or
  `Playoff Round N` label rather than failing the sync.

## Open Decisions

- Persist a distinct `offseason_draft` phase or persist draft context alongside an existing phase.
- Final draft wording: `Preseason Draft`, `Offseason Draft`, or another phrase matching the game UI.
- Whether the first release of Season-mode labels includes neutral matchup details or only league-wide
  progress and playoff round.
- Whether to display series length as `Game 3 / 5`.
- Whether to backfill every existing snapshot or only refresh the latest snapshot on the next sync.
- Whether an unexpected non-multiple-of-six offseason tick value should display a derived round or fall
  back to a generic draft label.

