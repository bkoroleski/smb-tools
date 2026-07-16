-- Track player retirements authoritatively from the save game.
--
-- stats_player_id captures t_stats_players.statsPlayerID at import time. This ID
-- survives the offseason rollover (the player's GUID does not — the live row is
-- deleted), so it is the durable link used to match a save-side retiree back to a
-- companion players row on a later sync.
--
-- retired_after_season_id is written when the save's t_stats_players.retirementSeason
-- is set for a player. retirementSeason holds the save's t_seasons.ID, which the
-- companion stores as seasons.save_game_season_id (scoped by league_guid); the
-- import resolves it to a companion seasons.id. This is the authoritative
-- retirement signal, replacing the Hall of Fame query's former "absent from
-- latest season" inference. NULL means active.

ALTER TABLE players ADD COLUMN stats_player_id INTEGER;
ALTER TABLE players ADD COLUMN retired_after_season_id INTEGER REFERENCES seasons(id);

CREATE INDEX idx_players_stats_player_id ON players(stats_player_id);
