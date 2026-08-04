ALTER TABLE save_game_snapshots ADD COLUMN phase TEXT
    CHECK (phase IN (
        'preseason',
        'regular_season',
        'end_regular_season',
        'playoffs',
        'playoffs_eliminated',
        'end_season'
    ));

ALTER TABLE save_game_snapshots ADD COLUMN game_number INTEGER
    CHECK (game_number IS NULL OR game_number > 0);

ALTER TABLE save_game_snapshots ADD COLUMN opponent_team_name TEXT;

ALTER TABLE save_game_snapshots ADD COLUMN is_home INTEGER
    CHECK (is_home IS NULL OR is_home IN (0, 1));
