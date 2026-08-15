package dps

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/uija/eqdps/internal/data"
)

type CastSpell struct {
	name      string
	timestamp time.Time
}

type Combat struct {
	activeFights  map[string]*Fight
	knownPlayers  map[string]bool
	blockedNames  map[string]bool
	lastCastSpell map[string]CastSpell
	history       []*Fight

	mu sync.RWMutex
}

func newCombat() Combat {
	return Combat{
		activeFights:  make(map[string]*Fight),
		knownPlayers:  make(map[string]bool),
		blockedNames:  make(map[string]bool),
		lastCastSpell: make(map[string]CastSpell),
		history:       make([]*Fight, 0),
	}
}

func (c *Combat) findFightFor(name string) (*Fight, bool) {
	for _, fight := range c.activeFights {
		if fight.hasParticipant(name) {
			return fight, true
		}
	}
	return nil, false
}
func (c *Combat) findUnvalidatedFightFor(name string) (*Fight, bool) {
	for _, fight := range c.activeFights {
		if !fight.validated && fight.hasParticipant(name) {
			return fight, true
		}
	}
	return nil, false
}
func (c *Combat) endTimedOutFights(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	changed := false
	for name, fight := range c.activeFights {
		if now.Sub(fight.end) > 20*time.Second {
			fight.endReason = END_REASON_TIMEOUT
			delete(c.activeFights, name)
			changed = true
		}
	}
	return changed
}
func (c *Combat) getActiveFight(source string, target string) *Fight {
	player := source
	npc := target
	validated := false
	if source == "you" {
		player = source
		npc = target
		validated = true
	} else if target == "you" {
		player = target
		npc = source
		validated = true
	} else if _, ok := c.knownPlayers[source]; ok {
		player = source
		npc = target
		validated = true
	} else if _, ok := c.knownPlayers[target]; ok {
		player = target
		npc = source
		validated = true
	} else {
		source_words := len(strings.Split(source, " "))
		target_words := len(strings.Split(target, " "))
		if source_words == 1 && target_words > 1 {
			player = source
			npc = target
			if !c.blockedNames[player] {
				validated = true
			}
		} else if target_words == 1 && source_words > 1 {
			player = target
			npc = source
			if !c.blockedNames[player] {
				validated = true
			}
		}
	}
	if player == "you" {
		c.blockedNames[npc] = true
		if c.knownPlayers[npc] {
			delete(c.knownPlayers, npc)
			for name, f := range c.activeFights {
				if name != npc && f.hasParticipant(npc) && f.validated {
					c.activeFights[name].validated = false
				}
			}
		}
	}

	fight_name := evaluateFightName(npc)
	if validated {
		c.knownPlayers[player] = true
		fight, ok := c.activeFights[fight_name]
		if ok {
			fight.validated = true
			return fight
		}
		// see if we find a fight, that is not validated that contains the target
		fight, ok = c.findUnvalidatedFightFor(fight_name)
		if ok {
			// we have a fight thats under the wrong name
			delete(c.activeFights, fight.name)
			fight.name = fight_name
			fight.validated = true
			c.activeFights[fight_name] = fight
			return fight
		}
		c.activeFights[fight_name] = newFight(true)
		c.activeFights[fight_name].name = fight_name
		c.history = append(c.history, c.activeFights[fight_name])
		return c.activeFights[fight_name]
	}
	if fight, ok := c.activeFights[fight_name]; ok {
		return fight
	}
	if fight, ok := c.findFightFor(fight_name); ok {
		return fight
	}
	if fight, ok := c.findFightFor(player); ok {
		return fight
	}
	c.activeFights[fight_name] = newFight(false)
	c.activeFights[fight_name].name = fight_name
	c.history = append(c.history, c.activeFights[fight_name])
	return c.activeFights[fight_name]
}

