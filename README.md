# eqdps

`eqdps` is a terminal DPS meter built primarily for **EverQuest Legends** log
files. Its parser and feature set follow EverQuest Legends combat, experience,
zone, loot, and quest messages. Other EverQuest variants may work where their
log formats overlap, but they are not the project's main compatibility target.

It tails an EQ log, tracks every engaged mob independently, and shows per-mob
damage in a live terminal UI. It can also replay recent log history to compare
parses or debug combat detection.

`Because I am lazy and wanted to play EverQuest Legends Open Beta, Codex did most of the work.`

DPS Screen
![eqdps terminal interface](img/Screenshot-2026-07-16_17-11-16.png)
Plane of Sky (EverQuest Legends) tracker
![eqdps terminal interface](img/Screenshot-2026-07-16_17-11-41.png)

## Features

- Live EverQuest log tailing
- Concurrent active-mob and completed-mob history display
- Independent mob endings from death, player death, and idle timeout
- Player, pet, mob, spell, proc, DoT, and damage shield parsing
- Per-mob combatant rows with DPS/SDPS, hits, crits, min/max, and active time
- Session XP percentage and XP/hour with long pauses excluded
- Expandable details grouped by melee, cast magic, proc, DoT, and shield
- Adaptive table widths for narrow terminals
- In-app history reload menu
- EverQuest Legends Plane of Sky class-quest inventory and completion tracker
- Plain text output mode for comparisons

## Install

Build the terminal application from a local checkout:

```bash
git clone https://github.com/uija/eqdps.git
cd eqdps
go build -o eqdps ./tui
```

## Usage

Run the live TUI:

```bash
eqdps /path/to/eqlog_character_server.txt
```

From a local checkout:

```bash
go run ./tui /path/to/eqlog_character_server.txt
```

### Graphical frontend

The in-development Gio frontend is isolated from the terminal module and can
be run from a checkout with:

```bash
go run ./gui
```

Open the compact current-fight window through **View → Show DPS overlay**. Its
visible/hidden state is remembered between launches.

Combat history replays and large Plane of Sky catch-ups show cancellable
progress. **Combat → Reset session** clears the current combat and XP session
and resumes at the end of the selected logfile. A newly completed Plane of Sky
item set is announced in the clickable status segment without blocking combat.

#### DPS overlay on Wayland

Wayland compositors control floating, stacking, opacity, and placement; an
application cannot request those behaviors portably. eqdps detects Wayland and
shows this explanation once when the DPS overlay is first opened. It remains
available under **Help → Wayland overlay setup**.

For Hyprland 0.55 and newer, add this rule to
`~/.config/hypr/hyprland.lua`:

```lua
hl.window_rule({
    name = "eqdps-overlay",
    match = { title = "^eqdps — Current Fight$" },
    float = true,
    pin = true,
    no_initial_focus = true,
    persistent_size = true,
    move = {100, 100},
    opacity = "0.75 override 0.75 override 0.75 override",
})
```

Change `move` and `opacity` to taste. KDE Plasma users can create an equivalent
Window Rule matching the title `eqdps — Current Fight`. Sway users can match
the same title with `for_window` and enable floating/sticky behavior. GNOME
Wayland may require an extension for persistent always-on-top behavior.

By default, combat live mode starts at the current end of the log file and only
parses new combat lines written after startup. Once Plane of Sky tracking is
enabled, its character state resumes from its saved logfile offset and catches
up missed loot and turn-ins before following live lines. A backlog larger than
5 MiB opens the TUI immediately and uses the shared progress overlay; `Esc`
cancels that catch-up and exits with a valid saved checkpoint.

## Hotkeys

| Key | Action |
| --- | --- |
| `o` | Open history menu, including a full-log replay |
| `p` | Open the Plane of Sky quest tracker |
| `/` | Filter the displayed fights by mob name |
| `Enter` | Expand/collapse a mob, combatant, or detail category |
| `a` | Fully expand or collapse the selected subtree |
| `r` | Reset the combat and session XP meters and start fresh |
| `q` / `Esc` | Quit |

## Plane of Sky Quest Tracker (EverQuest Legends)

The Plane of Sky tracker supports the EverQuest Legends class-unlock system.
It watches Plane of Sky loot, records required class-quest components, shows
what is owned and still missing, and highlights quests that are ready to hand
in. Completed turn-ins are detected from their item, rune, quest-giver, and
reward log messages and are marked done in the checklist.

