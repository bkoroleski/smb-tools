package service_test

import (
	"context"
	"testing"

	"smb-tools/internal/models"
	"smb-tools/internal/service"
	"smb-tools/internal/store"
)

const (
	userTeamGUID = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	opponentGUID = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	otherTeamGUID = "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
)

type snapshotPhaseReader struct {
	store.SaveGameReader
	progress      models.SaveGameSeasonProgress
	userTeamGUID  string
	regularGames  []models.SaveGameGame
	playoffGames  []models.SaveGamePlayoffGame
	playoffSeries []models.SaveGamePlayoffSeries
	playoffConfig *models.SaveGamePlayoffConfig
}

func (r snapshotPhaseReader) GetSeasonProgress(context.Context, int) (models.SaveGameSeasonProgress, error) {
	return r.progress, nil
}

func (r snapshotPhaseReader) GetUserTeamGUID(context.Context, string) (string, error) {
	return r.userTeamGUID, nil
}

func (r snapshotPhaseReader) GetSeasonSchedule(context.Context, int) ([]models.SaveGameGame, error) {
	return r.regularGames, nil
}

func (r snapshotPhaseReader) GetPlayoffSchedule(context.Context, int) ([]models.SaveGamePlayoffGame, error) {
	return r.playoffGames, nil
}

func (r snapshotPhaseReader) GetPlayoffSeries(context.Context, int) ([]models.SaveGamePlayoffSeries, error) {
	return r.playoffSeries, nil
}

func (r snapshotPhaseReader) GetSeasonPlayoffConfig(context.Context, int) (*models.SaveGamePlayoffConfig, error) {
	return r.playoffConfig, nil
}

func TestDetectSnapshotMetadata_SeasonStates(t *testing.T) {
	tests := []struct {
		name         string
		reader       snapshotPhaseReader
		wantPhase    store.SnapshotPhase
		wantNil      bool
		wantGame     int
		wantOpponent string
		wantHome     *bool
	}{
		{
			name: "post-draft preseason",
			reader: snapshotPhaseReader{
				progress: models.SaveGameSeasonProgress{ScheduledGames: 384},
			},
			wantPhase: store.SnapshotPhasePreseason,
		},
		{
			name: "first regular game at home",
			reader: snapshotPhaseReader{
				progress:     models.SaveGameSeasonProgress{ScheduledGames: 384, CompletedGames: 1},
				userTeamGUID: userTeamGUID,
				regularGames: []models.SaveGameGame{
					{GameNumber: 1, HomeTeamGUID: userTeamGUID, HomeTeamName: "Bards", AwayTeamGUID: opponentGUID, AwayTeamName: "Firebirds"},
				},
			},
			wantPhase:    store.SnapshotPhaseRegularSeason,
			wantGame:     1,
			wantOpponent: "Firebirds",
			wantHome:     boolPointer(true),
		},
		{
			name: "successive regular games use team-relative count and latest away game",
			reader: snapshotPhaseReader{
				progress:     models.SaveGameSeasonProgress{ScheduledGames: 384, CompletedGames: 3},
				userTeamGUID: userTeamGUID,
				regularGames: []models.SaveGameGame{
					{GameNumber: 1, HomeTeamGUID: userTeamGUID, HomeTeamName: "Bards", AwayTeamGUID: opponentGUID, AwayTeamName: "Firebirds"},
					{GameNumber: 2, HomeTeamGUID: opponentGUID, HomeTeamName: "Firebirds", AwayTeamGUID: otherTeamGUID, AwayTeamName: "Hares"},
					{GameNumber: 3, HomeTeamGUID: opponentGUID, HomeTeamName: "Firebirds", AwayTeamGUID: userTeamGUID, AwayTeamName: "Bards"},
				},
			},
			wantPhase:    store.SnapshotPhaseRegularSeason,
			wantGame:     2,
			wantOpponent: "Firebirds",
			wantHome:     boolPointer(false),
		},
		{
			name: "full regular-season schedule",
			reader: snapshotPhaseReader{
				progress: models.SaveGameSeasonProgress{ScheduledGames: 384, CompletedGames: 384},
			},
			wantPhase: store.SnapshotPhaseEndRegularSeason,
		},
		{
			name: "completed season takes precedence",
			reader: snapshotPhaseReader{
				progress: models.SaveGameSeasonProgress{IsComplete: true, ScheduledGames: 384, CompletedGames: 384},
			},
			wantPhase: store.SnapshotPhaseEndSeason,
		},
		{
			name: "unknown schedule state",
			reader: snapshotPhaseReader{
				progress: models.SaveGameSeasonProgress{},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := service.DetectSnapshotMetadata(context.Background(), tt.reader, 2, "league")
			if err != nil {
				t.Fatalf("DetectSnapshotMetadata: %v", err)
			}
			if tt.wantNil {
				if metadata != nil {
					t.Errorf("metadata: got %+v, want nil", metadata)
				}
				return
			}
			assertSnapshotMetadata(t, metadata, tt.wantPhase, tt.wantGame, tt.wantOpponent, tt.wantHome)
		})
	}
}