// getActiveFightLegacy selects the fight using the endpoint rules from the
// legacy FightTracker.mobForEvent implementation. Unlike getActiveFight, it
// never searches active fights by participant: each event is assigned to the
// fight keyed by the endpoint that was identified as the mob.
func (c *Combat) getActiveFightLegacy(source string, target string) *Fight {
	sourceFightName := c.legacyFightName(source)
	targetFightName := c.legacyFightName(target)
	sourceIsMob := c.activeFights[sourceFightName] != nil
	targetIsMob := c.activeFights[targetFightName] != nil
	sourceIsPlayer := c.knownPlayers[source]
	targetIsPlayer := c.knownPlayers[target]

	mob := target
	switch {
	case target == "you":
		mob = source
	case source == "you":
		mob = target
	case sourceIsMob && !targetIsMob:
		mob = source
	case targetIsMob:
		mob = target
	case sourceIsPlayer && !targetIsPlayer:
		mob = target
	case targetIsPlayer:
		mob = source
	}

	if mob == source {
		if !targetIsMob {
			c.knownPlayers[target] = true
		}
	} else if !sourceIsMob {
		c.knownPlayers[source] = true
	}

	fightName := c.legacyFightName(mob)
	if fight := c.activeFights[fightName]; fight != nil {
		return fight
	}

	fight := newFight(true)
	fight.name = fightName
	c.activeFights[fightName] = fight
	c.history = append(c.history, fight)
	return fight
}

// legacyFightName mirrors FightTracker.mobIdentity. Names ending in " pet"
// always identify their owner. Possessive pet names identify their owner only
// while that owner's fight is active.
func (c *Combat) legacyFightName(name string) string {
	trimmed := strings.TrimSpace(name)
	if owner, ok := strings.CutSuffix(trimmed, " pet"); ok && owner != "" {
		return strings.TrimSpace(owner)
	}
	for _, separator := range []string{"`s ", "'s "} {
		owner, petName, ok := strings.Cut(trimmed, separator)
		if ok && owner != "" && petName != "" && c.activeFights[owner] != nil {
			return owner
		}
	}
	return trimmed
}

func evaluateFightName(name string) string {
	if ret, ok := strings.CutSuffix(name, " pet"); ok {
		return ret
	}
	if ret, ok := strings.CutSuffix(name, "`s warder"); ok {
		return ret
	}
	return name
}

