package store_test

import (
	"context"
	"testing"

	"smb-tools/internal/store"
	"smb-tools/internal/testutil"
)

func TestGetSeasonPlayoffConfig_Found(t *testing.T) {
	// Season 100 has a t_playoffs row with rounds=1, seriesLength=5.
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")
	ctx := context.Background()

	cfg, err := reader.GetSeasonPlayoffConfig(ctx, 100)
	if err != nil {
		t.Fatalf("GetSeasonPlayoffConfig(100): %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for season 100, got nil")
		return
	}
	if cfg.Rounds != 1 {
		t.Errorf("Rounds: want 1, got %d", cfg.Rounds)
	}
	if cfg.SeriesLength != 5 {
		t.Errorf("SeriesLength: want 5, got %d", cfg.SeriesLength)
	}
}

func TestGetSeasonPlayoffConfig_NotFound(t *testing.T) {
	// Season 101 has no t_playoffs row — reader should return nil without error.
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")
	ctx := context.Background()

	cfg, err := reader.GetSeasonPlayoffConfig(ctx, 101)
	if err != nil {
		t.Fatalf("GetSeasonPlayoffConfig(101): %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config for season 101 (no t_playoffs row), got %+v", cfg)
	}
}

func TestGetSeasonInningsPerGame_ReadsNonDefaultValue(t *testing.T) {
	// Season 100's t_seasons.innings is seeded to 7 (non-default) so this test
	// can't pass by coincidentally reading the column default instead of the
	// real column.
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")
	ctx := context.Background()

	got, err := reader.GetSeasonInningsPerGame(ctx, 100)
	if err != nil {
		t.Fatalf("GetSeasonInningsPerGame(100): %v", err)
	}
	if got != 7 {
		t.Errorf("GetSeasonInningsPerGame(100) = %d, want 7", got)
	}
}

func TestGetSeasonInningsPerGame_DefaultValue(t *testing.T) {
	// Season 101 doesn't override innings, so it should read the column
	// default (9, the standard SMB4 game length).
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")
	ctx := context.Background()

	got, err := reader.GetSeasonInningsPerGame(ctx, 101)
	if err != nil {
		t.Fatalf("GetSeasonInningsPerGame(101): %v", err)
	}
	if got != 9 {
		t.Errorf("GetSeasonInningsPerGame(101) = %d, want 9", got)
	}
}

func TestGetRetiredPlayers_ReturnsSeededRetiree(t *testing.T) {
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")
	ctx := context.Background()

	retirees, err := reader.GetRetiredPlayers(ctx)
	if err != nil {
		t.Fatalf("GetRetiredPlayers: %v", err)
	}
	if len(retirees) != 1 {
		t.Fatalf("expected 1 retiree, got %d: %+v", len(retirees), retirees)
	}
	r := retirees[0]
	if r.StatsPlayerID != 7 {
		t.Errorf("StatsPlayerID: want 7, got %d", r.StatsPlayerID)
	}
	if r.RetirementSeason != 100 {
		t.Errorf("RetirementSeason: want 100, got %d", r.RetirementSeason)
	}
	if r.FirstName != "Retired" || r.LastName != "Guy" {
		t.Errorf("name: want Retired Guy, got %q %q", r.FirstName, r.LastName)
	}
}

func TestGetCurrentSeasonPlayers_CapturesStatsPlayerID(t *testing.T) {
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")
	ctx := context.Background()

	players, err := reader.GetCurrentSeasonPlayers(ctx, 100)
	if err != nil {
		t.Fatalf("GetCurrentSeasonPlayers: %v", err)
	}
	var foundAA bool
	for _, p := range players {
		if p.FirstName == "Test" && p.LastName == "Batter" {
			foundAA = true
			if p.StatsPlayerID != 1 {
				t.Errorf("AA StatsPlayerID: want 1, got %d", p.StatsPlayerID)
			}
		}
	}
	if !foundAA {
		t.Fatal("did not find Test Batter in season 100 players")
	}
}
