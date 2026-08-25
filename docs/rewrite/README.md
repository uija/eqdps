# eqdps

`eqdps` is a graphical EverQuest log parser and damage meter built primarily
for [EverQuest Legends](https://www.everquest.com/). It follows the active log
file, keeps concurrent fights separate, and presents combat, experience, event,
and Plane of Sky information without modifying or interacting with the game
client.

Other EverQuest versions may work where their log messages use the same format,
but EverQuest Legends is the project's main compatibility target.

## Features

- Live logfile following and recent-history replay.
- Concurrent-fight damage tracking with melee, spell, proc, DoT, and
  damage-shield breakdowns.
- DPS and SDPS, hit and critical-hit counts, active time, fight filtering, and
  expandable combatant details.
- A compact separate DPS overlay with current-fight and event-timer displays.
- Session XP/hour, current-level progress, and estimated time to the next
  level.
- Configurable spell-fade, spell-timer, text, and regular-expression events.
- Desktop notifications, embedded notification sounds, shared volume control,
  and selectable spell-icon sets extracted from the EverQuest installation.
- Plane of Sky quest, item, rune, watched-quest, and turn-in tracking.
- Optional inventory and Plane of Sky uploads to
  [eqldb.org](https://eqldb.org/).
- Recent logfile selection, font scaling, update checks, and persistent
  application settings.
- Native Windows support for DPS-overlay position, opacity, and
  always-on-top behavior.

## Screenshots

### DPS meter

![eqdps DPS meter](img/eqdps_mainwindow_dps.png)

### Plane of Sky quest tracker

![eqdps Plane of Sky quest tracker](img/eqdps_mainwindow_pos.png)

### Plane of Sky inventory

![eqdps Plane of Sky inventory](img/eqdps_mainwindow_pos_inventory.png)

### Events

![eqdps Events configuration](img/eqdps_mainwindow_events.png)

### DPS overlay

![eqdps DPS overlay](img/eqdps_overlay.png)

## Download

Published builds are available from the
[GitHub releases page](https://github.com/uija/eqdps/releases).

Windows builds are currently not code-signed. Windows may therefore show an
**Unknown publisher** warning. Only download binaries from the official
repository.

Linux users can build and install eqdps using the included Makefile.

### Legacy terminal interface

Users who prefer the terminal interface can continue to use the version kept
on the [`legacy` branch](https://github.com/uija/eqdps/tree/legacy). That branch
preserves the previous TUI and its associated tools, but it is archived and
will not receive maintenance, bug fixes, compatibility updates, or new
features.

## Getting Started

Enable logging in EverQuest and select the resulting logfile in eqdps through
**File → Open logfile**. A normal filename looks like:

```text
eqlog_CHARACTER_SERVER.txt
```

EverQuest usually stores these files in its `Logs` directory. eqdps remembers
the selected logfile and offers recently opened logs through the File menu.

Use the left sidebar to switch between the DPS meter, Plane of Sky tracker,
Events, and EQLDB integration. Replay options and application preferences are
available from the menu bar.

## Building from Source

The module currently requires the Go version declared in `go.mod` and Make.

Clone the repository and build the graphical application:

```bash
git clone https://github.com/uija/eqdps.git
cd eqdps
make
```

The binary is written to:

```text
dist/eqdps-gui
```

Run it directly from the source tree without creating a distribution build:

```bash
go run .
```

Install the binary, desktop entry, and application icon system-wide:

```bash
sudo make install
```

Install for the current user instead:

```bash
make install PREFIX="$HOME/.local"
```

Other useful targets are:

```bash
make test       # tests, vet, and race-enabled tests
make windows    # Windows amd64 GUI build
make clean
make uninstall
```

### Linux build dependencies

Gio requires the native development packages for the Linux display and input
stack. Package names differ between distributions.

Fedora:

```bash
sudo dnf install \
    golang make gcc pkgconf-pkg-config \
    libxkbcommon-devel wayland-devel vulkan-loader-devel \
    libX11-devel libglvnd-devel libxkbcommon-x11-devel \
    libXcursor-devel libXfixes-devel
```

Ubuntu, Debian, and Mint:

```bash
sudo apt install \
    golang-go make gcc pkg-config build-essential \
    libxkbcommon-dev libwayland-dev libvulkan-dev \
    libx11-dev libx11-xcb-dev libglvnd-dev \
    libxkbcommon-x11-dev libxcursor-dev libxfixes-dev
```

## DPS Overlay on Linux

The overlay uses this stable window title:

```text
eqdps — Current Fight
```

On Windows, eqdps can request and restore the overlay's native position,
opacity, and topmost state. On Wayland, those decisions belong to the desktop
compositor, so a compositor-specific window rule may be required.

### KDE Plasma

KDE Plasma users can create a Window Rule matching the exact title
`eqdps — Current Fight`. Add the **Layer** property and set it to
**Force → Overlay**. Depending on the Plasma version, the rule can be opened
through **More Actions → Configure Special Window Settings**.

![KDE Plasma overlay window actions](img/kde_overlay_fix_1.png)

![KDE Plasma Window Rule for eqdps](img/kde_overlay_fix_2.png)

### GNOME

Focus the overlay, press `Alt+Space`, and choose **Always on Top**. GNOME may
require this again after restarting the application.

### Sway, Hyprland, and other compositors

Create a floating, pinned or topmost window rule matching the title above.
Rule syntax and support for forced opacity and position depend on the
compositor version.

These Linux desktop notes are community-tested guidance rather than behavior
that Gio can enforce portably.

## Configuration and Character Data

General settings are stored in the operating system's user-configuration
directory:

| Platform | Default location |
| --- | --- |
| Linux | `$XDG_CONFIG_HOME/eqdps` or `~/.config/eqdps` |
| Windows | `%AppData%\eqdps` |
| macOS | `~/Library/Application Support/eqdps` |

`config.json` contains the recent logfile list, Events configuration, sound
volume, selected spell-icon set, EQLDB connection, and UI preferences.
Extracted spell icons are stored below `spell-icons/` in the same directory.

Plane of Sky progress is character-specific and is stored beside the selected
logfile as:

```text
eqdps_CHARACTER_SERVER_PoS.json
```

## Why a Rewrite?

The original eqdps proved that the parser and its companion tools were useful,
but rapid development left behavior spread across multiple frontends and made
changes increasingly difficult to reason about.

This rewrite is intentionally human-written around a smaller, consistent Gio
application. It gives parsing, application modules, persistent data, reusable
UI controls, and the separate overlay clearer ownership. The goal is easier
maintenance, more predictable concurrency, and one coherent interface in
which new features do not need to be implemented several times.

The legacy implementation remains valuable as a behavioral reference while
the rewrite is developed.

## LLM Use

This project is human-directed and human-reviewed. OpenAI Codex was used as a
development assistant for bounded tasks, including:

- explaining Gio APIs and acting as searchable Gio documentation;
- investigating bugs and comparing behavior with the legacy implementation;
- mechanical work such as searches and repetitive replacements;
- reusable form controls and other self-contained UI work; and
- HTTP/Web API client implementations.

Product requirements, architecture, behavior, and final integration decisions
remain the responsibility of the maintainer. Suggested code was reviewed,
tested, and changed where necessary rather than accepted as an authority.

## Community and Thanks

A big thank you to the **Side Gigg** guild on Rivervale and especially to
**Karthar** for test data, repeated testing and feedback, and the KDE Plasma
overlay guidance and screenshots; and to **Gigglemage** for Windows testing,
feature ideas, and shoutouts on stream.

Thanks to **Brent Aureli** for the original community contribution that added
clickable Wiki links to Plane of Sky rewards.

eqdps is built on excellent open-source projects, particularly:

- [Gio](https://gioui.org/) for the immediate-mode graphical interface;
- [Oto](https://github.com/ebitengine/oto) and
  [go-mp3](https://github.com/hajimehoshi/go-mp3) for audio playback and
  decoding;
- [beeep](https://github.com/gen2brain/beeep) for desktop notifications;
- [Zenity](https://github.com/ncruces/zenity) for native desktop dialogs; and
- the Go standard library and the wider Go open-source ecosystem.

Thanks also to the [EQL Wiki contributors](https://eqlwiki.com/) for
documenting EverQuest Legends.

## Plane of Sky Data Attribution

The embedded Plane of Sky quest dataset is derived from the
[EQL Wiki Plane of Sky class-quest data](https://eqlwiki.com/Plane_of_Sky#Plane_of_Sky_Class_Quests)
and related class and item pages. It includes quest names, quest givers,
requirements, rewards, and item sources, normalized into the application's
structured format and supplemented with corrections observed in logs.

EQL Wiki content is available under the
[Creative Commons Attribution-ShareAlike 4.0 International license](https://creativecommons.org/licenses/by-sa/4.0/)
unless otherwise noted. The EQL Wiki-derived dataset and its adaptations are
therefore provided under CC BY-SA 4.0.

## License

The eqdps application source is licensed under the [MIT License](LICENSE).
The EQL Wiki-derived Plane of Sky dataset is licensed under CC BY-SA 4.0 as
described above.

EverQuest is a trademark of Daybreak Game Company LLC. eqdps is an independent
community project and is not affiliated with or endorsed by Daybreak Game
Company.
