package statistics

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/data"
)

type combatInterval struct{ Start, End int64 }
type visitCombatCheckpoint struct {
	Fights  map[string]combatInterval
	Closed  []combatInterval
	LastRow int64
}
type visitCombatTracker struct {
	visitID         int64
	entered         int64
	damage, seconds int64
	checkpoint      visitCombatCheckpoint
}

func (m *Module) combatTimeout() time.Duration {
	if m.ctx != nil && m.ctx.Config != nil && m.ctx.Config.CombatTimeout >= 20 {
		return time.Duration(m.ctx.Config.CombatTimeout) * time.Second
	}
	return 40 * time.Second
}

func (i *Import) visitCombat(visitID int64) (*visitCombatTracker, error) {
	if i.combat != nil && i.combat.visitID == visitID {
		return i.combat, nil
	}
	if err := i.flushCombat(); err != nil {
		return nil, err
	}
	tx, err := i.activeTx()
	if err != nil {
		return nil, err
	}
	c := &visitCombatTracker{visitID: visitID}
	var entered time.Time
	var checkpoint string
	if err := tx.QueryRow(`SELECT entered_at, total_damage, combat_seconds, combat_checkpoint FROM zone_visits WHERE id = ?`, visitID).Scan(&entered, &c.damage, &c.seconds, &checkpoint); err != nil {
		return nil, fmt.Errorf("load visit combat: %w", err)
	}
	c.entered = entered.Unix()
	if err := json.Unmarshal([]byte(checkpoint), &c.checkpoint); err != nil {
		return nil, fmt.Errorf("decode visit combat checkpoint: %w", err)
	}
	if c.checkpoint.Fights == nil {
		c.checkpoint.Fights = make(map[string]combatInterval)
	}
	i.combat = c
	return c, nil
}