Press `p` to open the tracker. Press `h` there to hide quests for which no
required component has been collected. The main DPS screen shows `PoS: N ready`
and briefly highlights the status bar when newly looted items make another
quest ready.

The quest database is embedded in the executable, so no separate database file
is required. Character progress remains separate in
`CHARACTER_SERVER_PoS.json` beside the logfile. On first use, the app asks
before scanning existing history. Later launches resume from the saved byte
offset and catch up missed loot and turn-ins. Loot is counted only while the
character is in Plane of Sky, including numbered/adaptive instances.

## History And Replay

The in-app history menu offers Now, 1h, 4h, 8h, 1d, and Full. After loading
history, press `/` and enter a case-insensitive mob-name substring to compare
matching fights. Submit an empty filter to show every fight again.

Seed the TUI with recent history before continuing live:

```bash
eqdps --back=30 /path/to/log.txt
```

Parse from an exact log timestamp:

```bash
eqdps --since "2026-07-06 19:22" /path/to/log.txt
```

Show all completed mobs instead of limiting history:

```bash
eqdps --history=0 --since "2026-07-06 19:22" /path/to/log.txt
```

Print text output instead of opening the TUI:

```bash
eqdps --text --back=30 /path/to/log.txt
```

## Session XP Rate

The information bar shows progress in the current level, average XP/hour,
estimated time until the next level, and the number of ready Plane of Sky
turn-ins. Shortcuts occupy a separate line below it. Progress resets when a
level-up is observed,
and the paired XP award from the dinging kill is not counted in the new level.
When the app starts partway through a level, progress is prefixed with `~`
because the log does not reveal the character's starting XP bar. The ETA always
uses `~` because it is a projection. XP comes from the log's `You gain
experience! (N.NNN%)` messages.

XP/hour continues across level-ups and covers the full period since startup,
replay cutoff, history reload, or the last reset.

Ordinary combat and pull time counts toward the average. When combat activity
stops for more than one minute, only the first minute of that idle period counts.
This keeps travel and longer breaks from depressing the session rate while still
including normal time between fights. The same summary appears in text mode.

## Flags

| Flag | Default | Description |
| --- | ---: | --- |
| `--back=N` | `0` | Parse the last `N` minutes before live tailing |
| `--since "YYYY-MM-DD HH:MM"` | empty | Parse from an absolute log timestamp |
| `--history=N` | `0` | Completed mobs to keep/show; `0` keeps all |
| `--idle-timeout=15s` | `15s` | End each mob record after no activity for this duration |
| `--text` | `false` | Print text output instead of opening the TUI |

## Per-Mob Combat Tracking

Each hostile mob has an independent record. Outgoing damage is assigned to its
target; incoming damage is assigned to its hostile source. Learned player and
mob roles handle group combat where the local player is not involved in every
event.

Several mobs can remain active simultaneously. AoE, riposte, damage-shield, and
DoT events update the mob they actually affect without changing another mob's
lifecycle. A mob's death closes only its own record. Local-player death closes
all active mobs, and inactivity closes each idle mob independently.

`Your enemies have forgotten you!` closes every visible fight immediately.
Those completed records remain available for attributable lingering DoTs. Each
DoT tick renews an eight-second retention window without reopening combat; a
later non-DoT event involving that mob starts a new fight immediately.

Recognizable `<owner> pet` damage is included in the owner's mob record, while a
pet death does not close a living owner's record. Damage at the same timestamp
as a mob's death remains with that mob. Later same-name DoTs are buffered for up
to eight seconds: a later non-DoT confirms a new spawn and receives the buffered
DoTs; otherwise they return to the completed mob when the grace period expires.

Every player who damages a mob appears in that mob's section; there is no player
limit. DPS uses the combatant or ability's active interval (and the deliberate
engagement interval for `You`). SDPS uses the shared mob duration and is hidden
when it is within ten percent of DPS.

## Development

Project documentation:

- [Parser recheck guide](docs/PARSER_RECHECK.md)
- [Project context and engineering handoff](docs/PROJECT_CONTEXT.md)

Run tests:

```bash
go test ./...
go test ./tui/...
```

Build the terminal application:

```bash
go build -o eqdps ./tui
```

## License

MIT
