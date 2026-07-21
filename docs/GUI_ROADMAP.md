# Gio Frontend Roadmap

The graphical frontend will be developed as a separate `gui` Go module. The
existing TUI remains functional and keeps its independent dependency graph.
Both frontends consume the shared parser and application packages from the
root module.

## Running the Graphical Frontend

From the repository root:

```bash
go run ./gui
```

The graphical frontend opens and remembers EverQuest logfiles, replays selected
history ranges with progress, follows live combat, and renders shared parser
results in the graphical combat tree.

## 1. Application Shell

**Status:** complete

- Create the isolated Gio module and executable.
- Establish the dark visual theme.
- Add a text menu bar, flat workspace rail, and bottom status bar.
- Render useful empty, loading, and error states before combat data exists.

**Complete when:** the resizable shell demonstrates the intended navigation
and visual direction without requiring a logfile.

## 2. Live DPS

**Status:** functional live combat view available

- Accept a logfile path and connect to the shared engine.
- Follow new log entries without blocking the window.
- Display current fights, combatants, damage, DPS, hits, crits, and duration.
- Show useful empty, loading, and error states.

The current integration remembers the last logfile, restores it in live-only
mode at startup, and offers initial/reload ranges for one, four, eight hours, or
the full file with replay progress.

**Complete when:** the GUI can serve as a minimal live graphical DPS meter.

## 3. DPS Overlay

**Status:** functional on Linux and Windows; Linux setup is compositor-specific

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

- A separate compact, borderless, draggable, and resizable window is available.
- It follows the latest direct local-player target and updates independently of
  main-window visibility.
- Its visible state and size are remembered, and retained fights expire with
  the configurable combat idle timeout.
- Wayland stacking, opacity, and screen position remain compositor-managed.
  Hyprland and KDE are documented; GNOME has a per-window manual workflow.
- Native Windows topmost behavior, opacity, and position restoration are
  implemented and passed the initial Windows 11 validation.

**Complete when:** the overlay can remain legible over EverQuest while the game
is being played. Wayland support remains best effort because compositor rules
may prevent reliable stacking or placement.

## 4. Combat Feature Parity

**Status:** core parity complete; continued live-log validation remains

- Show concurrent active fights and completed history.
- Add expandable melee, spell, proc, DoT, pet, and damage-shield details.
- Add filtering and history replay ranges.
- Reuse progress reporting and cancellation for large replays.
- Expose every operation through visible menus or controls.
- Reset the live combat and XP session without reopening the application.

**Complete when:** normal combat analysis no longer requires returning to the
TUI.

## 5. Plane of Sky Workspace

**Status:** database view, first-use scan, checkpoint catch-up, persistence, and live updates available

- Display class and quest progress, owned items, requirements, and sources.
- Show ready-to-turn-in and completed quests.
- Support hiding unstarted quests.
- Reuse initial scan, persistence, checkpoint catch-up, and nonblocking
  ready-to-turn-in notifications.

**Complete when:** the graphical tracker provides the same practical Plane of
Sky workflow as the TUI.

## 6. Desktop Polish and Releases

- Logfile selection, recent files, saved preferences, and remembered window
  sizes are implemented.
- Fedora build requirements and Linux compositor setup are documented.
- Application icons are embedded in Windows builds; broader desktop metadata
  remains optional polish.
- A console-free Windows GUI executable is available in the manually published
  `v0.1.0` tester release.
- GUI/TUI snapshot comparisons and a final real-log validation pass remain
  useful before release.
- Release automation is intentionally deferred until Windows packaging is
  defined, so Linux and Windows artifacts can share one workflow.

**Complete when:** ordinary Windows users can install and operate the GUI
without terminal knowledge.

The implemented Windows behavior, regression checklist, and remaining artifact
work are maintained in
[`WINDOWS_HANDOFF.md`](WINDOWS_HANDOFF.md).

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
