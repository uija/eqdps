package statistics

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/uija/eqdps/internal/data"
)

type SessionFactionDetails struct {
	Name   string
	Gained int64
	Lost   int64 // Negative sum, not an absolute value.
}

func (s SessionFactionDetails) NetChange() int64 { return s.Gained + s.Lost }

// GetFactionStatistics includes all imported adjustments, even when the zone
// was unknown. Capped observations are not adjustments and are not stored.
func GetFactionStatistics(db *sql.DB) ([]SessionFactionDetails, error) {
	if db == nil {
		return nil, fmt.Errorf("get faction statistics: database is nil")
	}
	rows, err := db.Query(`SELECT name,
		SUM(CASE WHEN adjustment > 0 THEN adjustment ELSE 0 END),
		SUM(CASE WHEN adjustment < 0 THEN adjustment ELSE 0 END)
		FROM faction_adjustments GROUP BY name COLLATE NOCASE ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("get faction statistics: %w", err)
	}
	defer rows.Close()
	result := make([]SessionFactionDetails, 0)
	for rows.Next() {
		var s SessionFactionDetails
		if err := rows.Scan(&s.Name, &s.Gained, &s.Lost); err != nil {
			return nil, fmt.Errorf("scan faction statistics: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read faction statistics: %w", err)
	}
	return result, nil
}

func (m *Module) importFaction(e *data.LogRowEvent) error {
	if len(e.Data) < 3 {
		return unsupportedObservation("statistics faction event has incomplete data")
	}
	name := strings.TrimSpace(e.Data[1])
	if name == "" {
		return unsupportedObservation("statistics faction event has an empty name")
	}
	amount := strings.TrimSpace(e.Data[2])
	if amount == "" {
		if len(e.Data) > 3 && (e.Data[3] == "better" || e.Data[3] == "worse") {
			// Already capped: no adjustment actually took place.
			return nil
		}
		return unsupportedObservation("statistics faction event has no adjustment")
	}
	adjustment, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		return unsupportedObservation("statistics invalid faction adjustment %q", amount)
	}
	tx, err := m.activeImport.activeTx()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO faction_adjustments (name, zone_id, observed_at, adjustment)
		VALUES (?, ?, ?, ?)`, name, nullableInt64(m.currentZonePointer()), e.Timestamp, adjustment)
	if err != nil {
		return fmt.Errorf("store statistics faction adjustment: %w", err)
	}
	return nil
}

func GetSessionFactionDetails(db *sql.DB, session SessionStatistics) ([]SessionFactionDetails, error) {
	if db == nil {
		return nil, fmt.Errorf("get session factions: database is nil")
	}
	if session.ZoneID < 1 || session.EnteredAt.IsZero() || session.Duration <= 0 {
		return nil, fmt.Errorf("get session factions: invalid session")
	}
	rows, err := db.Query(`SELECT name,
		SUM(CASE WHEN adjustment > 0 THEN adjustment ELSE 0 END),
		SUM(CASE WHEN adjustment < 0 THEN adjustment ELSE 0 END)
		FROM faction_adjustments
		WHERE zone_id = ? AND observed_at >= ? AND observed_at < ?
		GROUP BY name COLLATE NOCASE ORDER BY name COLLATE NOCASE`,
		session.ZoneID, session.EnteredAt, session.EnteredAt.Add(session.Duration))
	if err != nil {
		return nil, fmt.Errorf("get session factions: %w", err)
	}
	defer rows.Close()
	result := make([]SessionFactionDetails, 0)
	for rows.Next() {
		var s SessionFactionDetails
		if err := rows.Scan(&s.Name, &s.Gained, &s.Lost); err != nil {
			return nil, fmt.Errorf("scan session faction: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read session factions: %w", err)
	}
	return result, nil
}
