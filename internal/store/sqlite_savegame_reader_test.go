package store_test

import (
	"context"
	"testing"

	"smb-tools/internal/store"
	"smb-tools/internal/testutil"
)

func TestGetUserTeamGUID(t *testing.T) {
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")

	guid, err := reader.GetUserTeamGUID(context.Background(), "EE000000000000000000000000000000")
	if err != nil {
		t.Fatalf("GetUserTeamGUID: %v", err)
	}
	if guid != "01000000000000000000000000000000" {
		t.Errorf("GUID: got %q, want test franchise team GUID", guid)
	}
}

func TestGetSeasonProgress(t *testing.T) {
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")

	progress, err := reader.GetSeasonProgress(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetSeasonProgress: %v", err)
	}
	if progress.IsComplete {
		t.Error("IsComplete: got true, want false")
	}
	if progress.ScheduledGames != 2 || progress.CompletedGames != 2 {
		t.Errorf("game counts: got %d/%d, want 2/2", progress.CompletedGames, progress.ScheduledGames)
	}

	if _, err := db.ExecContext(context.Background(), `
		UPDATE t_seasons SET completionDate = 1783793548 WHERE ID = 100
	`); err != nil {
		t.Fatalf("setting completion date: %v", err)
	}
	progress, err = reader.GetSeasonProgress(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetSeasonProgress after completion: %v", err)
	}
	if !progress.IsComplete {
		t.Error("IsComplete after completion date: got false, want true")
	}
}

func TestGetPlayoffSeries_IncludesCreatedSeries(t *testing.T) {
	db := testutil.NewTestSaveGameDB(t)
	reader := store.NewSqliteSaveGameReader(db, "")

	series, err := reader.GetPlayoffSeries(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetPlayoffSeries: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("series count: got %d, want 1", len(series))
	}
	if series[0].SeriesNum != 1 || series[0].Team1Name != "Home Squad" || series[0].Team2Name != "Away Crew" {
		t.Errorf("series: got %+v, want series 1 Home Squad vs Away Crew", series[0])
	}
}

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
