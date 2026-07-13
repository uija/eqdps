# Project Context

This is the engineering handoff for continuing `eqdps` in a fresh context. Read
this document, then inspect `git status`, the current diff, and the files named
below. The code remains the source of truth if this document becomes stale.

## Identity and Goal

- Project: `eqdps`
- Module: `github.com/uija/eqdps`
- License: MIT
- Language: Go 1.26.4 as declared in `go.mod`
- TUI: `github.com/rivo/tview` over `tcell`
- Entrypoint: root `main.go`

The application tails an EverQuest log and shows the current fight, or the last
fight when combat is inactive. It also keeps completed fight history, supports
historical replay, and has a plain-text mode for parser comparisons.

The primary sample corpus is `eqlog_Wyrmberg_rivervale.txt`. It is large and is
intentionally not duplicated under `docs/`.

## Repository Map

| Path | Responsibility |
| --- | --- |
| `main.go` | CLI, replay, live file tailing, TUI, rendering, input handling |
| `main_test.go` | UI layout and history-menu helper tests |
| `internal/eqlog/parser.go` | Log envelope, damage, spell, shield, and death parsing |
| `internal/eqlog/parser_test.go` | Exact production log format regressions |
| `internal/combat/combat.go` | Stats, pet merging, fight lifecycle, history |
| `internal/combat/combat_test.go` | Meter and fight-state behavior |
| `README.md` | User-facing installation and usage |
| `docs/PARSER_RECHECK.md` | Full-corpus parser quality audit procedure |

## Runtime Data Flow

```text
log line
  -> eqlog.ParseLine / ParseDeathLine
  -> main.processLine
  -> combat.FightTracker
  -> combat.Meter
  -> text renderer or tview table
```

Live mode opens the file and seeks to EOF, so invocation without replay flags
starts at "now." `followLog` polls EOF every 250 ms and processes appended lines.

Replay mode scans the file from a cutoff, uses log timestamps for idle endings,
then live tailing still opens at the current EOF. `--since` takes precedence over
`--back` when both are present.

## CLI Behavior

```text
eqdps [flags] <everquest-log-file>
```

| Flag | Meaning |
| --- | --- |
| `--text` | Print sections once instead of opening the TUI |
| `--idle-timeout=15s` | Combat inactivity required to end a fight |
| `--back=N` | Replay the last N log minutes before live mode |
| `--since="YYYY-MM-DD HH:MM"` | Replay from an exact log timestamp |
| `--history=N` | Keep N completed fights; zero means unlimited |

`--since` also accepts `YYYY-MM-DDTHH:MM`. Timestamps are parsed without a
location and compared to timestamps parsed from the log in the same way.

## TUI Behavior and Constraints

The layout is a one-line title/path header, the fight table, and a one-line
status bar. Columns are Combatant, Damage, DPS, Hits, Crits, Active, and Last
Target. Combatant and target widths adapt to terminal width and use `...` when
truncated.

Hotkeys:

| Key | Action |
| --- | --- |
| `o` | History overlay: Now, 1h, 4h, 8h, 1d |
| `Enter` | Toggle details on a `You` row |
| `r` | Clear the in-memory tracker |
| `q` or `Esc` | Quit |

When the history overlay is open, it owns input. `Enter` selects its button and
`Esc` closes only the overlay. After reload/reset, `resetTableView` scrolls to
the beginning and selects row 1.

Only the `You` row is expandable. Details show damage grouped by ability. Each
detail row places ability damage under Damage, ability DPS under DPS, and its
share of total damage under Last Target. Events without an ability are grouped
as `Melee`; merged pet damage is grouped as `Pet: <pet name>`.

Important tview concurrency rule: background goroutines use
`app.QueueUpdateDraw(render)`. UI event handlers call `render()` directly;
queueing an update from inside an event handler can freeze the application.
Access to the replaceable `tracker` and associated row maps is guarded by `mu`.

## Damage and DPS Model

Each accepted event increments source damage and hit count. Critical events also
increment crit count. Active duration is:

```text
last event timestamp - first event timestamp + 1 second
```

DPS is source damage divided by that active duration. Combatants remain ordered
by first-seen timestamp, then name, so rows do not reorder on every update.

The parser uses these local-player conventions:

- `You`: local player as a damage source.
- `YOU`: local player as a target.
- Other case variants of target `you` normalize to `YOU`.

Leading `A`, `An`, and `The` are lowercased during name normalization so a mob
at the beginning of a sentence does not become a second entity.

## Pet Model

Possessive names remain independent raw damage sources. During
`Meter.Players()`, a source such as `Sobatin's warder` or ``Sobatin`s warder`` is
merged into `Sobatin` only when `Sobatin` also appears as a source in that fight.

If the apparent owner is absent, the possessive entity remains separate. This
prevents mobs such as ``Innoruuk`s Chosen`` from being truncated to `Innoruuk`.
Merged pet damage retains a `Pet: warder` breakdown entry.

