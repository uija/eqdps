# Concurrency and Goroutine Architecture

This document describes the processes, goroutines, communication paths, and
shared state used by eqdps. It is based on a static analysis of the current
code.

## Process Model

The application uses one operating-system process per frontend:

- the Gio GUI is one executable and process;
- the Tview TUI is a separate executable and process;
- `logtest`, `spellcatalog`, and `skyquestdb` are standalone tools.

The tools are not started by either frontend. They only run when launched
explicitly.

There is no external logfile follower, parser, uploader, or audio process
managed by eqdps.

Go does not permanently assign a goroutine to one operating-system thread. The
Go runtime schedules goroutines across its thread pool. Gio's `app.Main()` is
the notable exception because it must remain on the platform's main thread.

## Central Logfile Pipeline

Logfile processing does not fan out into several competing parsers. One
logfile-follower goroutine reads complete lines and passes each line through the
consumers in order:

```text
One logfile follower goroutine
        │
        ▼
read one complete line
        │
        ├─ combat and DPS
        ├─ XP
        ├─ inventory-export detection
        ├─ Plane of Sky tracker
        ├─ kill/drop collector
        └─ event trigger matching
                │
                ├─ sound queue ─────────► sound worker
                └─ notification queue ──► notification worker
        │
        ▼
read next line
```

Everything on the left happens serially in logfile order. Combat, XP, Plane of
Sky, loot collection, and events therefore see the same ordering.

`engine.FollowWithPoll` explicitly guarantees that its callbacks run serially.
The follower checks for new data every 250 milliseconds while it is at the end
of the logfile.

## GUI Goroutine Graph

```text
OS process: eqdps GUI
│
├─ platform/main goroutine
│    └─ Gio app.Main()
│
├─ main-window goroutine
│    ├─ receives Gio window events
│    ├─ owns almost all GUI widget state
│    └─ consumes background-result channels
│
├─ logfile goroutine                     [while a logfile is open]
│    ├─ optional combat-history replay
│    ├─ drop-collector catch-up
│    └─ live logfile follower
│         └─ serial parser pipeline
│
├─ EQLDB event-upload runner             [long-lived]
│    └─ wakes every 5 seconds or when explicitly triggered
│
├─ desktop-notification worker           [long-lived]
│
├─ sound-dispatch worker                 [long-lived]
│    └─ one temporary monitor goroutine per playing sound
│
└─ DPS overlay window goroutine          [only while the overlay is open]
```

The original GUI goroutine remains in `app.Main()`. A second goroutine owns the
main window and its Gio event loop.

The logfile goroutine performs an optional history replay first and then starts
following the live logfile. Replay and live following do not run concurrently
for the same load operation.

The DPS overlay is an independent Gio window with its own event loop and theme.
It receives coalesced fight snapshots rather than accessing the mutable combat
tracker directly.

### Normal GUI Goroutine Count

At the application-code level, the normal steady state is approximately:

- 6 goroutines with a logfile open;
- 7 goroutines with a logfile open and the DPS overlay visible.

This does not include goroutines managed internally by Go, Gio, Oto, the HTTP
transport, or operating-system integration libraries.

### Temporary GUI Goroutines

The GUI starts temporary goroutines for:

- the native logfile chooser;
- a Plane of Sky initial scan;
- Plane of Sky catch-up;
- EQLDB connection and authentication;
- inventory upload;
- spell-icon extraction;
- every currently playing sound;
- Windows overlay initialization;
- inventory-export timer callbacks.

## TUI Goroutine Graph

```text
OS process: eqdps TUI
│
├─ main/Tview goroutine
│    ├─ terminal input
│    ├─ layouts and rendering
│    └─ executes QueueUpdateDraw callbacks
│
├─ logfile follower goroutine
│    └─ serial parser pipeline
│
├─ one-second ticker goroutine
│    ├─ closes idle fights
│    ├─ updates the XP display
│    └─ advances EQLDB dialogs and timers
│
├─ event-error forwarding goroutine
│
├─ EQLDB event-upload runner
│
├─ desktop-notification worker
│
└─ sound-dispatch worker
     └─ one temporary monitor goroutine per playing sound
```

The Tview event loop runs on the calling goroutine. Background work submits
widget changes through `QueueUpdateDraw`, keeping most widget state confined to
the Tview goroutine.

### Normal TUI Goroutine Count

