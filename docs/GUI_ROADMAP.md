# Gio Frontend Roadmap

The graphical frontend will be developed as a separate `gui` Go module. The
existing TUI remains functional and keeps its independent dependency graph.
Both frontends consume the shared parser and application packages from the
root module.

## Running the Current Preview

From the repository root:

```bash
go run ./gui
```

The current preview opens and remembers EverQuest logfiles, replays selected
history ranges with progress, follows live combat, and renders shared parser
results in the graphical combat tree.

## 1. Application Shell

**Status:** initial preview available

- Create the isolated Gio module and executable.
- Establish the dark visual theme.
- Add a text menu bar, flat workspace rail, and bottom status bar.
- Render a placeholder DPS table from fake data.

**Complete when:** the resizable shell demonstrates the intended navigation
and visual direction without requiring a logfile.

## 2. Live DPS

**Status:** initial logfile opening, replay, and live-follow integration available

- Accept a logfile path and connect to the shared engine.
- Follow new log entries without blocking the window.
- Display current fights, combatants, damage, DPS, hits, crits, and duration.
- Show useful empty, loading, and error states.

The current integration remembers the last logfile, restores it in live-only
mode at startup, and offers initial/reload ranges for one, four, eight hours, or
the full file with replay progress.

**Complete when:** the GUI can serve as a minimal live graphical DPS meter.

## 3. Overlay Proof of Concept

**Status:** normal second-window live-data prototype available

Gio exposes native topmost behavior on Windows and macOS. Wayland compositors
control floating, stacking, opacity, and placement themselves. For Hyprland
0.55 and newer, the overlay's stable title can be matched in
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

The `move` coordinates are monitor-local and can be changed to the desired
starting location. Hyprland persists the floating size, but not an arbitrary
last dragged position; the rule therefore provides a stable chosen position.

- Open a separate compact current-fight window.
- Make it borderless, always on top, draggable, and resizable.
- Apply adjustable whole-window opacity.
- Add lock/unlock behavior and remember its geometry and opacity.
- Validate Windows first, then investigate Linux/X11 support.

**Complete when:** the overlay can remain legible over EverQuest while the game
is being played. Wayland support remains best effort because compositor rules
may prevent reliable stacking or placement.

## 4. Combat Feature Parity

- Show concurrent active fights and completed history.
- Add expandable melee, spell, proc, DoT, pet, and damage-shield details.
- Add filtering and history replay ranges.
- Reuse progress reporting and cancellation for large replays.
- Expose every operation through visible menus or controls.

**Complete when:** normal combat analysis no longer requires returning to the
TUI.

## 5. Plane of Sky Workspace

**Status:** database view, first-use scan, checkpoint catch-up, persistence, and live updates available

- Display class and quest progress, owned items, requirements, and sources.
- Show ready-to-turn-in and completed quests.
- Support hiding unstarted quests.
- Reuse initial scan, persistence, checkpoint catch-up, and notifications.

**Complete when:** the graphical tracker provides the same practical Plane of
Sky workflow as the TUI.

## 6. Desktop Polish and Releases

- Add logfile selection, recent files, and saved preferences.
- Remember main-window and overlay state.
- Add application icons, metadata, and friendly startup/error screens.
- Produce a Windows GUI build without a console window.
- Document Linux build requirements and add release automation.
- Compare GUI and TUI results against the same sample logs.

**Complete when:** ordinary Windows users can install and operate the GUI
without terminal knowledge.

## Development Order

```text
application shell
  -> live DPS
  -> overlay feasibility
  -> combat parity
  -> Plane of Sky parity
  -> desktop release
```

Each stage should remain runnable and be delivered in a small, reviewable
commit series. GUI-specific code and dependencies must stay inside the `gui`
module.