func (i *Import) flushCombat() error {
	if i.combat == nil {
		return nil
	}
	tx, err := i.activeTx()
	if err != nil {
		return err
	}
	c := i.combat
	b, err := json.Marshal(c.checkpoint)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE zone_visits SET total_damage=?, combat_seconds=?, combat_checkpoint=? WHERE id=?`, c.damage, c.seconds, string(b), c.visitID)
	if err != nil {
		return fmt.Errorf("save visit combat: %w", err)
	}
	return nil
}

func mergeCombatIntervals(intervals []combatInterval) []combatInterval {
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Start < intervals[j].Start })
	out := make([]combatInterval, 0, len(intervals))
	for _, p := range intervals {
		if p.End <= p.Start {
			continue
		}
		if len(out) > 0 && p.Start <= out[len(out)-1].End {
			out[len(out)-1].End = max(out[len(out)-1].End, p.End)
		} else {
			out = append(out, p)
		}
	}
	return out
}
func (c *visitCombatTracker) intervals() []combatInterval {
	a := append([]combatInterval(nil), c.checkpoint.Closed...)
	for _, p := range c.checkpoint.Fights {
		a = append(a, p)
	}
	return mergeCombatIntervals(a)
}
func intervalSeconds(a []combatInterval) int64 {
	var n int64
	for _, p := range a {
		n += p.End - p.Start
	}
	return n
}
func (c *visitCombatTracker) finish(name string, at int64) {
	if p, ok := c.checkpoint.Fights[name]; ok {
		if at > 0 {
			p.End = max(p.End, at+1)
		}
		c.checkpoint.Closed = append(c.checkpoint.Closed, p)
		delete(c.checkpoint.Fights, name)
	}
}
func (c *visitCombatTracker) expire(now int64, timeout time.Duration) {
	for name, p := range c.checkpoint.Fights {
		if now-(p.End-1) > int64(timeout/time.Second) {
			c.finish(name, 0)
		}
	}
}
func (c *visitCombatTracker) account(before int64) {
	merged := c.intervals()
	c.seconds += intervalSeconds(merged) - before
	// Only intervals which an unfinished fight could extend need to survive
	// the checkpoint. Older durations are already included in seconds.
	c.checkpoint.Closed = nil
	if len(c.checkpoint.Fights) > 0 {
		first := int64(1<<63 - 1)
		for _, p := range c.checkpoint.Fights {
			first = min(first, p.Start)
		}
		for _, p := range merged {
			if p.End >= first {
				c.checkpoint.Closed = append(c.checkpoint.Closed, p)
			}
		}
	}
}
func (i *Import) closeVisitCombat(visitID int64, at time.Time, timeout time.Duration) error {
	c, err := i.visitCombat(visitID)
	if err != nil {
		return err
	}
	before := intervalSeconds(c.intervals())
	c.expire(at.Unix(), timeout)
	for name := range c.checkpoint.Fights {
		c.finish(name, at.Unix()-1)
	}
	// Entry and exit use an exclusive end; clip the final inclusive second.
	for j, p := range c.checkpoint.Closed {
		p.End = min(p.End, at.Unix())
		p.Start = max(p.Start, c.entered)
		c.checkpoint.Closed[j] = p
	}
	c.account(before)
	c.seconds = max(0, c.seconds)
	return i.flushCombat()
}

func (m *Module) importSessionCombat(e *data.LogRowEvent) error {
	if m.currentVisit < 1 {
		return nil
	}
	c, err := m.activeImport.visitCombat(m.currentVisit)
	if err != nil {
		return err
	}
	now := e.Timestamp.Unix()
	before := intervalSeconds(c.intervals())
	if c.checkpoint.LastRow != 0 && now < c.checkpoint.LastRow {
		for name := range c.checkpoint.Fights {
			c.finish(name, 0)
		}
	}
	c.expire(now, m.combatTimeout())
	c.checkpoint.LastRow = now
	own := func(name string) bool {
		return strings.EqualFold(strings.TrimSpace(name), "You") || (m.characterName != "" && strings.EqualFold(strings.TrimSpace(name), m.characterName))
	}
	key := func(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
	source, target, amount := "", "", ""
	d := e.Data
	switch e.Type {
	case data.LogRowEventTypeDamage:
		if len(d) >= 8 {
			source, target, amount = d[1], d[3], d[4]
		}
	case data.LogRowEventTypeYourDamageOverTime:
		if len(d) >= 5 {
			source, target, amount = "You", d[1], d[2]
		}
	case data.LogRowEventTypeDamageOverTime:
		if len(d) >= 6 {
			source, target, amount = d[4], d[1], d[2]
		}
	case data.LogRowEventTypeYourDamageShield:
		if len(d) >= 4 {
			source, target, amount = "You", d[1], d[3]
		}
	case data.LogRowEventTypeDamageShield:
		if len(d) >= 5 {
			source, target, amount = d[2], d[1], d[4]
		}
	case data.LogRowEventTypeFailedMelee:
		if len(d) >= 4 {
			source, target = "You", d[2]
		}
	case data.LogRowEventTypeFailedMeleeOthers:
		if len(d) >= 5 {
			source, target = d[1], d[3]
		}
	case data.LogRowEventTypeYouSlain:
		if len(d) >= 2 {
			c.finish(key(d[1]), now)
		}
	case data.LogRowEventTypeSlainBy, data.LogRowEventTypeSomeoneDied:
		if len(d) >= 2 {
			if own(d[1]) {
				for name := range c.checkpoint.Fights {
					c.finish(name, now)
				}
			} else {
				c.finish(key(d[1]), now)
			}
		}
	case data.LogRowEventTypeAggroClear:
		for name := range c.checkpoint.Fights {
			c.finish(name, now)
		}
	}
	if source != "" && target != "" && own(source) != own(target) {
		valid := true
		var damage int64
		if amount != "" {
			damage, err = strconv.ParseInt(amount, 10, 64)
			valid = err == nil && damage >= 0
		}
		if valid {
			if own(source) {
				c.damage += damage
			}
			mob := key(target)
			if own(target) {
				mob = key(source)
			}
			if now >= c.entered {
				p, ok := c.checkpoint.Fights[mob]
				if !ok {
					p = combatInterval{Start: now}
				}
				p.End = max(p.End, now+1)
				c.checkpoint.Fights[mob] = p
			}
		}
	}
	c.account(before)
	return nil
}
