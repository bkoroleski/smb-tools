package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"smb-tools/internal/store"
	"smb-tools/internal/testutil"
)

func seedSnapshot(t *testing.T, s *store.SnapshotStore, seasonNum int) int64 {
	t.Helper()
	snap := store.Snapshot{
		SeasonNum:     seasonNum,
		CapturedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		FileName:      store.SnapshotFileName("0001_abc123.sqlite"),
		SHA256Hash:    store.SHA256Hex("aabbccdd"),
		FileSizeBytes: 1024,
	}
	id, err := s.Record(context.Background(), snap)
	if err != nil {
		t.Fatalf("seedSnapshot: %v", err)
	}
	return id
}

func TestSnapshotStore_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	s := store.NewSnapshotStore(db)
	ctx := context.Background()

	id := seedSnapshot(t, s, 3)

	got, err := s.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID: got %d, want %d", got.ID, id)
	}
	if got.SeasonNum != 3 {
		t.Errorf("SeasonNum: got %d, want 3", got.SeasonNum)
	}
	if got.FileSizeBytes != 1024 {
		t.Errorf("FileSizeBytes: got %d, want 1024", got.FileSizeBytes)
	}
	if got.Metadata != nil {
		t.Errorf("Metadata: got %+v, want nil for a legacy snapshot", got.Metadata)
	}
}

func TestSnapshotStore_MetadataRoundTrip(t *testing.T) {
	db := testutil.NewTestDB(t)
	s := store.NewSnapshotStore(db)
	ctx := context.Background()
	gameNumber := 4
	opponent := "Away Crew"
	isHome := true

	id, err := s.Record(ctx, store.Snapshot{
		SeasonNum:     2,
		CapturedAt:    time.Date(2026, 7, 19, 12, 30, 0, 0, time.UTC),
		FileName:      store.SnapshotFileName("0002_phase.sqlite"),
		SHA256Hash:    store.SHA256Hex("phasehash"),
		FileSizeBytes: 2048,
		Metadata: &store.SnapshotMetadata{
			Phase:            store.SnapshotPhaseRegularSeason,
			GameNumber:       &gameNumber,
			OpponentTeamName: &opponent,
			IsHome:           &isHome,
		},
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := s.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Metadata == nil {
		t.Fatal("Metadata: got nil, want populated metadata")
	}
	if got.Metadata.Phase != store.SnapshotPhaseRegularSeason {
		t.Errorf("Phase: got %q, want %q", got.Metadata.Phase, store.SnapshotPhaseRegularSeason)
	}
	if got.Metadata.GameNumber == nil || *got.Metadata.GameNumber != 4 {
		t.Errorf("GameNumber: got %v, want 4", got.Metadata.GameNumber)
	}
	if got.Metadata.OpponentTeamName == nil || *got.Metadata.OpponentTeamName != "Away Crew" {
		t.Errorf("OpponentTeamName: got %v, want Away Crew", got.Metadata.OpponentTeamName)
	}
	if got.Metadata.IsHome == nil || !*got.Metadata.IsHome {
		t.Errorf("IsHome: got %v, want true", got.Metadata.IsHome)
	}

	listed, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 || listed[0].Metadata == nil {
		t.Fatalf("List metadata: got %+v, want one populated snapshot", listed)
	}
}

func TestSnapshotStore_GetByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	s := store.NewSnapshotStore(db)

	_, err := s.GetByID(context.Background(), 9999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}