The steady state is approximately 7 application-owned goroutines, excluding
runtime and library-owned goroutines.

### Temporary TUI Goroutines

The TUI starts temporary goroutines for:

- Plane of Sky catch-up;
- a Plane of Sky initial scan;
- combat-history replay;
- EQLDB authentication;
- inventory upload;
- spell-icon extraction;
- inventory-export timer callbacks;
- every currently playing sound.

## Event Runtime

Event matching is synchronous. The logfile goroutine matches a new live line
and attempts to enqueue the resulting deliveries.

There are two bounded delivery queues:

- a notification queue with a capacity of 32;
- a sound queue with a capacity of 32.

If a delivery queue is full, the new delivery is dropped and an error is
reported instead of blocking logfile parsing.

The event runtime starts two long-lived workers:

```text
live logfile line
       │
       ▼
synchronous event matching
       │
       ├─ notification queue ──► one notification worker
       │
       └─ sound queue ─────────► one sound-dispatch worker
                                      │
                                      └─ temporary playback monitor
                                          for each playing sound
```

Sounds can overlap. Each sound creates an Oto player and a short-lived goroutine
that monitors playback until the sound finishes or the application context is
cancelled.

Audio initialization, decoding, and playback do not run on the UI or logfile
goroutines.

## EQLDB Collection and Uploading

Observation collection and network uploading are separated:

```text
logfile goroutine
    │
    ├─ appends Plane of Sky events
    ├─ appends kill observations
    └─ appends loot observations
             │
             ▼
       durable JSONL queues
             │
             ▼
EQLDB runner goroutine
    ├─ wakes when explicitly triggered
    └─ otherwise checks every 5 seconds
             │
             ▼
        HTTP requests
```

The EQLDB runner processes Plane of Sky events, kills, and drops serially. It
does not start one HTTP goroutine per observation.

It uploads a maximum of three batches per wake, with up to 2,000 queue entries
per batch. The next wake continues with any remaining entries.

Inventory uploads use separate temporary goroutines because they originate
from detected inventory exports rather than the observation queues.

## UI Communication

### GUI

The main-window goroutine owns most GUI state. Background goroutines communicate
with it through buffered channels and call `Window.Invalidate()` to request a
new frame.

Important channels include:

- combat updates;
- Plane of Sky updates;
- file-chooser results;
- EQLDB UI events;
- event-runtime errors;
- EQLDB synchronization errors;
- overlay updates and close notifications.

Combat updates use a capacity-one channel and are coalesced. If the GUI has not
yet consumed the previous snapshot, the producer merges it with the newer
snapshot instead of building an unbounded backlog.

Plane of Sky updates also use a capacity-one channel. A newer snapshot replaces
an unconsumed older snapshot.

The overlay has its own capacity-one update channel and similarly receives only
the newest useful fight state.

### TUI

Background goroutines use `QueueUpdateDraw` to execute widget changes in the
Tview event loop.

This keeps widget access serialized, although it means a producer may wait for
the UI loop to process its requested update.

## Synchronization and Shared State

### In-Process Synchronization

The major mutex and atomic boundaries are:

- combat and XP state in the TUI is protected by a mutex;
- the active Plane of Sky tracker is protected by a frontend mutex;
- `PersistentTracker` also has its own internal mutex;
- the GUI's drop-collector pointer is protected by a mutex;
- each `dropcollector.Collector` has an internal mutex;
- the event dispatcher uses a read/write mutex when replacing event rules;
- event audio volume is stored atomically;
- the active spell-icon set is protected by a read/write mutex;
- decoded audio data is protected by the playback cache mutex;
- GUI overlay state and native window state have separate mutexes.

### Cross-Process Synchronization

Some files can be shared by independently running GUI and TUI processes.
Protection is currently inconsistent:

| State | Protection |
| --- | --- |
| Event configuration and event settings | Atomic replacement and explicit cross-process lock |
| EQLDB JSONL upload queues and queue cursors | Explicit cross-process lock |
| EQLDB event uploader | Cross-process upload lease |
| Inventory uploader | Separate cross-process upload lease |
| `eqldb.json` connection state | Atomic replacement, but no read-modify-write lock |
| Plane of Sky sidecar state | Atomic replacement and in-process mutex only |
| Per-log kill/drop checkpoint | Atomic replacement and in-process mutex only |
| Drop-collection opt-in setting | Atomic replacement only |
| GUI preferences | Atomic replacement; normally GUI-only |

