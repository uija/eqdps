package statistics

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/data"
)

// These are inferred evacs: a cast followed within 30 seconds by loading,
// then re-entry into the exact same zone/instance within two minutes.
// Pending observations live in the import transaction so incremental imports
// can stop between the cast, loading message and zone entry.
func (m *Module) importEvacuation(e *data.LogRowEvent) error {
	message := strings.TrimSpace(e.Message)
	isZone := e.Type == data.LogRowEventTypeZoneChange && len(e.Data) >= 2
	isCast := e.Type == data.LogRowEventTypeCast && len(e.Data) >= 3
	isLoading := message == "LOADING, PLEASE WAIT..."
	reset := message == loginMessage || (e.Type == data.LogRowEventTypeSlainBy && len(e.Data) >= 2 && strings.EqualFold(e.Data[1], "You"))
	interrupted := strings.HasSuffix(message, " spell is interrupted.")
	if !isZone && !isCast && !isLoading && !reset && !interrupted {
		return nil
	}
	tx, err := m.activeImport.activeTx()
	if err != nil {
		return err
	}
	if isCast {
		spell := strings.TrimSpace(e.Data[2])
		name := strings.ToLower(spell)
		// Ignore a trailing Roman-numeral rank for recognition, but retain
		// spell verbatim below so ranked interruption messages still match.
		if separator := strings.LastIndexByte(name, ' '); separator >= 0 {
			rank := name[separator+1:]
			if rank != "" && strings.Trim(rank, "ivxlcdm") == "" {
				name = strings.TrimSpace(name[:separator])
			}
		}
		if name != "lesser evacuate" && name != "lesser succor" && name != "evacuate" && name != "succor" &&
			!strings.HasPrefix(name, "evacuate: ") && !strings.HasPrefix(name, "succor: ") {
			return nil
		}
		if m.currentZone < 1 || m.currentVisit < 1 {
			return nil
		}
		_, err = tx.Exec(`INSERT INTO evacuation_casts (zone_id, raw_zone_name, caster, spell, cast_at)
			SELECT zone_id, raw_zone_name, ?, ?, ? FROM zone_visits WHERE id = ?`, strings.TrimSpace(e.Data[1]), spell, e.Timestamp, m.currentVisit)
	} else if interrupted {
		// Match the full interruption message, not a substring of the caster
		// or spell name. An interruption only cancels that caster's candidates.
		_, err = tx.Exec(`UPDATE evacuation_casts SET cancelled = 1
			WHERE completed_at IS NULL AND cancelled = 0 AND
			((caster || '''s ' || spell || ' spell is interrupted.') = ? COLLATE NOCASE
			 OR (caster = 'You' COLLATE NOCASE AND ('Your ' || spell || ' spell is interrupted.') = ? COLLATE NOCASE))`, message, message)
	} else if isLoading {
		_, err = tx.Exec(`UPDATE evacuation_casts SET loading_at = ?
			WHERE completed_at IS NULL AND cancelled = 0 AND loading_at IS NULL
			AND zone_id = ? AND cast_at >= ? AND cast_at <= ?`, e.Timestamp, m.currentZone, e.Timestamp.Add(-30*time.Second), e.Timestamp)
	} else if isZone {
		// Multiple players may be casting: one zone transition counts once.
		_, err = tx.Exec(`UPDATE evacuation_casts SET completed_at = ? WHERE id = (
			SELECT id FROM evacuation_casts WHERE completed_at IS NULL AND cancelled = 0
			AND zone_id = ? AND raw_zone_name = ? COLLATE NOCASE
			AND loading_at >= ? AND loading_at <= ?
			ORDER BY cast_at DESC, id DESC LIMIT 1
		)`, e.Timestamp, m.currentZone, strings.TrimSpace(e.Data[1]), e.Timestamp.Add(-2*time.Minute), e.Timestamp)
		if err == nil {
			_, err = tx.Exec(`UPDATE evacuation_casts SET cancelled = 1 WHERE completed_at IS NULL AND cancelled = 0`)
		}
	} else if reset {
		_, err = tx.Exec(`UPDATE evacuation_casts SET cancelled = 1 WHERE completed_at IS NULL AND cancelled = 0`)
	}
	if err != nil {
		return fmt.Errorf("track statistics evacuation: %w", err)
	}
	return nil
}

func getSessionEvacCount(db *sql.DB, session SessionStatistics) (int64, error) {
	var count int64
	err := db.QueryRow(`SELECT COUNT(*) FROM evacuation_casts
		WHERE zone_id = ? AND completed_at IS NOT NULL AND cast_at >= ? AND cast_at < ?`,
		session.ZoneID, session.EnteredAt, session.EnteredAt.Add(session.Duration)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get session evacuation count: %w", err)
	}
	return count, nil
}
