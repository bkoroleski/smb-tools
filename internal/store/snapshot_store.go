package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SHA256Hex is the hex-encoded string representation of a SHA-256 digest.
// Using a named type makes the intent explicit at call sites and prevents
// accidentally passing an arbitrary string where a hash is expected.
type SHA256Hex string

// SnapshotFileName is the relative path of a snapshot file within the
// franchise's snapshots/ directory (e.g. "0007_4a8b3c912f1d.sqlite").
type SnapshotFileName string

// SnapshotPhase identifies the point in a season when a snapshot was captured.
type SnapshotPhase string

const (
	SnapshotPhasePreseason          SnapshotPhase = "preseason"
	SnapshotPhaseRegularSeason      SnapshotPhase = "regular_season"
	SnapshotPhaseEndRegularSeason   SnapshotPhase = "end_regular_season"
	SnapshotPhasePlayoffs           SnapshotPhase = "playoffs"
	SnapshotPhasePlayoffsEliminated SnapshotPhase = "playoffs_eliminated"
	SnapshotPhaseEndSeason          SnapshotPhase = "end_season"
)

// SnapshotMetadata describes the season phase and, when applicable, the
// user-controlled team's latest completed game.
type SnapshotMetadata struct {
	Phase            SnapshotPhase
	GameNumber       *int
	OpponentTeamName *string
	IsHome           *bool
}

// Snapshot represents one save game snapshot record from save_game_snapshots.
type Snapshot struct {
	ID                  int64
	SeasonNum           int
	CapturedAt          time.Time
	FileName            SnapshotFileName
	SHA256Hash          SHA256Hex
	FileSizeBytes       int64
	Compressed          bool
	CompressedSizeBytes *int64
	Metadata            *SnapshotMetadata
}

// SnapshotStore handles reads and writes for the save_game_snapshots table
// in a per-franchise companion database.
type SnapshotStore struct {
	db DBTX
}

func NewSnapshotStore(db DBTX) *SnapshotStore {
	return &SnapshotStore{db: db}
}

// LatestHash returns the SHA-256 hash of the most recently captured snapshot,
// or "" if no snapshots exist yet.
func (s *SnapshotStore) LatestHash(ctx context.Context) (SHA256Hex, error) {
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT sha256_hash FROM save_game_snapshots ORDER BY id DESC LIMIT 1`,
	).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("querying latest snapshot hash: %w", err)
	}
	return SHA256Hex(hash.String), nil
}

// Record inserts a new snapshot record. fileName is the relative path within
// the franchise's snapshots/ directory.
func (s *SnapshotStore) Record(ctx context.Context, snap Snapshot) (int64, error) {
	phase, gameNumber, opponentTeamName, isHome := snapshotMetadataValues(snap.Metadata)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO save_game_snapshots
			(season_num, captured_at, file_name, sha256_hash, file_size_bytes, compressed,
			 phase, game_number, opponent_team_name, is_home)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		snap.SeasonNum,
		snap.CapturedAt.UTC().Format("2006-01-02T15:04:05Z"),
		snap.FileName,
		snap.SHA256Hash,
		snap.FileSizeBytes,
		boolToInt(snap.Compressed),
		phase,
		gameNumber,
		opponentTeamName,
		isHome,
	)
	if err != nil {
		return 0, fmt.Errorf("recording snapshot: %w", err)
	}
	return res.LastInsertId()
}

// MarkCompressed updates the snapshot record after it has been compressed.
func (s *SnapshotStore) MarkCompressed(ctx context.Context, id int64, compressedFileName SnapshotFileName, compressedSize int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE save_game_snapshots
		SET compressed = 1, file_name = ?, compressed_size_bytes = ?
		WHERE id = ?
	`, compressedFileName, compressedSize, id)
	if err != nil {
		return fmt.Errorf("marking snapshot %d as compressed: %w", id, err)
	}
	return nil
}

// GetByID returns the snapshot with the given id.
func (s *SnapshotStore) GetByID(ctx context.Context, id int64) (Snapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, season_num, captured_at, file_name, sha256_hash,
		       file_size_bytes, compressed, compressed_size_bytes,
		       phase, game_number, opponent_team_name, is_home
		FROM save_game_snapshots
		WHERE id = ?
	`, id)
	sn, err := scanSnapshot(row)
	if err != nil {
		return Snapshot{}, fmt.Errorf("getting snapshot %d: %w", id, err)
	}
	return sn, nil
}

// List returns all snapshots for the franchise, ordered by capture time ascending.
func (s *SnapshotStore) List(ctx context.Context) ([]Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, season_num, captured_at, file_name, sha256_hash,
		       file_size_bytes, compressed, compressed_size_bytes,
		       phase, game_number, opponent_team_name, is_home
		FROM save_game_snapshots
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var snaps []Snapshot
	for rows.Next() {
		sn, err := scanSnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning snapshot: %w", err)
		}
		snaps = append(snaps, sn)
	}
	return snaps, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(scanner rowScanner) (Snapshot, error) {
	var sn Snapshot
	var capturedAt, fileName, sha256Hash string
	var compressedSize, gameNumber, isHome sql.NullInt64
	var phase, opponentTeamName sql.NullString
	var compressed int
	if err := scanner.Scan(
		&sn.ID, &sn.SeasonNum, &capturedAt, &fileName,
		&sha256Hash, &sn.FileSizeBytes, &compressed, &compressedSize,
		&phase, &gameNumber, &opponentTeamName, &isHome,
	); err != nil {
		return Snapshot{}, err
	}

	sn.FileName = SnapshotFileName(fileName)
	sn.SHA256Hash = SHA256Hex(sha256Hash)
	sn.Compressed = compressed == 1
	if compressedSize.Valid {
		sn.CompressedSizeBytes = &compressedSize.Int64
	}
	if phase.Valid {
		metadata := SnapshotMetadata{Phase: SnapshotPhase(phase.String)}
		if gameNumber.Valid {
			value := int(gameNumber.Int64)
			metadata.GameNumber = &value
		}
		if opponentTeamName.Valid {
			value := opponentTeamName.String
			metadata.OpponentTeamName = &value
		}
		if isHome.Valid {
			value := isHome.Int64 == 1
			metadata.IsHome = &value
		}
		sn.Metadata = &metadata
	}
	t, _ := time.Parse("2006-01-02T15:04:05Z", capturedAt)
	sn.CapturedAt = t
	return sn, nil
}

func snapshotMetadataValues(metadata *SnapshotMetadata) (phase, gameNumber, opponentTeamName, isHome any) {
	if metadata == nil {
		return nil, nil, nil, nil
	}
	phase = string(metadata.Phase)
	if metadata.GameNumber != nil {
		gameNumber = *metadata.GameNumber
	}
	if metadata.OpponentTeamName != nil {
		opponentTeamName = *metadata.OpponentTeamName
	}
	if metadata.IsHome != nil {
		isHome = boolToInt(*metadata.IsHome)
	}
	return phase, gameNumber, opponentTeamName, isHome
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
