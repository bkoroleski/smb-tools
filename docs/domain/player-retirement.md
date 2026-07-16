# Player Retirement in SMB4

This document describes retirement behavior confirmed from an SMB4 save's SQLite trigger, SQL extracted from the game executable, and before-and-after franchise save snapshots.

## Database Scope

The tables, view, and trigger described here are in the SQLite database embedded in the SMB4 per-league `league-{GUID}.sav` snapshots. They are not from `master.sav` or another game database. The cleanup SQL was extracted from the executable and connected to this database by the affected tables and the observed before-and-after snapshot changes.

## Retirement Lifecycle

Retirement is applied during the offseason rollover into the next season. The pre-season draft state is already part of that next season in the save data.

When the game removes a retiring player through `v_franchise_players_including_pending_players`, the view's `INSTEAD OF DELETE` trigger, `tr_record_deleted_franchise_player_information`:

1. Copies the player's name, positions, pitcher role, age, and salary to `t_stats_players`.
2. Sets `t_stats_players.retirementSeason`.
3. Deletes the live `t_baseball_players` row.

The resulting `t_stats_players` row has `baseballPlayerLocalID = NULL`, but retains its `statsPlayerID`, frozen player metadata, and retirement flag. The flag is therefore visible once the next season has begun and can be detected by a sync at any point in that season. It is not limited to the draft announcement screen.

## Meaning of `retirementSeason`

`t_seasons` is the generic season model used beneath the game's different modes. Franchise mode adds a persistent playthrough in `t_franchise`, distinct from the league definition in `t_leagues`. `t_franchise_seasons` links that playthrough's `franchiseGUID` to each season's `t_seasons.GUID` through `seasonGUID`.

The deletion trigger calculates `retirementSeason` with a `COUNT(*)` over the links for the retiring player's franchise. It is therefore the franchise-local number of the season that just ended.

For example, a player removed during the rollover from the first season into the second receives `retirementSeason = 1`.

The trigger does not read `t_seasons.ID`; `t_franchise_seasons` does not contain that column. In the snapshots inspected, the count and integer season ID are nevertheless numerically equal because the league database creates one append-only `t_seasons` row and one franchise link per season, starting at 1. The relationship is structurally through `t_seasons.GUID`, while equality with `t_seasons.ID` is an observed invariant of these saves.

smb-tools resolves the flag through the imported season's `save_game_season_id`, scoped to the same league source. A chained source's display offset is already represented by that imported companion season and does not change this lookup.

## Durable Player Identity

The live player GUID identifies a row in `t_baseball_players`. That row is deleted at retirement, so the GUID alone cannot connect a later save to the companion player.

`t_stats_players.statsPlayerID` is the durable link:

- smb-tools records it while the player is still active.
- The game retains it on the frozen `t_stats_players` row after deleting the live player.
- A later sync matches the frozen row to the companion player by `statsPlayerID` and records the season after which the player retired.

This explicit flag is safer than inferring retirement from a player disappearing between saves. The game also replaces live free agents that have no stats identity during player-pool regeneration. Those players disappear without producing a retirement record.

## Retention and Cleanup

A frozen `t_stats_players` row is not permanent merely because `retirementSeason` is set. Its lifetime is determined by references from `t_stats`.

During offseason cleanup, the game executes these operations in order:

1. Delete regular-season `t_stats` aggregators outside the franchise's newest 50 `t_seasons.ID` rows, except records protected by `allSeasonLeaders`.
2. Apply the equivalent cleanup to playoff aggregators.
3. Delete a retired player's regular-season career aggregator when that player has no remaining regular-season aggregates for the franchise, except records protected by `careerLeaders`.
4. Apply the equivalent cleanup to the playoff career aggregator.
5. Delete any `t_stats_players` row whose `statsPlayerID` is no longer referenced by `t_stats`.

These are executable query IDs `0xA4` through `0xA8`. Both 50-season cleanup queries bind the literal `0x32` (50); the boundary is not inferred from the UI.

This means a retirement record survives while at least one stats aggregator still references its `statsPlayerID`. An ordinary retired identity can be pruned on the first offseason cleanup after its remaining seasonal and career aggregates are removed. Leader records can keep the identity alive longer.

The cleanup batch removes old detail and aggregators, not season identity rows. We found no deletion of `t_seasons` or `t_franchise_seasons`; the 50-season boundary applies to referenced stats data rather than resetting season numbering or the season table's autoincrement value.

## Snapshot Validation

The final Season 1 snapshot contained 567 live players with all three of the following: a `t_stats_players` identity, a referencing `t_stats` row, and a Season 1 `t_season_stats` aggregate. In the first Season 2 snapshot:

- 550 remained live.
- 17 disappeared from the live-player table and all 17 had `retirementSeason = 1`.
- All 17 retained their stats identity and Season 1 aggregate.
- No player in this 567-player cohort disappeared without a retirement flag.
- No stats identity in the cohort was deleted.

The same rollover also removed 33 free agents that had no `t_stats_players` identity and added replacement players. This confirms that disappearance alone is not an authoritative retirement signal.

The snapshots establish that recorded retirements survive the rollover and remain importable in the next season. The executable's cleanup SQL establishes the later pruning rule; we have not yet observed a franchise crossing that boundary in snapshots.

## Implications for smb-tools

- Capture `statsPlayerID` while a player is live.
- Treat a non-null `retirementSeason` as the authoritative retirement flag.
- Resolve the flag through the imported save-game season for the same league source.
- Do not infer retirement from absence in the latest roster or latest season.
- Continue syncing before offseason rollover to preserve the season's per-game detail; detecting the retirement itself can happen afterward.
