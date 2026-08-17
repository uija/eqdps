package dps

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/uija/eqdps/internal/data"
)

const GRACE_PERIOD_TIME = 8 * time.Second

type CastSpell struct {
	name      string
	timestamp time.Time
}
type GracePeriod struct {
	Ends  time.Time
	Fight *data.Fight
}

type Combat struct {
	activeFights      map[string]*data.Fight
	gracePeriodFights map[string]GracePeriod
	knownPlayers      map[string]bool
	blockedNames      map[string]bool
	lastCastSpell     map[string]CastSpell
	history           []*data.Fight

	mu sync.RWMutex
}

func newCombat() *Combat {
	return &Combat{
		activeFights:      make(map[string]*data.Fight),
		gracePeriodFights: make(map[string]GracePeriod),
		knownPlayers:      make(map[string]bool),
		blockedNames:      make(map[string]bool),
		lastCastSpell:     make(map[string]CastSpell),
		history:           make([]*data.Fight, 0),
	}
}

func (c *Combat) findFightFor(name string) (*data.Fight, bool) {
	for _, fight := range c.activeFights {
		if fight.HasParticipant(name) {
			return fight, true
		}
	}
	return nil, false
}
func (c *Combat) findUnvalidatedFightFor(name string) (*data.Fight, bool) {
	for _, fight := range c.activeFights {
		if !fight.Validated && fight.HasParticipant(name) {
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
		if now.Sub(fight.End) > 20*time.Second {
			fight.EndReason = data.END_REASON_TIMEOUT
			delete(c.activeFights, name)
			changed = true
		}
	}
	return changed
}

// getActiveFightLegacy selects the fight using the endpoint rules from the
// legacy FightTracker.mobForEvent implementation. Unlike getActiveFight, it
// never searches active fights by participant: each event is assigned to the
// fight keyed by the endpoint that was identified as the mob.
func (c *Combat) getActiveFightLegacy(event *data.DamageEvent) *data.Fight {
	source := event.NormalizedSource
	target := event.NormalizedTarget
	sourceFightName := c.legacyFightName(source)
	targetFightName := c.legacyFightName(target)
	sourceIsMob := c.activeFights[sourceFightName] != nil
	targetIsMob := c.activeFights[targetFightName] != nil
	sourceIsPlayer := c.knownPlayers[source]
	targetIsPlayer := c.knownPlayers[target]

	if gp, ok := c.gracePeriodFights[sourceFightName]; ok {
		if gp.Fight.End.Equal(event.Time) {
			return gp.Fight
		}
	}
	if gp, ok := c.gracePeriodFights[targetFightName]; ok {
		if gp.Fight.End.Equal(event.Time) {
			return gp.Fight
		}
	}

	switch event.Type {
	case data.LogRowEventTypeDamageOverTime,
		data.LogRowEventTypeDamageShield:
		if gp, ok := c.gracePeriodFights[sourceFightName]; ok {
			gp.Ends = event.Time.Add(GRACE_PERIOD_TIME)
			c.gracePeriodFights[sourceFightName] = gp
			return gp.Fight
		}
		if gp, ok := c.gracePeriodFights[targetFightName]; ok {
			gp.Ends = event.Time.Add(GRACE_PERIOD_TIME)
			c.gracePeriodFights[targetFightName] = gp
			return gp.Fight
		}
	default:
		if _, ok := c.gracePeriodFights[sourceFightName]; ok {
			delete(c.gracePeriodFights, sourceFightName)
		}
		if _, ok := c.gracePeriodFights[targetFightName]; ok {
			delete(c.gracePeriodFights, targetFightName)
		}
	}

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

	fight := data.NewFight(true)
	fight.Name = fightName
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

	for name := range c.gracePeriodFights {
		if e.Timestamp.After(c.gracePeriodFights[name].Ends) {
			delete(c.gracePeriodFights, name)
		}
	}

	event, ok := c.damageFromLogRow(e)
	if ok {
		fight := c.getActiveFightLegacy(event)
		fight.AddDamageEvent(event)
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
		for name, fight := range c.activeFights {
			fight.End = e.Timestamp
			fight.EndReason = data.END_REASON_FD
			c.gracePeriodFights[name] = GracePeriod{
				Fight: fight,
				Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
			}
		}
		c.activeFights = make(map[string]*data.Fight)
	case data.LogRowEventTypeZoneChange:
		for name, fight := range c.activeFights {
			fight.End = e.Timestamp
			fight.EndReason = data.END_REASON_ZONED
			c.gracePeriodFights[name] = GracePeriod{
				Fight: fight,
				Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
			}
		}
		c.activeFights = make(map[string]*data.Fight)
	case data.LogRowEventTypeSlainBy:
		target := normalizeName(e.Data[1])
		if target == "you" {
			for name, fight := range c.activeFights {
				fight.End = e.Timestamp
				fight.EndReason = data.END_REASON_DEATH
				c.gracePeriodFights[name] = GracePeriod{
					Fight: fight,
					Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
				}
			}
			c.activeFights = make(map[string]*data.Fight)
		} else {
			if fight, ok := c.activeFights[target]; ok {
				fight.End = e.Timestamp
				fight.EndReason = e.Data[2]
				delete(c.activeFights, target)
				c.gracePeriodFights[target] = GracePeriod{
					Fight: fight,
					Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
				}
			}
		}
	case data.LogRowEventTypeYouSlain:
		target := normalizeName(e.Data[1])
		if fight, ok := c.activeFights[target]; ok {
			fight.End = e.Timestamp
			fight.EndReason = "You"
			delete(c.activeFights, target)
			c.gracePeriodFights[target] = GracePeriod{
				Fight: fight,
				Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
			}
		}
	case data.LogRowEventTypeSomeoneDied:
		target := normalizeName(e.Data[1])
		if target == "you" {
			for name, fight := range c.activeFights {
				fight.End = e.Timestamp
				fight.EndReason = data.END_REASON_DEATH
				c.gracePeriodFights[name] = GracePeriod{
					Fight: fight,
					Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
				}
			}
			c.activeFights = make(map[string]*data.Fight)
		} else {
			if fight, ok := c.activeFights[target]; ok {
				fight.End = e.Timestamp
				fight.EndReason = "Unknown"
				delete(c.activeFights, target)
				c.gracePeriodFights[target] = GracePeriod{
					Fight: fight,
					Ends:  e.Timestamp.Add(GRACE_PERIOD_TIME),
				}
			}
		}
	default:
		return false
	}
	return true
}
func (c *Combat) damageFromLogRow(e *data.LogRowEvent) (*data.DamageEvent, bool) {
	if e == nil {
		return nil, false
	}

	switch e.Type {
	case data.LogRowEventTypeDamage:
		if len(e.Data) < 8 {
			return nil, false
		}
		de := data.NewDamageEvent(e.Timestamp, e.Data[1], e.Data[3], e.Data[2], e.Data[4], e.Data[6], e.Data[7], e.Type)
		if de.IsSpell() {
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
		return data.NewDamageEvent(e.Timestamp, e.Data[4], e.Data[1], "", e.Data[2], e.Data[3], e.Data[5], e.Type), true
		// <a mob name> has taken 20 damage from Ignite by <a player name>.
	case data.LogRowEventTypeDamageShield:
		if len(e.Data) < 7 {
			return nil, false
		}
		return data.NewDamageEvent(e.Timestamp, e.Data[2], e.Data[1], "", e.Data[4], e.Data[3], e.Data[6], e.Type), true
		// <a mob name> is burned by <a player name>'s Shield of Flame for 20 points of non-melee damage.
	case data.LogRowEventTypeYourDamageOverTime:
		if len(e.Data) < 5 {
			return nil, false
		}
		return data.NewDamageEvent(e.Timestamp, "You", e.Data[1], "", e.Data[2], e.Data[3], e.Data[4], data.LogRowEventTypeDamageOverTime), true
		// <a mob name> has taken 20 damage from your Ignite.
	case data.LogRowEventTypeYourDamageShield:
		if len(e.Data) < 6 {
			return nil, false
		}
		return data.NewDamageEvent(e.Timestamp, "You", e.Data[1], "", e.Data[3], e.Data[2], e.Data[5], data.LogRowEventTypeDamageShield), true
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
