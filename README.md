# eqdps

`eqdps` is a terminal DPS meter for EverQuest log files.

It tails an EQ log, detects fights, and shows damage per combatant in a live
terminal UI. It can also replay recent log history to compare parses or debug
fight detection.

`Because I am lazy and wanted to play EverQuest Legends Open Beta, Codex did most of the work.`

## Features

- Live EverQuest log tailing
- Current fight and fight history display
- Automatic fight endings from mob deaths, player death, and idle timeout
- Player, pet, mob, spell, proc, DoT, and damage shield parsing
- Per-fight combatant rows with damage, DPS, hits, crits, duration, and target
- Expandable `You` row with damage breakdown by melee/spell/proc type
- Adaptive table widths for narrow terminals
- In-app history reload menu
- Plain text output mode for comparisons

## Install

```bash
go install github.com/uija/eqdps@latest
```

Or build from a local checkout:

```bash
git clone https://github.com/uija/eqdps.git
cd eqdps
go build .
```

## Usage

Run the live TUI:

```bash
eqdps /path/to/eqlog_character_server.txt
```

From a local checkout:

```bash
go run . /path/to/eqlog_character_server.txt
```

By default, live mode starts at the current end of the log file and only parses
new lines written after startup.

## Hotkeys

| Key | Action |
| --- | --- |
| `o` | Open history menu |
| `Enter` | Expand/collapse damage breakdown on the `You` row |
| `r` | Reset the in-memory meter and start fresh |
| `q` / `Esc` | Quit |

## History And Replay

Seed the TUI with recent history before continuing live:

```bash
eqdps --back=30 /path/to/log.txt
```

Parse from an exact log timestamp:

```bash
eqdps --since "2026-07-06 19:22" /path/to/log.txt
```

Show all completed fights instead of limiting history:

```bash
eqdps --history=0 --since "2026-07-06 19:22" /path/to/log.txt
```

Print text output instead of opening the TUI:

```bash
eqdps --text --back=30 /path/to/log.txt
```

## Flags

| Flag | Default | Description |
| --- | ---: | --- |
| `--back=N` | `0` | Parse the last `N` minutes before live tailing |
| `--since "YYYY-MM-DD HH:MM"` | empty | Parse from an absolute log timestamp |
| `--history=N` | `0` | Completed fights to keep/show; `0` keeps all |
| `--idle-timeout=15s` | `15s` | End current fight after no combat for this duration |
| `--text` | `false` | Print text output instead of opening the TUI |

## Fight Detection

A fight starts when a damage event is parsed.

A fight ends when one of these happens:

- a slain/death message is found
- `You have been slain by ...` is found
- no combat is seen for the idle timeout

If the log prints late damage from the same slain mob immediately after the
death line, `eqdps` keeps that damage in the completed fight. Damage involving a
different mob starts a new fight.

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
go build .
```

## License

MIT
