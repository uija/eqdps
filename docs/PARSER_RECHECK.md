# Parser Recheck Guide

This document defines how to audit EverQuest log files against `eqdps` and how
to turn newly discovered message formats into safe parser improvements.

The parser is intentionally attribution-first: damage is only counted when the
log identifies a source. A larger total is not an improvement if damage is
assigned to the wrong combatant.

## Current Reference Baseline

Reference file: `eqlog_Wyrmberg_rivervale.txt`

Baseline measured on 2026-07-16:

| Measurement | Count |
| --- | ---: |
| Log lines | 1,286,318 |
| Parsed damage events | 537,314 |
| Rejected damage-like lines | 260 |
| Deliberately ignored source-less spell/DoT lines | 226 |
| Deliberately ignored source-less non-melee lines | 9 |
| Rejected chat lines containing damage-like text | 25 |

Reference SHA-256:
`a11318ae8d9917bc6fa71d955027f2cf185d5a6ba165e8294d031f7f9d87d658`.

These counts only apply while the reference file is unchanged. Record the file
line count and checksum when using a different or updated corpus:

```bash
wc -l eqlog_Wyrmberg_rivervale.txt
sha256sum eqlog_Wyrmberg_rivervale.txt
```

The known source-less formats are:

```text
A fire giant warrior has taken 18 damage by Denon's Disruptive Discord.
You were hit by non-melee for 10 damage.
```

Do not assign these to `You`, the target, or a guessed caster. They may only be
counted in the future if reliable source inference is designed and tested.

## Formats Currently Supported

`internal/eqlog/parser.go` recognizes:

- Direct melee and proc damage: `Source <verb> Target for N points of damage.`
- Direct spell damage: `Source hit Target for N points of magic damage by Spell.`
- Spell and DoT damage: `Target has taken N damage from Spell by Source.`
- Incoming spell and DoT damage: `You have taken N damage from Spell by Source.`
- Local Bard/DoT damage: `Target has taken N damage from your Spell.`
- Local damage shields: `Target is ... by YOUR thorns for N points ...`
- Other damage shields: `Target is/are ... by Source's thorns/flames/frost ...`
- Optional markers such as `(Critical)`, `(Riposte Critical)`, and
  `(Finishing Blow)`.
- Deaths: `You have slain Target!` and `Victim has been slain by Killer!`.
- Cast starts: `Source begins to cast Ability.` and supported wording variants.
- Experience gains, level-ups, and `Your enemies have forgotten you!`.

Runtime processing uses `ParseRecord`, which parses the timestamp envelope once
and classifies the message as cast, damage, experience, level-up, aggro clear,
death, or unknown. The focused `ParseLine` API remains the full-corpus damage
audit entrypoint.

Direct-damage verbs are an explicit allowlist. Review `damageRE` whenever a log
contains a new verb. Current examples include `frenzy on`, `frenzies on`,
`reaves`, and `smashes` in addition to common attacks such as `hits`, `slashes`,
`pierces`, and `kicks`.

## Recheck Workflow

### 1. Preserve a clean baseline

Before changing parser code:

```bash
git status --short
go test ./...
go vet ./...
```

Do not discard unrelated worktree changes. Note the current parsed event count
for the corpus being audited.

### 2. Find rejected damage-like lines

Scan every line. For each line:

1. Call `eqlog.ParseLine(line)`.
2. Skip accepted lines.
3. Keep rejected lines matching this broad candidate expression:

```text
(?i)([0-9]+ (points? of )?([a-z-]+ )?damage|damage (from|by)|hit(s)? you for [0-9]+)
```

4. Remove the timestamp envelope.
5. Replace numbers with `#` and group identical shapes.
6. Sort by frequency descending and retain one exact example per shape.

Run the audit from the repository root so a temporary Go audit program can
import `github.com/uija/eqdps/internal/eqlog`. A program outside the module
cannot import this internal package. Remove temporary audit files afterward.

Minimal reference program for `audit_parser.go` in the repository root:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/uija/eqdps/internal/eqlog"
)

var envelopeRE = regexp.MustCompile(`^\[[^]]+\] `)
var candidateRE = regexp.MustCompile(
	`(?i)([0-9]+ (points? of )?([a-z-]+ )?damage|damage (from|by)|hit(s)? you for [0-9]+)`,
)
var numberRE = regexp.MustCompile(`[0-9]+`)