Running one frontend at a time avoids most cross-process conflicts. Running the
GUI and TUI simultaneously against the same logfile is not completely safe for
the Plane of Sky and kill/drop checkpoint files.

## External Processes

The following operating-system processes may be launched in response to user
actions.

### Opening a Browser

- Linux: `xdg-open`
- Windows: `rundll32.exe`
- macOS: `open`

### Clipboard Integration

- Linux: `wl-copy`, `xclip`, or `xsel`
- Windows: `clip.exe`
- macOS: `pbcopy`

### Other Platform Integration

- The Linux native-file-dialog library may invoke Zenity.
- `beeep` uses native platform notification facilities and may use
  platform-dependent fallback helpers.

These helpers are not permanent eqdps background processes.

## Concurrency Concerns

### 1. GUI Logfile Switching Does Not Join the Old Follower

When the GUI loads another logfile or reloads history, it closes the old
follower's cancellation channel and immediately starts a new logfile goroutine.

The old follower checks cancellation between reads. At end-of-file it can take
up to approximately 250 milliseconds to notice cancellation. A history replay
checks cancellation periodically rather than on every line.

This creates a short period where old and new logfile goroutines may overlap.
The old goroutine can still publish a combat update after the new operation has
started.

Plane of Sky asynchronous updates contain their logfile path, and the GUI
rejects updates that do not belong to the current logfile. Combat updates do
not currently contain a logfile path or generation identifier.

This is the clearest application-level concurrency correctness risk found in
the current code.

Possible future solutions include:

- assigning every load operation a generation number and ignoring results from
  older generations;
- waiting for the old follower to exit before accepting results from the new
  one;
- combining cancellation with a generation check.

### 2. Shutdown Cancels Workers but Does Not Join Them

Both frontends signal cancellation when shutting down, but they do not use a
`sync.WaitGroup` or equivalent mechanism to confirm that all workers have
exited.

The TUI then closes the drop collector and saves Plane of Sky state. Mutexes
serialize these operations against concurrent parser activity, so this is not
automatically corrupting state. Nevertheless, shutdown is cancellation-based
rather than fully graceful.

The GUI relies more heavily on process termination after the window event loop
has returned.

### 3. Disk Writes Can Pause Logfile Processing

The following work may perform synchronous disk access from the logfile
goroutine:

- Plane of Sky state updates;
- Plane of Sky upload-queue appends;
- kill/drop queue appends;
- kill/drop checkpoint persistence.

A slow filesystem can therefore make the parser temporarily fall behind the
live logfile. It will not reorder records, but it can delay combat updates and
event detection.

Moving this work to independent unordered goroutines would not be safe because
it could change event order. Any future optimization should use an ordered
worker or ordered journal.

### 4. Cross-Process File Protection Is Incomplete

The event configuration and EQLDB upload queues have explicit cross-process
locking. Plane of Sky state, drop-collector checkpoints, and general EQLDB
connection settings do not have equivalent protection.

Two independently running frontends can therefore perform valid atomic writes
while still losing one process's logical update because both started from an
older in-memory copy.

### 5. The Parser Pipeline Is Deliberately Coupled

Combat, XP, Sky, loot collection, and event matching share the logfile
goroutine. A slow consumer delays the following consumers and the next line.

This coupling provides deterministic ordering. Splitting every parser into a
separate goroutine would require:

- a common ordered input stream;
- bounded queues and explicit backpressure;
- lifecycle and cancellation coordination;
- a policy for consumers that fall behind;
- ordered persistence where one observation depends on another.

The current single-stream architecture is simpler and safer unless profiling
shows that one consumer causes meaningful delays.

## Overall Assessment

The concurrency model is generally sensible:

- one ordered logfile pipeline;
- GUI and TUI widget confinement;
- bounded event-delivery queues;
- coalesced GUI snapshots;
- isolated network, notification, and audio workers;
- durable upload queues instead of one goroutine per observation.

The highest-value concurrency improvement would be to give GUI logfile-load
operations an identity so stale results from a cancelled follower cannot
replace newer state.

The next improvement would be explicit worker joining during shutdown. After
that, cross-process ownership of Plane of Sky state, drop checkpoints, and
`eqldb.json` should be clarified if simultaneous GUI and TUI operation is meant
to be supported.
