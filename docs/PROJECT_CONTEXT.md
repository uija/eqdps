# Project Context

This is the engineering handoff for continuing `eqdps` in a fresh context. Read
this document, then inspect `git status`, the current diff, and the files named
below. The code remains the source of truth if this document becomes stale.

## Identity and Goal

- Project: `eqdps`
- Module: `github.com/uija/eqdps`
- License: MIT
- Language: Go 1.26.4 as declared in the module files
- TUI: `github.com/rivo/tview` over `tcell`
- Entrypoint: `tui/main.go`

The application tails an EverQuest log and shows concurrent per-mob combat
records. It keeps completed mob history, supports historical replay, and has a
plain-text mode for parser comparisons.

## Current Status

The frontend separation plus the initial Linux and Windows GUI passes are
complete on branch `refactor/frontend-separation`. Native Windows overlay
opacity and position restoration, repeated settings saves, Windows defaults,
and embedded icons are implemented. The GUI ran stably during direct Windows
11 testing, and a manually generated executable is available in the GitHub
`v0.1.0` release for wider testing.

Read [`WINDOWS_HANDOFF.md`](WINDOWS_HANDOFF.md) before changing native window
code. Packaging, version metadata, automated releases, and broader volunteer
feedback remain future work.

The primary sample corpus is `eqlog_Wyrmberg_rivervale.txt`. It is large and is
intentionally not duplicated under `docs/`.

## Repository Map

| Path | Responsibility |
| --- | --- |
| `tui/main.go` | CLI, TUI construction, rendering, input handling, and presentation callbacks |
| `tui/main_test.go` | UI layout and history-menu helper tests |
| `tui/go.mod` | Terminal frontend module and tview/tcell dependency graph |
| `gui/main.go` | Gio application shell, navigation, combat view, and status presentation |
| `gui/go.mod` | Graphical frontend module and Gio dependency graph |
| `go.mod` | Shared parser and application-engine module with no UI dependencies |
| `go.work` | Local workspace connecting the shared, TUI, and GUI modules |
| `internal/eqlog/parser.go` | Unified log records plus damage, cast, XP, aggro, and death parsing |
| `internal/engine/log.go` | UI-independent logfile replay, live tailing, and combat/XP record dispatch |
| `internal/eqlog/parser_test.go` | Exact production log format regressions |
| `internal/combat/combat.go` | Stats, pet merging, per-mob lifecycle, history |
| `internal/combat/combat_test.go` | Meter and per-mob state behavior |
| `internal/xp/session.go` | Session XP totals, active time, and XP/hour |
| `internal/xp/session_test.go` | Pause-capping and XP-rate behavior |
| `internal/skyquest/database.go` | Embedded Plane of Sky class-quest database loader |
| `internal/skyquest/plane_of_sky_quests.json` | Generated EQL Wiki quest requirements and rewards |
| `internal/skyquest/tracker.go` | Zone-aware quest holdings and ready-quest calculation |
| `internal/skyquest/persistence.go` | Character state, initial scan, and byte-offset checkpoints |
| `tools/skyquestdb/main.go` | Regenerates the embedded database from EQL Wiki |
| `README.md` | User-facing installation and usage |
| `docs/GUI_ROADMAP.md` | Graphical frontend status and remaining release work |
| `docs/WINDOWS_HANDOFF.md` | Windows 11 implementation and native-behavior checklist |
| `docs/PARSER_RECHECK.md` | Full-corpus parser quality audit procedure |

## Runtime Data Flow

```text
log line
  -> eqlog.ParseRecord (one envelope/timestamp parse)
  -> internal/engine dispatch
  -> combat.FightTracker / xp.Session
  -> combat.Meter / XP snapshot
  -> text, tview, or Gio presentation
```

Live mode opens the file and seeks to EOF, so invocation without replay flags
starts at "now." `engine.Follow` polls EOF every 250 ms and processes appended
lines.

Log replay, live tailing, and combat/XP record dispatch live in
`internal/engine` and must remain free of `tview`, `tcell`, Gio, and other
frontend dependencies. `tui/main.go` and the files under `gui/` own their
respective widgets and presentation callbacks.

Both frontends are nested modules. This keeps tview/tcell out of GUI builds and
keeps Gio and its Linux native requirements out of TUI builds. Their module
paths remain below `github.com/uija/eqdps`, allowing both to consume the shared
`internal` packages.

Plane of Sky quest tracking is independent of combat replay. Once enabled, it
maintains `CHARACTER_SERVER_PoS.json` beside the selected logfile and resumes
from an exact byte offset. A missing state file opens an opt-in prompt in either
interactive frontend for a one-time full-log scan. Choosing `Not Now` or
cancelling creates no state and asks again next launch. Text mode does not
initiate the first scan.

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

