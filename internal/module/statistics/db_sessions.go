package statistics

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SessionStatistics struct {
	VisitID          int64
	ZoneID           int64
	Zone             string
	EnteredAt        time.Time
	Duration         time.Duration
	Kills            int64
	ExperienceGained float64
	Motes            int64
	MotesPerHour     float64
}

type SessionMobDetails struct {
	Name        string
	Kills       int64
	KilledByYou int64
}

type SessionLootDetails struct {
	Item     string
	Mob      string
	Quantity int64
}

type SessionDeathDetails struct {
	Mob    string
	Deaths int64
}

type SessionDetails struct {
	Money  int64
	Mobs   []SessionMobDetails
	Loot   []SessionLootDetails
	Deaths []SessionDeathDetails
}

// GetSessionStatistics returns zone sessions lasting at least one minute in
// which the player landed the killing blow at least once. Consecutive visits
// to the same raw zone with no time gap are treated as one session, which
// covers evacuating and zoning back into the same zone. Kills includes all
// confirmed kills observed during the session.
func GetSessionStatistics(db *sql.DB) ([]SessionStatistics, error) {
	if db == nil {
		return nil, errors.New("get session statistics: database is nil")
	}

	rows, err := db.Query(`
		WITH raw_visits AS (
			SELECT
				zone_visits.id,
				zone_visits.zone_id,
				zone_visits.raw_zone_name,
				zone_visits.entered_at,
				COALESCE(
					zone_visits.left_at,
					CASE
						WHEN zone_visits.id = (SELECT MAX(id) FROM zone_visits)
						THEN (SELECT last_timestamp FROM log_state WHERE id = 1)
					END
				) AS ended_at
			FROM zone_visits
		), visit_boundaries AS (
			SELECT
				raw_visits.*,
				CASE
					WHEN zone_id = LAG(zone_id) OVER (ORDER BY id)
						AND LOWER(raw_zone_name) = LOWER(LAG(raw_zone_name) OVER (ORDER BY id))
						AND entered_at = LAG(ended_at) OVER (ORDER BY id)
					THEN 0
					ELSE 1
				END AS starts_session
			FROM raw_visits
		), numbered_visits AS (
			SELECT
				visit_boundaries.*,
				SUM(starts_session) OVER (
					ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
				) AS session_number
			FROM visit_boundaries
		), visits AS (
			SELECT
				MIN(id) AS id,
				zone_id,
				MIN(raw_zone_name) AS raw_zone_name,
				MIN(entered_at) AS entered_at,
				MAX(ended_at) AS ended_at
			FROM numbered_visits
			GROUP BY session_number, zone_id
		)
		SELECT
			visits.id,
			visits.zone_id,
			visits.raw_zone_name,
			unixepoch(visits.entered_at),
			unixepoch(visits.ended_at) - unixepoch(visits.entered_at),
			(
				SELECT COUNT(*)
				FROM kills
				WHERE kills.zone_id = visits.zone_id
					AND kills.killed_at >= visits.entered_at
					AND kills.killed_at < visits.ended_at
					AND kills.kill_type <> 'unknown'
			),
			(
				SELECT COALESCE(SUM(experience.percent), 0)
				FROM experience
				WHERE experience.zone_id = visits.zone_id
					AND experience.received_at >= visits.entered_at
					AND experience.received_at < visits.ended_at
			),
			(
				SELECT COALESCE(SUM(loot.quantity), 0)
				FROM loot
				JOIN items ON items.id = loot.item_id
				WHERE loot.zone_id = visits.zone_id
					AND loot.looted_at >= visits.entered_at
					AND loot.looted_at < visits.ended_at
					AND items.name LIKE 'Mote of %'
			)
		FROM visits
		WHERE visits.ended_at IS NOT NULL
			AND unixepoch(visits.ended_at) - unixepoch(visits.entered_at) >= 60
			AND EXISTS (
				SELECT 1
				FROM kills
				WHERE kills.zone_id = visits.zone_id
					AND kills.killed_at >= visits.entered_at
					AND kills.killed_at < visits.ended_at
					AND kills.kill_type = 'player'
			)
		ORDER BY visits.entered_at DESC, visits.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("get session statistics: %w", err)
	}
	defer rows.Close()

	result := make([]SessionStatistics, 0)
	for rows.Next() {
		var value SessionStatistics
		var enteredAtSeconds int64
		var durationSeconds int64
		if err := rows.Scan(
			&value.VisitID,
			&value.ZoneID,
			&value.Zone,
			&enteredAtSeconds,
			&durationSeconds,
			&value.Kills,
			&value.ExperienceGained,
			&value.Motes,
		); err != nil {
			return nil, fmt.Errorf("scan session statistics: %w", err)
		}
		value.EnteredAt = time.Unix(enteredAtSeconds, 0).In(time.Local)
		value.Duration = time.Duration(durationSeconds) * time.Second
		value.MotesPerHour = float64(value.Motes) / float64(value.Duration.Hours())
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read session statistics: %w", err)
	}
	return result, nil
}

func GetSessionDetails(db *sql.DB, session SessionStatistics) (SessionDetails, error) {
	if db == nil {
		return SessionDetails{}, errors.New("get session details: database is nil")
	}
	if session.ZoneID < 1 || session.EnteredAt.IsZero() || session.Duration <= 0 {
		return SessionDetails{}, errors.New("get session details: invalid session")
	}
	endedAt := session.EnteredAt.Add(session.Duration)
	arguments := []any{session.ZoneID, session.EnteredAt, endedAt}

	var result SessionDetails
	if err := db.QueryRow(`
		SELECT COALESCE(SUM(amount_copper), 0)
		FROM money
		WHERE zone_id = ?
			AND received_at >= ?
			AND received_at < ?
	`, arguments...).Scan(&result.Money); err != nil {
		return SessionDetails{}, fmt.Errorf("get session money: %w", err)
	}

	rows, err := db.Query(`
		SELECT
			mobs.name,
			COUNT(*),
			SUM(CASE WHEN kills.kill_type = 'player' THEN 1 ELSE 0 END)
		FROM kills
		JOIN mobs ON mobs.id = kills.mob_id
		WHERE kills.zone_id = ?
			AND kills.killed_at >= ?
			AND kills.killed_at < ?
			AND kills.kill_type <> 'unknown'
		GROUP BY mobs.id, mobs.name
		ORDER BY COUNT(*) DESC, mobs.name COLLATE NOCASE
	`, arguments...)
	if err != nil {
		return SessionDetails{}, fmt.Errorf("get session mobs: %w", err)
	}
	for rows.Next() {
		var value SessionMobDetails
		if err := rows.Scan(&value.Name, &value.Kills, &value.KilledByYou); err != nil {
			rows.Close()
			return SessionDetails{}, fmt.Errorf("scan session mob: %w", err)
		}
		result.Mobs = append(result.Mobs, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SessionDetails{}, fmt.Errorf("read session mobs: %w", err)
	}
	rows.Close()

	rows, err = db.Query(`
		SELECT
			item_name,
			GROUP_CONCAT(mob_name, ', '),
			SUM(quantity)
		FROM (
			SELECT
				items.id AS item_id,
				items.name AS item_name,
				mobs.name AS mob_name,
				SUM(loot.quantity) AS quantity
			FROM loot
			JOIN items ON items.id = loot.item_id
			JOIN mobs ON mobs.id = loot.mob_id
			WHERE loot.zone_id = ?
				AND loot.looted_at >= ?
				AND loot.looted_at < ?
			GROUP BY items.id, items.name, mobs.id, mobs.name
			ORDER BY mobs.name COLLATE NOCASE
		)
		GROUP BY item_id, item_name
		ORDER BY SUM(quantity) DESC, item_name COLLATE NOCASE
	`, arguments...)
	if err != nil {
		return SessionDetails{}, fmt.Errorf("get session loot: %w", err)
	}
	for rows.Next() {
		var value SessionLootDetails
		if err := rows.Scan(&value.Item, &value.Mob, &value.Quantity); err != nil {
			rows.Close()
			return SessionDetails{}, fmt.Errorf("scan session loot: %w", err)
		}
		result.Loot = append(result.Loot, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SessionDetails{}, fmt.Errorf("read session loot: %w", err)
	}
	rows.Close()

	rows, err = db.Query(`
		SELECT mobs.name, COUNT(*)
		FROM player_deaths
		JOIN mobs ON mobs.id = player_deaths.mob_id
		WHERE player_deaths.zone_id = ?
			AND player_deaths.died_at >= ?
			AND player_deaths.died_at < ?
		GROUP BY mobs.id, mobs.name
		ORDER BY COUNT(*) DESC, mobs.name COLLATE NOCASE
	`, arguments...)
	if err != nil {
		return SessionDetails{}, fmt.Errorf("get session deaths: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value SessionDeathDetails
		if err := rows.Scan(&value.Mob, &value.Deaths); err != nil {
			return SessionDetails{}, fmt.Errorf("scan session death: %w", err)
		}
		result.Deaths = append(result.Deaths, value)
	}
	if err := rows.Err(); err != nil {
		return SessionDetails{}, fmt.Errorf("read session deaths: %w", err)
	}
	return result, nil
}
