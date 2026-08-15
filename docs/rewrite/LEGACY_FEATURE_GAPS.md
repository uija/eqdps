# Legacy Feature Gaps

This document compares the current rewrite working tree with the legacy
application. It focuses on substantial user-visible features and runtime
behavior rather than small presentation differences.

## Already present in the rewrite

The rewrite already provides:

- the Gio application shell;
- live logfile following and time-range replay;
- a basic concurrent-fight DPS view with damage breakdowns;
- a separate DPS overlay window;
- session XP/hour;
- a basic Plane of Sky progression and inventory tracker;
- an Events configuration interface and embedded spell catalogue.

## 1. Event execution

The Events interface can create and persist configurations, but
`internal/module/events.Module.OnLogRow` is empty. Configured spell, text,
regular-expression, and timer events therefore do not trigger yet.

The legacy implementation also provided:

- compiled event matching;
- non-blocking notification and sound queues;
- embedded and user-provided MP3/WAV playback;
- shared sound volume control;
- extraction and selection of EverQuest spell-icon sets;
- spell icons in desktop notifications;
- runtime error handling and regular-expression validation.

Relevant legacy packages:

- `legacy/internal/event`
- `legacy/internal/eventruntime`
- `legacy/internal/audio`
- `legacy/internal/notify`
- `legacy/internal/spellicon`

The legacy persistence checkbox was not functional. The setting was stored,
but `legacy/internal/notify.Desktop.Send` discarded it before calling
`beeep.Notify`.

## 2. EQLDB integration

The rewrite has no current equivalent for:

- device-code connection and token management;
- inventory-export uploads;
- `/who` metadata collection;
- queued Plane of Sky event uploads;
- optional kill and personal-loot contribution;
- durable queues and retries after temporary upload failures.

Relevant legacy packages:

- `legacy/internal/eqldb`
- `legacy/internal/eqldbqueue`
- `legacy/internal/eqldbsync`
- `legacy/internal/inventorysync`
- `legacy/internal/dropcollector`
- `legacy/gui/eqldb_ui.go`

## 3. Preferences and saved application state

The Preferences menu item and sidebar entry currently have no implementation
in `internal/view/shell.go`.

The legacy GUI remembered or configured:

- recent logfiles;
- main-window size;
- overlay size and position;
- main-window font scale;
- overlay font scale;
- overlay opacity;
- combat and overlay idle timeout;
- drop-collection opt-in.

The current `internal/data.Config` stores only the last logfile, overlay
visibility, and Events configuration.

Relevant legacy files:

- `legacy/gui/preferences.go`
- `legacy/gui/settings.go`

## 4. Combat lifecycle and statistics parity

The rewrite supports basic concurrent fights and damage breakdowns, but its
combat implementation does not yet include several mature legacy behaviors:

- `Your enemies have forgotten you!` does not end the active fights;
- player death does not close every active fight;
- the idle timeout is hardcoded to 20 seconds;
- there is no death-grace period for same-timestamp or delayed DoT damage;
- there is no retained-fight handling for DoTs after Feign Death;
- there is no completed-fight history limit;
- pet damage is not merged into the owner's displayed combatant row;
- engaged/shared DPS behavior is not equivalent;
- min/max damage is collected but not displayed;
- the SDPS column is absent.

Current implementation:

- `internal/module/dps/combat.go`
- `internal/module/dps/fight.go`
- `internal/module/dps/damage.go`
- `internal/module/dps/module.go`

Legacy reference:

- `legacy/internal/combat/combat.go`

## 5. History and session controls

The rewrite provides one-hour, four-hour, eight-hour, one-day, and full replay
choices in `internal/view/historyselection.go`.

It does not yet provide:

- fight-name filtering;
- a Now action that returns to a fresh live session;
- explicit combat and XP reset;
- cancellable replay;
- recent-log selection;
- replay error presentation;
- configurable completed-fight history size.

The progress display in `internal/view/progress.go` reports replay progress but
does not offer cancellation.

Relevant legacy files:

- `legacy/gui/combat_controls.go`
- `legacy/gui/runtime.go`
- `legacy/gui/main.go`

## 6. XP tracking

The current `internal/module/xphour/module.go` calculates session XP/hour and
offers replay from the last observed level-up or zone change.

It does not yet provide the legacy application's:

- current-level progress;
- approximate time until the next level;
- level-up progress reset while preserving the full-session XP rate;
- explicit session reset;
- combined progress, rate, and ETA status display.

Legacy reference:

- `legacy/internal/xp/session.go`

## 7. Plane of Sky tracking

The rewrite includes basic progression, inventory, persistence, loot, and
turn-in handling under `internal/module/sky`.

The following legacy behavior is missing or incomplete:

- first-use consent before scanning existing history;
- cancellable initial scan and checkpoint catch-up;
- safe handling of replaced or truncated logfiles;
- correct partial-line checkpoint handling;
- restricting collected loot to Plane of Sky zones;
- removing destroyed items from holdings;
- handling rejected trades;
- persistent watched quests;
- rendering the watched section, which currently always reports zero;
- opening quest reward links instead of only logging the click;
- EQLDB upload of quest and rune events.

Legacy reference:

- `legacy/internal/skyquest`
- `legacy/gui/sky_runtime.go`
- `legacy/gui/sky_workspace.go`

## 8. DPS overlay parity

The current `internal/module/dps/overlay.go` creates a separate borderless,
topmost window.

It does not yet provide:

- saved size and position;
- configurable font scale;
- configurable opacity;
- configurable idle timeout;
- removal of a retained completed fight after the timeout;
- stable intentional-target selection during rapid or incidental combat;
- a completed fight timer (`overlay.go` still contains `TODO: Timer`);
- native Windows opacity and position restoration;
- Wayland compositor guidance.

Relevant legacy files:

- `legacy/gui/overlay.go`
- `legacy/gui/native_overlay_windows.go`
- `legacy/gui/preferences.go`
- `legacy/gui/settings.go`

## 9. Terminal and plain-text frontends

The current entry point starts only the Gio application. The following legacy
interfaces have not been migrated:

- the complete terminal frontend and its keyboard workflow;
- the plain-text parser comparison tool.

Legacy locations:

- `legacy/tui`
- `legacy/tools/logtest`

