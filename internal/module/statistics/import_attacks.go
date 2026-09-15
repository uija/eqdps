package statistics

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/uija/eqdps/internal/data"
)

func (m *Module) importAttack(e *data.LogRowEvent) error {
	var source, name, amount, annotation, failure string
	category := data.CATEGORY_MELEE
	switch e.Type {
	case data.LogRowEventTypeDamage:
		if len(e.Data) < 8 {
			return unsupportedObservation("statistics damage event has incomplete data")
		}
		source, name, amount, annotation = e.Data[1], e.Data[2], e.Data[4], e.Data[7]
		if (name == "hit" || name == "hits") && strings.TrimSpace(e.Data[6]) != "" {
			name, category = e.Data[6], data.CATEGORY_SPELLS
		}
	case data.LogRowEventTypeYourDamageOverTime:
		if len(e.Data) < 5 {
			return unsupportedObservation("statistics DoT event has incomplete data")
		}
		source, name, amount, annotation = "You", e.Data[3], e.Data[2], e.Data[4]
		category = data.CATEGORY_DOTS
	case data.LogRowEventTypeDamageOverTime:
		if len(e.Data) < 6 {
			return unsupportedObservation("statistics DoT event has incomplete data")
		}
		source, name, amount, annotation = e.Data[4], e.Data[3], e.Data[2], e.Data[5]
		category = data.CATEGORY_DOTS
	case data.LogRowEventTypeFailedMelee:
		if len(e.Data) < 4 {
			return unsupportedObservation("statistics failed melee event has incomplete data")
		}
		source, name, failure = "You", e.Data[1], e.Data[3]
	case data.LogRowEventTypeFailedMeleeOthers:
		if len(e.Data) < 5 {
			return unsupportedObservation("statistics failed melee event has incomplete data")
		}
		source, name, failure = e.Data[1], e.Data[2], e.Data[4]
	default:
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(source), "You") &&
		(m.characterName == "" || !strings.EqualFold(strings.TrimSpace(source), m.characterName)) {
		return nil
	}
	name = strings.TrimSpace(name)
	if category == data.CATEGORY_MELEE {
		// Third-person log verbs must share the row used by "You try to ...".
		name = strings.ToLower(name)
		switch name {
		case "frenzies on":
			name = "frenzy on"
		case "bashes", "slashes", "crushes", "punches", "smashes":
			name = strings.TrimSuffix(name, "es")
		default:
			name = strings.TrimSuffix(name, "s")
		}
	}
	if name == "" {
		return unsupportedObservation("statistics attack has an empty name")
	}
	if failure != "" {
		column := ""
		failure = strings.ToLower(strings.TrimSpace(failure))
		switch {
		case failure == "miss" || failure == "misses":
			column = "miss_count"
		case strings.HasSuffix(failure, " magical skin absorbs the blow"):
			column = "absorb_count"
		default:
			words := strings.Fields(failure)
			if len(words) > 0 {
				switch words[len(words)-1] {
				case "dodge", "dodges":
					column = "dodge_count"
				case "parry", "parries":
					column = "parry_count"
				case "block", "blocks":
					column = "block_count"
				case "riposte", "ripostes":
					column = "riposte_count"
				}
			}
		}
		if column == "" {
			return unsupportedObservation("statistics unknown melee failure %q", failure)
		}
		tx, err := m.activeImport.activeTx()
		if err != nil {
			return err
		}
		// column is chosen exclusively from the fixed names above.
		_, err = tx.Exec(`INSERT INTO attack_statistics (name, category, `+column+`)
			VALUES (?, ?, 1) ON CONFLICT (category, name) DO UPDATE SET
			`+column+` = attack_statistics.`+column+` + 1`, name, category)
		if err != nil {
			return fmt.Errorf("store statistics attack failure: %w", err)
		}
		return nil
	}
	damage, err := strconv.ParseInt(amount, 10, 64)
	if err != nil || damage < 0 {
		return unsupportedObservation("statistics invalid attack damage %q", amount)
	}
	return m.activeImport.addAttackHit(name, category, damage, strings.Contains(annotation, "Critical"))
}

func (i *Import) addAttackHit(name, category string, damage int64, critical bool) error {
	tx, err := i.activeTx()
	if err != nil {
		return err
	}
	prefix := "normal"
	if critical {
		prefix = "crit"
	}
	_, err = tx.Exec(`INSERT INTO attack_statistics
		(name, category, `+prefix+`_count, `+prefix+`_damage, `+prefix+`_min, `+prefix+`_max)
		VALUES (?, ?, 1, ?, ?, ?)
		ON CONFLICT (category, name) DO UPDATE SET
		`+prefix+`_count = attack_statistics.`+prefix+`_count + 1,
		`+prefix+`_damage = attack_statistics.`+prefix+`_damage + excluded.`+prefix+`_damage,
		`+prefix+`_min = MIN(COALESCE(attack_statistics.`+prefix+`_min, excluded.`+prefix+`_min), excluded.`+prefix+`_min),
		`+prefix+`_max = MAX(COALESCE(attack_statistics.`+prefix+`_max, excluded.`+prefix+`_max), excluded.`+prefix+`_max)`,
		name, category, damage, damage, damage)
	if err != nil {
		return fmt.Errorf("store statistics attack hit: %w", err)
	}
	return nil
}
