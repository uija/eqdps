# Automatic Checks and Dialogs

This document inventories UI that eqdps opens automatically during startup or
in response to parser activity. It also lists important silent startup checks.

## Summary

The application currently has three kinds of automatic startup UI:

1. the Plane of Sky enable-and-scan question;
2. the Wayland overlay information dialog;
3. the EQLDB introduction.

There is also an automatic Plane of Sky catch-up progress overlay. It reports
work but does not ask for a decision.

Two contextual dialogs can open automatically later:

1. spell-icon setup when the user enters Events;
2. missing inventory metadata after eqdps detects an inventory export.

## Automatic Startup Dialogs

### Plane of Sky Enable-and-Scan

**Frontends:** GUI and TUI

**When it is checked:**

- TUI: during startup for its required logfile;
- GUI: during startup when reopening a saved logfile;
- GUI: whenever another logfile is selected.

**Condition:**

- a logfile is available; and
- no Plane of Sky state sidecar exists for that character and server.

The sidecar is stored beside the logfile as:

```text
Character_Server_PoS.json
```

The user can enable tracking and scan the existing logfile, or choose
`Not now`.

`Not now` does not create a persistent decline. The question therefore appears
again when that character/logfile is loaded on a later run.

Selecting enable starts an initial full-log scan with a progress overlay.

Relevant code:

- [`gui/sky_runtime.go`](../gui/sky_runtime.go)
- [`tui/main.go`](../tui/main.go)
- [`internal/skyquest/persistence.go`](../internal/skyquest/persistence.go)

### Wayland Overlay Information

**Frontend:** GUI only

**When it is checked:**

- during startup when saved GUI preferences say the DPS overlay should be
  visible;
- when the user manually enables the overlay.

**Condition:**

- the session appears to be Wayland through `XDG_SESSION_TYPE` or
  `WAYLAND_DISPLAY`; and
- `wayland_overlay_notice_shown` is false.

The information dialog is shown before opening the overlay. Closing it stores
the preference and then opens the requested overlay.

Relevant code:

- [`gui/overlay.go`](../gui/overlay.go)
- [`gui/settings.go`](../gui/settings.go)
- [`gui/main.go`](../gui/main.go)

### EQLDB Introduction

**Frontends:** GUI and TUI

**When it is checked:**

- automatically after the main application UI is available;
- the TUI checks from its one-second UI tick;
- the GUI checks during frame updates.

**Condition:**

- the introduction has not been marked as shown;
- there is no saved EQLDB access token;
- no other EQLDB modal is open.

The GUI additionally waits until:

- combat-history loading has finished;
- Plane of Sky loading has finished;
- the Plane of Sky setup dialog is closed;
- the Wayland information dialog is closed;
- the About dialog is closed.

The TUI waits until the main DPS page is the front page.

The dialog has a 30-second automatic close timer. Connecting, explicitly
closing, or pressing Escape marks the introduction as shown. Timer expiry closes
it without marking it as shown, so it can appear again on the next run.

The remembered state is stored in `eqldb.json`.

Relevant code:

- [`gui/eqldb_ui.go`](../gui/eqldb_ui.go)
- [`tui/eqldb_ui.go`](../tui/eqldb_ui.go)
- [`internal/eqldb/store.go`](../internal/eqldb/store.go)

## Automatic Startup Progress

### Plane of Sky Catch-up

If a Plane of Sky sidecar exists, eqdps compares its logfile checkpoint with
the current logfile size and processes the missing range.

For a backlog of more than approximately 5 MiB:

- the GUI shows `Catching up Plane of Sky tracker…`;
- the TUI starts on a catch-up progress page.

Smaller TUI backlogs are processed before the main UI starts. Smaller GUI
backlogs are processed asynchronously without the progress overlay.

This is not a confirmation question, although the visible operation can be
cancelled.

## Contextual Automatic Dialogs

### Spell-Icon Setup

**Frontends:** GUI and TUI