type result struct {
	shape string
	count int
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run audit_parser.go LOGFILE")
		os.Exit(2)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer file.Close()

	parsed := 0
	rejected := make(map[string]int)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if _, ok := eqlog.ParseLine(line); ok {
			parsed++
			continue
		}
		message := envelopeRE.ReplaceAllString(line, "")
		if candidateRE.MatchString(message) {
			rejected[numberRE.ReplaceAllString(message, "#")]++
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	results := make([]result, 0, len(rejected))
	total := 0
	for shape, count := range rejected {
		results = append(results, result{shape: shape, count: count})
		total += count
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].count != results[j].count {
			return results[i].count > results[j].count
		}
		return results[i].shape < results[j].shape
	})

	fmt.Printf("parsed=%d rejected=%d shapes=%d\n", parsed, total, len(results))
	for _, item := range results {
		fmt.Printf("%6d  %s\n", item.count, item.shape)
	}
}
```

Run and remove it with:

```bash
go run audit_parser.go eqlog_Wyrmberg_rivervale.txt
rm -f audit_parser.go
```

Do not run `go test ./...` while this temporary root file exists because it
defines another `main` function in the root package.

The candidate expression is deliberately broad and will include chat. Do not
turn a rejected line into a parser rule until an exact log example proves it is
a combat event.

Useful exploratory commands:

```bash
rg -n -i "damage|taken [0-9]+|for [0-9]+ points|hit you for" LOGFILE
rg -n "has taken [0-9]+ damage by" LOGFILE
rg -n "\(Riposte Critical\)" LOGFILE
```

Avoid placing EverQuest backticks inside a double-quoted shell expression;
shell command substitution can execute them. Use a script or safely quoted
pattern when searching names containing backticks.

### 3. Audit accepted events too

Only checking rejected lines finds false negatives. Also inspect accepted
events for false positives and wrong attribution:

- Group accepted events by source, target, attack, ability, flags, and message
  shape. Relevant flags are critical, passive, damage-over-time, and incidental.
- Confirm chat lines are never accepted as combat.
- Confirm `You` is the local outgoing source and `YOU` is the local target.
- Confirm leading articles normalize consistently (`An ire ghast` and
  `an ire ghast` must be one entity).
- Confirm apostrophes remain part of names when appropriate.
- Confirm damage-shield source extraction uses the final possessive suffix.
  `Innoruuk's Chosen's thorns` belongs to `Innoruuk's Chosen`.
- Confirm a possessive combatant is not truncated to its apparent owner.

Compare total damage, event count, crit count, and ability breakdown. A line can
be accepted while still losing its ability name or critical marker.

### 4. Audit fight boundaries separately

Damage parsing and fight detection are separate concerns. Search for death-like
messages and inspect their surrounding lines:

```bash
rg -n -i "slain|have died|has died| dies[.!]|you died" LOGFILE
```

Do not automatically treat `Name dies.` as a death. In the reference log these
messages are generated by Feign Death. Real supported death messages contain
`slain`.

For every mob-boundary change, verify:

- Each mob name has an independent active record; damage involving another mob
  cannot split or close it.
- Same-timestamp damage remains with the mob whose death was just reported.
- Later same-name DoTs are buffered for eight seconds. A non-DoT or second death
  confirms a successor and receives the buffered DoTs; otherwise they return to
  the old record when grace expires.
- A local-player death closes every active mob immediately.
- Each mob closes independently after the configured idle timeout.
- `Your enemies have forgotten you!` closes all active mobs, while attributable
  follow-up DoTs may update the matching completed record for eight seconds
  without reopening combat. A non-DoT creates a new record.
- `<owner> pet` damage routes to an observed owner's record and the pet's death
  does not close that owner record.
- Simultaneous living mobs with the same name remain one active record because
  the log provides no spawn identifier.
- Replay uses log timestamps, while live idle and grace detection use wall-clock
  time.

### 5. Add exact regression tests

Every new accepted format needs an exact line copied from a real log in
`internal/eqlog/parser_test.go`. Assert all meaningful fields:

- timestamp parsing when relevant
- source
- target
- amount
- melee attack verb when present
- ability
- critical state
- passive, damage-over-time, and incidental state when relevant

Also add a negative test when a similar-looking format must remain ignored.
Fight grouping changes belong in `internal/combat/combat_test.go`.

### 6. Measure the effect

After implementation:

```bash
gofmt -w internal/eqlog/parser.go internal/eqlog/parser_test.go
go test ./...
go vet ./...
go build -o /tmp/eqdps-check .
/tmp/eqdps-check --text --since "YYYY-MM-DD HH:MM" LOGFILE
rm -f /tmp/eqdps-check
git diff --check
```

Rerun the full rejection audit. Explain every remaining high-frequency shape.
Record how many events each parser change adds and which combatants it affects.
Distinguish these outcomes:

- Missing entirely: totals, DPS, hits, and breakdown were all wrong.
- Parsed without metadata: total was right, but ability or crit count was wrong.
- Source-less: amount is visible in the log but cannot safely enter a player row.

## Quality Rules

- Prefer exact, anchored message formats over broad substring matching.
- Never infer a source only from a spell name without a documented ownership
  model; different players and NPCs can use the same ability.
- Preserve names before pet ownership is known.
- Normalize only known display variations, currently leading article case and
  local `you` target capitalization.
- Keep every discovered production format as a regression test.
- Recheck a large corpus after every regex expansion to detect false positives.
- Compare against another parser by fight and by ability, not only grand totals.