This is heuristic ownership, not an EverQuest pet registry. Changes here must
cover both apostrophe forms and possessive mob names.

## Parser State

Supported damage families include direct melee, direct spell/proc damage,
spells and DoTs with explicit sources, local `from your` DoTs, and local/remote
damage shields. Direct melee verbs are explicitly listed in `damageRE`.

Critical detection checks whether the trailing marker contains `Critical`, so
both `(Critical)` and `(Riposte Critical)` count. `(Finishing Blow)` alone does
not count as a critical.

Supported death messages:

```text
You have slain Target!
Victim has been slain by Killer!
```

Known intentional exclusions:

```text
Target has taken N damage by Ability.
You were hit by non-melee for N damage.
```

These messages do not identify a source and must not be guessed into a player
or mob total. `Name dies.` is also not a death signal: the sample log shows it
for Feign Death.

See `docs/PARSER_RECHECK.md` before changing parser expressions. The reference
audit on 2026-07-13 found 395,576 accepted damage events and no remaining
source-attributable damage-like rejection in the merged sample corpus.

## Fight State Machine

Important constants:

- Default idle timeout: 15 seconds
- Death grace period: 8 seconds
- Default history limit: 0, meaning unlimited

A damage event creates the current fight. A death becomes a pending fight end
only when the victim is the mob most recently attacked by `You`, when every
hostile mob observed fighting `You` is dead, or when `You` dies. Damage-shield
and damage-over-time events are passive: they identify involved hostiles but do
not replace the active target. This prevents a pet that only hits `You` or
triggers a damage shield from splitting the owner's fight. EverQuest can emit
late damage near a death message, so a qualifying mob death is not finalized
immediately.

While death is pending:

- Damage involving the slain mob stays in that fight within the grace period.
- Damage involving neither the slain source nor target finalizes the old fight
  and starts a new one.
- Exceeding the grace period finalizes the old fight.
- Local-player death finalizes immediately.

Deaths of other involved mobs are recorded but do not end the fight while the
active target or another observed hostile remains alive.

Without a death message, an idle gap finalizes the fight with reason
`idle timeout`. Live detection measures time since the line was observed;
historical replay measures gaps between log timestamps.

`DisplaySections` shows the current/pending fight first, followed by newest
completed fights. When there is no current fight, history index zero is shown as
the last fight.

## History and Reloading

History is stored only in memory. A positive `--history` value trims completed
fights; zero keeps all completed fights parsed during that process.

The `o` overlay replaces the tracker with a replayed tracker. Choosing `Now`
creates an empty tracker and continues from newly appended lines. The live tail
goroutine is not reopened at a historical offset; replay reads history once,
while the existing tail continues to follow EOF.

## Known Limitations

- `followLog` opens the file once and does not detect log rotation, replacement,
  or truncation.
- History and fight data are in memory only; restarting reconstructs them only
  when replay flags are used.
- Unlimited history can consume increasing memory during very large replays.
- Source-less damage remains excluded because attribution is unknowable.
- The local character's actual name is not configured; first-person log forms
  are represented as `You` and `YOU`.
- History replay runs synchronously from the overlay callback and may block the
  UI briefly on a very large file.
- A history-overlay replay error is currently ignored and leaves a new empty
  tracker; startup and text-mode replay errors are returned to the user.
- Timestamps carry no timezone information. Both CLI cutoffs and log timestamps
  use Go's location-less parsing, which keeps their comparisons consistent.

## Safe Change Procedure

1. Read `git status --short` and preserve changes not made for the task.
2. Read the surrounding implementation and existing tests before editing.
3. Put exact log examples into parser tests for every new format.
4. Add combat tests for fight lifecycle, history, ordering, or pet behavior.
5. Use `apply_patch` for manual edits and `gofmt` afterward.
6. Run:

```bash
go test ./...
go vet ./...
go build -o /tmp/eqdps-check .
git diff --check
```

7. For parser work, run the full-corpus audit in `docs/PARSER_RECHECK.md`.
8. For UI work, exercise the TUI in a real terminal, especially overlay focus,
   narrow widths, scrolling, and expanded `You` rows.

Do not leave a generated `eqdps` binary in the repository root after checks.

## Current Product Decisions

- Show the current fight; otherwise show the last fight.
- Show mobs as rows as well as players.
- Keep row ordering stable by first appearance.
- Keep history unlimited by default, with an optional cap.
- Hide local damage details until Enter is pressed on `You`.
- Attribute pets to an owner only when the owner is observed in the same fight.
- Favor correct attribution over counting source-less damage.
- Keep operational UI compact for a portrait-oriented 1080p display.

There is no currently documented migration or external persistence layer. Logs
are read directly, fights exist in memory, and no network service is involved.
