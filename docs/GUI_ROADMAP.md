# Gio Frontend Roadmap

The graphical frontend will be developed as a separate `gui` Go module. The
existing TUI remains functional and keeps its independent dependency graph.
Both frontends consume the shared parser and application packages from the
root module.

## 1. Application Shell

- Create the isolated Gio module and executable.
- Establish the dark visual theme.
- Add a text menu bar, flat workspace rail, and bottom status bar.
- Render a placeholder DPS table from fake data.

**Complete when:** the resizable shell demonstrates the intended navigation
and visual direction without requiring a logfile.

## 2. Live DPS

- Accept a logfile path and connect to the shared engine.
- Follow new log entries without blocking the window.
- Display current fights, combatants, damage, DPS, hits, crits, and duration.
- Show useful empty, loading, and error states.

**Complete when:** the GUI can serve as a minimal live graphical DPS meter.

## 3. Overlay Proof of Concept

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