**When it is checked:** whenever the user enters the Events workspace/page.

**Condition:**

- spell-icon setup state is `unknown`;
- a logfile is selected;
- the EverQuest spell-icon source can be detected.

It is deliberately not checked during general application startup.

The user can extract the detected icon sets, leave setup undecided, or decline
future automatic prompts. Manual icon setup remains available from Events.

Relevant code:

- [`gui/events_workspace.go`](../gui/events_workspace.go)
- [`tui/events_ui.go`](../tui/events_ui.go)
- [`internal/eventstore/store.go`](../internal/eventstore/store.go)

### Inventory Metadata Needed

**Frontends:** GUI and TUI

**When it is checked:** after the live logfile observer detects that an
EverQuest inventory export has completed.

**Condition:**

- EQLDB is connected;
- the export is not rejected by the upload cooldown;
- no recent matching `/who` result supplied level, race, and classes.

The application asks for the missing values before uploading the inventory.
This can happen at any time while following a live logfile.

Relevant code:

- [`gui/eqldb_ui.go`](../gui/eqldb_ui.go)
- [`tui/eqldb_ui.go`](../tui/eqldb_ui.go)

## Effective Startup Order

### GUI With a Remembered Logfile

```text
load GUI preferences
        │
        ▼
saved logfile still exists?
        │
        ├─ no  ─► start without a logfile
        │
        └─ yes
             │
             ▼
       Plane of Sky state check
             │
             ├─ no state ─► Plane of Sky setup dialog
             │
             └─ state ────► catch up, with progress UI if large
             │
             ▼
       reopen live logfile follower
             │
             ▼
       saved overlay requested on Wayland?
             │
             └─ yes ─► Wayland information dialog
             │
             ▼
       after blocking setup UI is gone
             │
             └─► EQLDB introduction when applicable
```

### TUI

```text
required logfile path
        │
        ▼
Plane of Sky sidecar exists?
        │
        ├─ no  ─► start on Plane of Sky setup dialog
        │
        └─ yes
             │
             ├─ small backlog ─► catch up before opening UI
             │
             └─ large backlog ─► start on catch-up progress page
        │
        ▼
main DPS page becomes active
        │
        └─► EQLDB introduction when applicable
```

## Silent Startup Checks

### GUI Preferences and Saved Logfile

The GUI loads `gui.json`, normalizes missing or invalid sizes and scales, and
checks whether the last logfile still exists. A missing logfile produces status
text rather than a dialog.

### Drop-Collector Checkpoint

The kill/drop collector opens its per-logfile checkpoint and catches up from
that byte offset when collection is enabled. It does not show progress.

If its checkpoint exceeds the logfile size, collection reports an error rather
than silently resetting the checkpoint.

### Plane of Sky Upload History

When Plane of Sky tracking is loaded, eqdps checks whether that logfile has
already been scanned for uploadable rune and quest events. If not, it performs
a one-time event backfill and records that the logfile was scanned.

### EQLDB Queue Upload

When an access token exists, the background synchronization runner immediately
tries to upload queued Plane of Sky events. Opted-in kill/drop observations are
also uploaded. It retries every five seconds or when explicitly awakened.

Failures produce status notices rather than startup questions.

### Event Runtime

Event definitions, audio volume, and icon selection are loaded during startup.
Notification and sound workers start without opening Events. Errors are shown
in the status area.

### Persisted DPS Overlay

Outside Wayland's one-time information case, a saved visible-overlay preference
reopens the overlay without a question.

## Things That Do Not Open Automatically

The following UI requires an explicit user action:

- logfile chooser;
- combat-history chooser and replay;
- combat filter;
- application preferences;
- kill/drop collection opt-in;
- normal EQLDB management;
- EQLDB authentication after choosing Connect;
- event add/edit/delete forms;
- event volume and icon-set settings;
- manual spell-icon setup;
- About dialog;
- Plane of Sky workspace after tracking is configured.
