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

The application tails an EverQuest log and shows concurrent per-mob combat
records. It keeps completed mob history, supports historical replay, and has a
plain-text mode for parser comparisons.

The primary sample corpus is `eqlog_Wyrmberg_rivervale.txt`. It is large and is
intentionally not duplicated under `docs/`.

## Repository Map

| Path | Responsibility |
| --- | --- |
| `main.go` | CLI, replay, live file tailing, TUI, rendering, input handling |
| `main_test.go` | UI layout and history-menu helper tests |
| `internal/eqlog/parser.go` | Log envelope, damage, spell, shield, and death parsing |
| `internal/eqlog/parser_test.go` | Exact production log format regressions |
| `internal/combat/combat.go` | Stats, pet merging, per-mob lifecycle, history |
| `internal/combat/combat_test.go` | Meter and per-mob state behavior |
| `internal/xp/session.go` | Session XP totals, active time, and XP/hour |
| `internal/xp/session_test.go` | Pause-capping and XP-rate behavior |
| `README.md` | User-facing installation and usage |
| `docs/PARSER_RECHECK.md` | Full-corpus parser quality audit procedure |

## Runtime Data Flow

```text
log line
  -> eqlog damage/death/XP/level-up/aggro-clear parsers
  -> main.processLine
  -> combat.FightTracker / xp.Session
  -> combat.Meter / XP snapshot
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
| `--idle-timeout=15s` | Per-mob inactivity required to close a record |
| `--back=N` | Replay the last N log minutes before live mode |
| `--since="YYYY-MM-DD HH:MM"` | Replay from an exact log timestamp |
| `--history=N` | Keep N completed mobs; zero means unlimited |

`--since` also accepts `YYYY-MM-DDTHH:MM`. Timestamps are parsed without a
location and compared to timestamps parsed from the log in the same way.

## TUI Behavior and Constraints

The layout is a one-line title/path header, the mob table, and a one-line status
bar. The status bar shows approximate current-level progress, XP/hour, estimated
time to the next level, and hotkeys. Columns are Combatant, %, Damage, DPS,
SDPS, Hits, Crits, Min, Max, and Active. DPS and percentages are rounded to
whole numbers. The Combatant column adapts to terminal width, never below 20
characters, and uses `...` when truncated. Active mobs are expanded by default;
completed mobs start collapsed.
There is no player-row limit and the table scrolls normally.

Hotkeys:

| Key | Action |
| --- | --- |
| `o` | History overlay: Now, 1h, 4h, 8h, 1d |
| `Enter` | Expand/collapse a mob, combatant, or detail category |
| `r` | Clear the combat tracker and session XP meter |
| `q` or `Esc` | Quit |

When the history overlay is open, it owns input. `Enter` selects its button and
`Esc` closes only the overlay. After reload/reset, `resetTableView` scrolls to
the beginning and selects row 1.

Mob headers and every combatant row are expandable. Combatant details are nested
under Melee, DoTs, Magic, Procs, Damage Shield, and merged `Pet: <pet name>`
categories. Melee is split by attack verb. Direct ability damage correlated to
an exact-source and exact-ability `begins casting` message within 30 seconds is
Magic; unmatched direct ability damage is a Proc. DoTs and shields retain their
individual ability names. Category and ability rows show percentage of
combatant damage, damage, active DPS, shared SDPS, hits, crits, min/max hit, and
active duration.

Important tview concurrency rule: background goroutines use
`app.QueueUpdateDraw(render)`. UI event handlers call `render()` directly;
queueing an update from inside an event handler can freeze the application.
Access to the replaceable `tracker` and associated row maps is guarded by `mu`.

## Session XP Model

`eqlog.ParseExperienceLine` accepts exact local messages of the form `You gain
experience! (N.NNN%)`. `eqlog.ParseLevelUpLine` recognizes the local level-up
message. `xp.Session` keeps both a full-session total for XP/hour and a progress
total that resets at each observed level-up.

The XP message paired with a level-up is the award that caused the ding, so it
remains in the full-session rate total but is excluded from new-level progress.
After an observed level-up, progress is treated as known and shown without `~`.
When replay or live mode starts partway through a level, the log does not reveal
starting progress, so the displayed gain since startup is prefixed with `~`.
The ETA is always approximate, uses the current session XP/hour, and is rounded
up to the next minute for display as hours and minutes.

Combat damage timestamps drive the active clock. Intervals up to one minute
count in full; longer intervals contribute exactly one minute. The live status
continues growing for at most one minute after the latest combat event, then
stops until combat resumes. This includes ordinary pull time while excluding
most travel and longer breaks. Replay and text mode use log time rather than
wall-clock time.

The XP session starts with the first observed combat or XP gain. Startup replay,
history reloads, choosing `Now`, and `r` each create the corresponding fresh or
replayed XP session alongside the combat tracker.

## Damage and DPS Model

Each accepted event is assigned to one mob record and increments its source's
damage and hit count. Critical events also increment crit count. Mob duration is:

```text
last event timestamp - first event timestamp + 1 second
```

DPS is damage divided by the combatant or detail row's active duration. For
`You`, the combatant DPS uses the deliberate engagement duration when known.
SDPS is damage divided by the shared mob duration and is hidden when it differs
from DPS by less than ten percent.
Combatants remain ordered by first-seen timestamp, then name. There is no cap on
the number of players in a mob record.

The parser uses these local-player conventions:

- `You`: local player as a damage source.
- `YOU`: local player as a target.
- Other case variants of target `you` normalize to `YOU`.

Leading `A`, `An`, and `The` are lowercased during name normalization so a mob
at the beginning of a sentence does not become a second entity.

## Pet Model

Possessive names remain independent raw damage sources. During
`Meter.Players()`, a source such as `Sobatin's warder` or ``Sobatin`s warder`` is
merged into `Sobatin` only when `Sobatin` also appears as a source in that mob
record.

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