The layout is a one-line title/path header, the mob table, a one-line information
bar, and a separate one-line shortcut bar. The dark-gray information bar shows
approximate current-level progress, XP/hour, estimated time to the next level,
and `PoS: N ready` on the far right. When live loot makes one or more quests
ready, its background becomes dark green and its center shows the first newly
ready quest for eight seconds. Startup/catch-up processing never produces this
notification. Columns are Combatant, %, Damage, DPS,
SDPS, Hits, Crits, Min, Max, and Active. DPS and percentages are rounded to
whole numbers. The Combatant column adapts to terminal width, never below 20
characters, and lets tview distribute otherwise-unused width before applying an
ellipsis. Active mobs are expanded by default; completed mobs start collapsed.
There is no player-row limit and the table scrolls normally.

Collapsed mob rows are compact summaries: the fullest mob/status text that fits,
`You` DPS when present, start date/time, and total mob duration. Because tview
tables have no column-span support, the start label, year, month/day, and time
occupy consecutive otherwise-empty statistic cells. Expanded rows use the normal
statistical columns described above.

Hotkeys:

| Key | Action |
| --- | --- |
| `o` | History overlay: Now, 1h, 4h, 8h, 1d, Full |
| `p` | Plane of Sky quest tracker |
| `/` | Filter displayed fights by case-insensitive mob-name substring |
| `Enter` | Expand/collapse a mob, combatant, or detail category |
| `a` | Fully expand or collapse the selected subtree |
| `r` | Clear the combat tracker and session XP meter |
| `q` or `Esc` | Quit |

When the history overlay is open, it owns input. `Enter` selects its button and
`Esc` closes only the overlay. The filter input also owns input; `Enter` applies
the trimmed query, `Esc` cancels, and an empty query clears the filter. Filtering
affects display only and continues to apply to newly parsed fights. After
reload/reset/filtering, `resetTableView` scrolls to the beginning and selects
row 1.

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

## Plane of Sky Quest Holdings

The Plane of Sky class-quest database is generated from the EQL Wiki MediaWiki
overview plus the maintained class-specific `CLASS_Plane_of_Sky_Tests` pages
when available, and embedded into the executable with Go `embed`. Class pages
provide current quest NPC and quest names; the overview supplies compact item
drop annotations. Dialogue keywords are implicit in the `Test of ...` quest
names and are neither stored nor repeated in the UI. Runtime use remains
offline and single-executable. The generated data currently contains 16
classes, 95 quests, 222 requirements, and 128 unique required items. Source page
and revision metadata are retained in the JSON.

Requirements without an overview drop annotation are enriched from their EQL
Wiki item pages during generation. The item-page `dropsfrom` NPC list is stored
without the redundant Plane of Sky zone link, replacing the former generic
`Plane of Sky` display for Efreeti weapons and other components.

The tracker adds only known requirements while the last parsed zone is the base
`The Plane of Sky` or an instance such as `The Plane of Sky 2 (Adaptive)`.
EverQuest Legends upgrade suffixes such as `+1` and `+3` are stripped before
matching, so upgraded quest components count under their database base name.
Normally retained loot and loot stored directly in the
currency tab count as owned. Items immediately sold, including `sold it for
free`, or converted into an upgraded item do not count. Exact `You successfully
destroyed N Item.` messages decrement known holdings in every zone because an
item may be destroyed after leaving Sky.

Quest completion is identified from the exact multiset of `You offered ... to
NPC.` lines followed by `You complete the trade with NPC.` while in Plane of
Sky. Offers remain pending and do not alter holdings until that confirmation.
`You have cancelled the trade.` clears all pending offers, while
`NPC has cancelled the trade.` clears that NPC's pending offer set.
The matching quest is then stored in `completed_quests`, its requirements are
consumed from holdings, it is removed from READY, and the table shows it as
DONE with per-class completion counts. Completed quest and consumed-requirement
rows use muted gray so active collection work remains visually prominent.

On first enable, the scanner processes the logfile from byte zero in a
background goroutine, reports byte and line progress, and keeps the result in
memory until successful completion. Cancellation creates no JSON or partial
checkpoint. Later starts load the holdings and process only complete lines after
the saved byte offset. CRLF offsets are measured from original bytes. A saved
first-line fingerprint and file-size check reject unsafe automatic recovery
after log replacement or truncation. Combat `--back`, `--since`, and history
reloads never mutate Sky holdings.

Existing-state catch-up remains silent for backlogs up to 5 MiB. Larger
backlogs open the TUI at once and reuse the shared byte/line progress overlay.
Live PoS processing waits at the catch-up snapshot boundary so it cannot skip
or overtake missed lines. `Esc` cancels and exits; the partially processed
in-memory tracker is discarded, so the last saved checkpoint remains valid.