func TestDetectSnapshotMetadata_PlayoffStates(t *testing.T) {
	threeGames := &models.SaveGamePlayoffConfig{SeriesLength: 3}
	fiveGames := &models.SaveGamePlayoffConfig{SeriesLength: 5}
	userSeries := models.SaveGamePlayoffSeries{
		SeriesNum: 1,
		Team1GUID: userTeamGUID,
		Team1Name: "Bards",
		Team2GUID: opponentGUID,
		Team2Name: "Firebirds",
	}
	otherSeries := models.SaveGamePlayoffSeries{
		SeriesNum: 0,
		Team1GUID: opponentGUID,
		Team1Name: "Firebirds",
		Team2GUID: otherTeamGUID,
		Team2Name: "Hares",
	}

	tests := []struct {
		name         string
		games        []models.SaveGamePlayoffGame
		series       []models.SaveGamePlayoffSeries
		config       *models.SaveGamePlayoffConfig
		wantPhase    store.SnapshotPhase
		wantGame     int
		wantOpponent string
		wantHome     *bool
	}{
		{
			name: "active series latest home game",
			games: []models.SaveGamePlayoffGame{
				playoffGame(1, 1, opponentGUID, "Firebirds", userTeamGUID, "Bards", 1, 3),
				playoffGame(1, 2, userTeamGUID, "Bards", opponentGUID, "Firebirds", 4, 2),
			},
			series:       []models.SaveGamePlayoffSeries{userSeries},
			config:       fiveGames,
			wantPhase:    store.SnapshotPhasePlayoffs,
			wantGame:     2,
			wantOpponent: "Firebirds",
			wantHome:     boolPointer(true),
		},
		{
			name: "game number resets in new series",
			games: []models.SaveGamePlayoffGame{
				playoffGame(1, 3, userTeamGUID, "Bards", opponentGUID, "Firebirds", 5, 1),
				playoffGame(4, 1, otherTeamGUID, "Hares", userTeamGUID, "Bards", 2, 4),
			},
			series: []models.SaveGamePlayoffSeries{
				userSeries,
				{SeriesNum: 4, Team1GUID: userTeamGUID, Team1Name: "Bards", Team2GUID: otherTeamGUID, Team2Name: "Hares"},
			},
			config:       fiveGames,
			wantPhase:    store.SnapshotPhasePlayoffs,
			wantGame:     1,
			wantOpponent: "Hares",
			wantHome:     boolPointer(false),
		},
		{
			name: "qualified series has not started",
			games: []models.SaveGamePlayoffGame{
				playoffGame(0, 1, opponentGUID, "Firebirds", otherTeamGUID, "Hares", 3, 2),
			},
			series:    []models.SaveGamePlayoffSeries{otherSeries, userSeries},
			config:    fiveGames,
			wantPhase: store.SnapshotPhasePlayoffs,
		},
		{
			name: "user team eliminated",
			games: []models.SaveGamePlayoffGame{
				playoffGame(1, 1, userTeamGUID, "Bards", opponentGUID, "Firebirds", 1, 2),
				playoffGame(1, 2, opponentGUID, "Firebirds", userTeamGUID, "Bards", 3, 1),
			},
			series:    []models.SaveGamePlayoffSeries{userSeries},
			config:    threeGames,
			wantPhase: store.SnapshotPhasePlayoffsEliminated,
		},
		{
			name: "user team did not qualify",
			games: []models.SaveGamePlayoffGame{
				playoffGame(0, 1, opponentGUID, "Firebirds", otherTeamGUID, "Hares", 3, 2),
			},
			series:    []models.SaveGamePlayoffSeries{otherSeries},
			config:    fiveGames,
			wantPhase: store.SnapshotPhasePlayoffsEliminated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := snapshotPhaseReader{
				progress:      models.SaveGameSeasonProgress{ScheduledGames: 384, CompletedGames: 384},
				userTeamGUID:  userTeamGUID,
				playoffGames:  tt.games,
				playoffSeries: tt.series,
				playoffConfig: tt.config,
			}
			metadata, err := service.DetectSnapshotMetadata(context.Background(), reader, 2, "league")
			if err != nil {
				t.Fatalf("DetectSnapshotMetadata: %v", err)
			}
			assertSnapshotMetadata(t, metadata, tt.wantPhase, tt.wantGame, tt.wantOpponent, tt.wantHome)
		})
	}
}

func playoffGame(
	seriesNum int,
	gameNumber int,
	homeGUID string,
	homeName string,
	awayGUID string,
	awayName string,
	homeScore int,
	awayScore int,
) models.SaveGamePlayoffGame {
	return models.SaveGamePlayoffGame{
		SeriesNum:     seriesNum,
		GameNumber:    gameNumber,
		HomeTeamGUID:  homeGUID,
		HomeTeamName:  homeName,
		AwayTeamGUID:  awayGUID,
		AwayTeamName:  awayName,
		HomeScore:     &homeScore,
		AwayScore:     &awayScore,
	}
}

func assertSnapshotMetadata(
	t *testing.T,
	metadata *store.SnapshotMetadata,
	wantPhase store.SnapshotPhase,
	wantGame int,
	wantOpponent string,
	wantHome *bool,
) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata: got nil, want populated metadata")
	}
	if metadata.Phase != wantPhase {
		t.Errorf("phase: got %q, want %q", metadata.Phase, wantPhase)
	}
	if wantGame == 0 {
		if metadata.GameNumber != nil || metadata.OpponentTeamName != nil || metadata.IsHome != nil {
			t.Errorf("game metadata: got %+v, want no game details", metadata)
		}
		return
	}
	if metadata.GameNumber == nil || *metadata.GameNumber != wantGame {
		t.Errorf("game number: got %v, want %d", metadata.GameNumber, wantGame)
	}
	if metadata.OpponentTeamName == nil || *metadata.OpponentTeamName != wantOpponent {
		t.Errorf("opponent: got %v, want %q", metadata.OpponentTeamName, wantOpponent)
	}
	if metadata.IsHome == nil || wantHome == nil || *metadata.IsHome != *wantHome {
		t.Errorf("is home: got %v, want %v", metadata.IsHome, wantHome)
	}
}

func boolPointer(value bool) *bool {
	return &value
}