func (c *Combat) AddEvent(e *data.LogRowEvent) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	event, ok := c.damageFromLogRow(e)
	if ok {
		fight := c.getActiveFightLegacy(event.NormalizedSource, event.NormalizedTarget)
		fight.addDamageEvent(event)
		return true
	}
	switch e.Type {
	case data.LogRowEventTypeCast:
		caster := normalizeName(e.Data[1])
		c.lastCastSpell[caster] = CastSpell{
			name:      e.Data[2],
			timestamp: e.Timestamp,
		}
	case data.LogRowEventTypeAggroClear:
		for _, fight := range c.activeFights {
			fight.end = e.Timestamp
			fight.endReason = END_REASON_FD
		}
		c.activeFights = make(map[string]*Fight)
	case data.LogRowEventTypeZoneChange:
		for _, fight := range c.activeFights {
			fight.end = e.Timestamp
			fight.endReason = END_REASON_ZONED
		}
		c.activeFights = make(map[string]*Fight)
	case data.LogRowEventTypeSlainBy:
		target := normalizeName(e.Data[1])
		if target == "you" {
			for _, fight := range c.activeFights {
				fight.end = e.Timestamp
				fight.endReason = END_REASON_DEATH
			}
			c.activeFights = make(map[string]*Fight)
		} else {
			if fight, ok := c.activeFights[target]; ok {
				fight.end = e.Timestamp
				fight.endReason = e.Data[2]
				delete(c.activeFights, target)
			}
		}
	case data.LogRowEventTypeYouSlain:
		target := normalizeName(e.Data[1])
		if fight, ok := c.activeFights[target]; ok {
			fight.end = e.Timestamp
			fight.endReason = "You"
			delete(c.activeFights, target)
		}
	case data.LogRowEventTypeSomeoneDied:
		target := normalizeName(e.Data[1])
		if target == "you" {
			for _, fight := range c.activeFights {
				fight.end = e.Timestamp
				fight.endReason = END_REASON_DEATH
			}
			c.activeFights = make(map[string]*Fight)
		} else {
			if fight, ok := c.activeFights[target]; ok {
				fight.end = e.Timestamp
				fight.endReason = "Unknown"
				delete(c.activeFights, target)
			}
		}
	default:
		return false
	}
	return true
}
func (c *Combat) damageFromLogRow(e *data.LogRowEvent) (*DamageEvent, bool) {
	if e == nil {
		return nil, false
	}

	switch e.Type {
	case data.LogRowEventTypeDamage:
		if len(e.Data) < 8 {
			return nil, false
		}
		de := NewDamageEvent(e.Timestamp, e.Data[1], e.Data[3], e.Data[2], e.Data[4], e.Data[6], e.Data[7], e.Type)
		if de.isSpell() {
			if cs, ok := c.lastCastSpell[de.NormalizedSource]; ok && cs.name == de.Ability {
				elapsed := de.Time.Sub(cs.timestamp)
				if elapsed >= 0 && elapsed <= 10*time.Second {
					de.IsCast = true
					if de.Source == "You" {
						de.Participation = true
					}
				}
			}
		} else {
			if de.Source == "You" {
				de.Participation = true
			}
			de.Ability = de.Verb
		}
		return de, true
		// Your melee: You pierce <a mob name> for 10 points of damage.
		// Incoming melee: <a mob name> hits YOU for 10 points of damage.
		// Another player's melee: <a player name> slashes <a mob name> for 10 points of damage.
		// A mob attacking another player: <a mob name> hits <a player name> for 10 points of damage.
		//
		// Direct spell or proc: You hit <a mob name> for 10 points of magic damage by Fireball.
		// Special melee ability: You frenzy on <a mob name> for 10 points of damage.
		// Annotated result: You crush <a mob name> for 10 points of damage. (Critical)
		// Other annotations include (Finishing Blow) and (Riposte Critical).
	case data.LogRowEventTypeDamageOverTime:
		if len(e.Data) < 6 {
			return nil, false
		}
		return NewDamageEvent(e.Timestamp, e.Data[4], e.Data[1], "", e.Data[2], e.Data[3], e.Data[5], e.Type), true
		// <a mob name> has taken 20 damage from Ignite by <a player name>.
	case data.LogRowEventTypeDamageShield:
		if len(e.Data) < 7 {
			return nil, false
		}
		return NewDamageEvent(e.Timestamp, e.Data[2], e.Data[1], "", e.Data[4], e.Data[3], e.Data[6], e.Type), true
		// <a mob name> is burned by <a player name>'s Shield of Flame for 20 points of non-melee damage.
	case data.LogRowEventTypeYourDamageOverTime:
		if len(e.Data) < 5 {
			return nil, false
		}
		return NewDamageEvent(e.Timestamp, "You", e.Data[1], "", e.Data[2], e.Data[3], e.Data[4], data.LogRowEventTypeDamageOverTime), true
		// <a mob name> has taken 20 damage from your Ignite.
	case data.LogRowEventTypeYourDamageShield:
		if len(e.Data) < 6 {
			return nil, false
		}
		return NewDamageEvent(e.Timestamp, "You", e.Data[1], "", e.Data[3], e.Data[2], e.Data[5], data.LogRowEventTypeDamageShield), true
		// <a mob name> is burned by YOUR Shield of Flame for 20 points of non-melee damage.
	default:
	}
	return nil, false
}
func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
func extractSourceAndTarget(e *data.LogRowEvent, sidx, tidx int) (string, string, error) {
	if sidx >= len(e.Data) || tidx >= len(e.Data) {
		return "", "", fmt.Errorf("index out of bounds")
	}
	return e.Data[sidx], e.Data[tidx], nil
}