Press `p` to open the read-only quest tracker. Its flat, wiki-style table lists
every quest and required item under its class, puts quests with every required
item in a ready-to-turn-in section, and shows owned and required quantities plus
known source hints and rewards. READY entries repeat their complete item/source
checklist and show the quest giver on a dedicated line at the top so narrow
terminals cannot hide it and no class-section lookup is needed. The
READY heading and quest rows are selectable, allowing Page Up to return fully
to the summary after scrolling away. Arrow and page keys browse the table; `p`
or `Esc` returns to the combat view. `h` toggles incomplete quests for which no
required item is owned; READY and DONE quests remain visible and class totals
continue to cover the full achievement.
When no quest is ready, the heading count is the complete empty-state message;
no long first-column sentence is rendered because tview tables have no column
spans and that text would distort the quest column width.

The state is an evidence-based estimate rather than an authoritative EverQuest
inventory snapshot. Logged destruction and confirmed quest turn-ins are
handled, but ordinary player trades, actions while logging is disabled, and
other unobserved removals still require future reconciliation. Turn-ins are
identified from offered-item sets and completion messages because the log does
not name the received reward.

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
audit on 2026-07-16 found 537,314 accepted damage events and no remaining
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
History-menu replays run in a background goroutine against a file-size snapshot.
A modal reports byte-based percentage and scanned-line progress every 5,000
lines. `Esc` cancels without replacing the current tracker; successful replay
replaces it atomically, and replay errors remain visible in the modal.

## EQLDB Connected Application

Both frontends can connect to EQLDB with the public-client device flow and
upload completed EverQuest Legends inventory exports. Shared code lives in:

- `internal/eqlog` for `/who` and `Outputfile Complete` records;
- `internal/inventorysync` for character matching, one-minute metadata
  correlation, and resolving the export above `Logs/`;
- `internal/eqldb` for device authentication, multipart uploads, shared
  configuration, and the cross-process upload lease;
- `internal/platform` for OS-specific browser opening.

Only newly followed lines trigger uploads; history replay never does. Export
bursts are combined for two seconds and uploads have a shared 15-second
cooldown. The TUI uses `e` and the GUI uses **Tools → EQLDB connection** for
connection management. Both ask for level, race, and classes when no recent
matching `/who` result exists.

Authentication state is stored in `eqdps/eqldb.json` below
`os.UserConfigDir()`, independently of the frontend. The access token is written
with owner-only permissions where the operating system supports Unix modes.
The GUI currently includes a clearly marked temporary simulator under its
connected-management dialog. It appends a current `/who` result and
`Outputfile Complete` entry to the followed log so the integration can be
tested while EverQuest Legends is offline. Remove it after the GUI upload path
has been verified.

The connected-application client uses the production endpoint
`https://eqldb.org`. Tests replace the client base URL with an isolated local
HTTP test server.

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
- Combat does not configure the local character's actual name; first-person log
  forms are represented as `You` and `YOU`. Connected inventory uploads derive
  the name from `eqlog_CHARACTER_SERVER.txt`.
- Large replays remain CPU-intensive even though their progress UI stays
  responsive and cancellation is available.
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
go test ./tui/...
go test ./gui/...
go vet ./...
go vet ./tui/...
go vet ./gui/...
go build -o /tmp/eqdps-check ./tui
go build -o /tmp/eqdps-gui-check ./gui
git diff --check
```

7. For parser work, run the full-corpus audit in `docs/PARSER_RECHECK.md`.
8. For UI work, exercise the TUI in a real terminal and the GUI on a real
   desktop. Check focus, narrow sizes, scrolling, collapsed summaries, expanded
   combatant rows, overlay stacking, and minimized-main-window updates.

Do not leave a generated `eqdps` binary in the repository root after checks.

## Current Product Decisions

- Track and display every active mob independently.
- Show all damage sources beneath each mob, without a player limit.
- Keep row ordering stable by first appearance.
- Show active DPS and shared-duration SDPS, suppressing near-identical SDPS.
- Keep completed-mob history unlimited by default, with an optional cap.
- Show session XP/hour using a one-minute cap on combat-inactivity gaps.
- Collapse mob sections and every combatant's nested details with Enter.
- Summarize collapsed mobs with start time, `You` DPS, and duration.
- Attribute player pets to an owner only when the owner is observed in the same
  mob record.
- Favor correct attribution over counting source-less damage.
- Keep operational UI compact for a portrait-oriented 1080p display.

There is no currently documented migration or external persistence layer. Logs
are read directly, fights exist in memory, and no network service is involved.