## Per-Mob State Machine

Important constants:

- Default idle timeout: 15 seconds
- Death grace period: 8 seconds
- Default history limit: 0, meaning unlimited

A damage event is assigned to a mob using its source and target plus learned
player/mob roles. `You` attacking identifies the target as a mob; an entity
attacking `YOU` identifies the source as a mob. Known roles then route group
events where the local player is not one endpoint. Unknown events default to the
damage target, matching the common player-to-mob form.

Every mob has its own meter, pending death, wall-clock activity, and log-time
activity. A mob death affects only that record. Damage from other mob names
cannot split or close it. Same-timestamp damage remains with a dead mob. Later
same-name DoTs are buffered during the eight-second grace period. A later
non-DoT confirms a new same-name mob, finalizes the old record, and moves the
buffered DoTs into the successor. Without confirmation, the buffered events
return to the old record when grace expires. A second same-name death also
confirms a buffered successor. Local-player death closes every active record
immediately. Without a death message, each mob closes independently after its
idle timeout.

The exact `Your enemies have forgotten you!` message closes every active record
with reason `enemies forgot you`. Each closed record remains in a per-name
forgotten registry for eight seconds. Attributable DoTs update that completed
fight and renew its log-time and wall-time retention window. A non-DoT event
deletes the forgotten association and creates a new fight. This prevents a DoT
left on the local player after Feign Death from reopening combat.

Names ending in `<owner> pet` map into the owner's mob record. A pet death is
ignored as a boundary once the owner itself has been observed. Possessive pet
names map to an already-active owner; unrelated possessive mob names remain
independent.

`DisplaySections` shows active and pending mob records ordered by first sight,
then completed records newest-first. A positive history limit trims only
completed records; active mobs and players are never capped.

## History and Reloading

History is stored only in memory. A positive `--history` value trims completed
mob records; zero keeps all completed mobs parsed during that process.

The `o` overlay replaces the combat and XP trackers with replayed trackers.
Choosing `Now` creates empty trackers and continues from newly appended lines.
The live tail goroutine is not reopened at a historical offset; replay reads
history once, while the existing tail continues to follow EOF.

## Known Limitations

- `followLog` opens the file once and does not detect log rotation, replacement,
  or truncation.
- History and fight data are in memory only; restarting reconstructs them only
  when replay flags are used.
- Session XP is based on percentage messages rather than raw character XP;
  EverQuest does not include the raw XP total in these log lines.
- Unlimited history can consume increasing memory during very large replays.
- Source-less damage remains excluded because attribution is unknowable.
- Simultaneous living mobs with exactly the same log name share one active
  record because EverQuest provides no spawn identifier. Death buffering can
  separate the successor only once a death supplies a boundary.
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
4. Add combat tests for mob lifecycle, history, ordering, or pet behavior.
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

- Track and display every active mob independently.
- Show all damage sources beneath each mob, without a player limit.
- Keep row ordering stable by first appearance.
- Show active DPS and shared-duration SDPS, suppressing near-identical SDPS.
- Keep completed-mob history unlimited by default, with an optional cap.
- Show session XP/hour using a one-minute cap on combat-inactivity gaps.
- Collapse mob sections and every combatant's nested details with Enter.
- Attribute player pets to an owner only when the owner is observed in the same
  mob record.
- Favor correct attribution over counting source-less damage.
- Keep operational UI compact for a portrait-oriented 1080p display.

There is no currently documented migration or external persistence layer. Logs
are read directly, fights exist in memory, and no network service is involved.
