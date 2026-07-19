package service

import (
	"context"
	"fmt"
	"strings"

	"smb-tools/internal/models"
	"smb-tools/internal/store"
)

// DetectSnapshotMetadata classifies the current season and, for active play,
// describes the user-controlled team's latest completed game. A nil result
// means the available save state is not sufficient for a reliable label.
func DetectSnapshotMetadata(
	ctx context.Context,
	reader store.SaveGameReader,
	seasonID int,
	leagueGUID string,
) (*store.SnapshotMetadata, error) {
	progress, err := reader.GetSeasonProgress(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	if progress.IsComplete {
		return phaseOnlyMetadata(store.SnapshotPhaseEndSeason), nil
	}

	playoffGames, err := reader.GetPlayoffSchedule(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	if len(playoffGames) > 0 {
		return detectPlayoffMetadata(ctx, reader, seasonID, leagueGUID, playoffGames)
	}

	if progress.ScheduledGames <= 0 {
		return nil, nil
	}
	if progress.CompletedGames < 0 || progress.CompletedGames > progress.ScheduledGames {
		return nil, fmt.Errorf(
			"invalid regular-season progress for season %d: %d completed of %d scheduled",
			seasonID,
			progress.CompletedGames,
			progress.ScheduledGames,
		)
	}
	if progress.CompletedGames == progress.ScheduledGames {
		return phaseOnlyMetadata(store.SnapshotPhaseEndRegularSeason), nil
	}
	if progress.CompletedGames == 0 {
		return phaseOnlyMetadata(store.SnapshotPhasePreseason), nil
	}

	userTeamGUID, err := reader.GetUserTeamGUID(ctx, leagueGUID)
	if err != nil {
		return nil, err
	}
	regularGames, err := reader.GetSeasonSchedule(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	var latest *models.SaveGameGame
	userGameCount := 0
	for i := range regularGames {
		game := &regularGames[i]
		if !gameIncludesTeam(game.HomeTeamGUID, game.AwayTeamGUID, userTeamGUID) {
			continue
		}
		userGameCount++
		if latest == nil || game.GameNumber > latest.GameNumber {
			latest = game
		}
	}
	if latest == nil {
		return nil, nil
	}

	return gameMetadata(
		store.SnapshotPhaseRegularSeason,
		userGameCount,
		userTeamGUID,
		latest.HomeTeamGUID,
		latest.HomeTeamName,
		latest.AwayTeamName,
	), nil
}

func detectPlayoffMetadata(
	ctx context.Context,
	reader store.SaveGameReader,
	seasonID int,
	leagueGUID string,
	playoffGames []models.SaveGamePlayoffGame,
) (*store.SnapshotMetadata, error) {
	userTeamGUID, err := reader.GetUserTeamGUID(ctx, leagueGUID)
	if err != nil {
		return nil, err
	}
	series, err := reader.GetPlayoffSeries(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	currentSeries := latestTeamPlayoffSeries(series, userTeamGUID)
	if currentSeries == nil {
		return phaseOnlyMetadata(store.SnapshotPhasePlayoffsEliminated), nil
	}

	config, err := reader.GetSeasonPlayoffConfig(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	if config == nil || config.SeriesLength <= 0 {
		return nil, fmt.Errorf("missing playoff series length for season %d", seasonID)
	}

	latest, opponentWins, err := playoffSeriesProgress(playoffGames, currentSeries.SeriesNum, userTeamGUID)
	if err != nil {
		return nil, err
	}

	if opponentWins >= config.SeriesLength/2+1 {
		return phaseOnlyMetadata(store.SnapshotPhasePlayoffsEliminated), nil
	}
	if latest == nil {
		return phaseOnlyMetadata(store.SnapshotPhasePlayoffs), nil
	}

	return gameMetadata(
		store.SnapshotPhasePlayoffs,
		latest.GameNumber,
		userTeamGUID,
		latest.HomeTeamGUID,
		latest.HomeTeamName,
		latest.AwayTeamName,
	), nil
}

func latestTeamPlayoffSeries(
	series []models.SaveGamePlayoffSeries,
	teamGUID string,
) *models.SaveGamePlayoffSeries {
	var latest *models.SaveGamePlayoffSeries
	for i := range series {
		item := &series[i]
		if !gameIncludesTeam(item.Team1GUID, item.Team2GUID, teamGUID) {
			continue
		}
		if latest == nil || item.SeriesNum > latest.SeriesNum {
			latest = item
		}
	}
	return latest
}

func playoffSeriesProgress(
	games []models.SaveGamePlayoffGame,
	seriesNum int,
	userTeamGUID string,
) (latest *models.SaveGamePlayoffGame, opponentWins int, err error) {
	for i := range games {
		game := &games[i]
		if game.SeriesNum != seriesNum {
			continue
		}
		if !gameIncludesTeam(game.HomeTeamGUID, game.AwayTeamGUID, userTeamGUID) {
			return nil, 0, fmt.Errorf("playoff series %d contains a game without the user-controlled team", seriesNum)
		}
		if game.HomeScore == nil || game.AwayScore == nil {
			return nil, 0, fmt.Errorf("playoff series %d has a completed game without a score", seriesNum)
		}
		if *game.HomeScore == *game.AwayScore {
			return nil, 0, fmt.Errorf("playoff series %d has a tied completed game", seriesNum)
		}

		userIsHome := strings.EqualFold(game.HomeTeamGUID, userTeamGUID)
		userWon := (userIsHome && *game.HomeScore > *game.AwayScore) ||
			(!userIsHome && *game.AwayScore > *game.HomeScore)
		if !userWon {
			opponentWins++
		}
		if latest == nil || game.GameNumber > latest.GameNumber {
			latest = game
		}
	}
	return latest, opponentWins, nil
}

func phaseOnlyMetadata(phase store.SnapshotPhase) *store.SnapshotMetadata {
	return &store.SnapshotMetadata{Phase: phase}
}

func gameMetadata(
	phase store.SnapshotPhase,
	gameNumber int,
	userTeamGUID string,
	homeTeamGUID string,
	homeTeamName string,
	awayTeamName string,
) *store.SnapshotMetadata {
	isHome := strings.EqualFold(homeTeamGUID, userTeamGUID)
	opponent := homeTeamName
	if isHome {
		opponent = awayTeamName
	}
	return &store.SnapshotMetadata{
		Phase:            phase,
		GameNumber:       &gameNumber,
		OpponentTeamName: &opponent,
		IsHome:           &isHome,
	}
}

func gameIncludesTeam(homeTeamGUID, awayTeamGUID, teamGUID string) bool {
	return strings.EqualFold(homeTeamGUID, teamGUID) || strings.EqualFold(awayTeamGUID, teamGUID)
}
