# Events Integration Handoff

## Implementation Status

The integration described here is implemented on the
`feature/events-integration` branch:

- shared model, compiled matcher, catalogue, persistence, audio, notification,
  and icon-extraction packages;
- one live-line-only dispatch boundary used by both frontends;
- replay and Plane of Sky catch-up suppression regressions;
- the TUI Events page and contextual spell-icon prompt;
- the Gio Events workspace and bounded floating selectors;
- a shared persisted master notification-sound volume control;
- extraction and shared selection of distinct EverQuest UI spell-icon sets;
- cross-platform notification delivery through `beeep`;
- migrated `logtest` and `spellcatalog` developer utilities;
- CGO-disabled Windows amd64 GUI and TUI build verification.

A real Windows 11 desktop-notification test remains required before release.

This document captures the decisions for integrating the standalone
`eqlogevents` prototype into `eqdps`. Read it together with
[`PROJECT_CONTEXT.md`](PROJECT_CONTEXT.md), then inspect both codebases before
starting implementation.

## Source Project

The working prototype is located at:

```text
/home/jk/projects/go/eqlogevents
```

It already provides:

- spell-fade, plain-text, exact-text, and regular-expression triggers;
- active/inactive event configuration stored as JSON;
- configurable notification text;
- embedded notification sounds plus user-provided MP3 and WAV files;
- non-blocking sound and notification queues;
- a Tview event table and editors;
- an embedded spell catalogue;
- optional extraction of EverQuest spell icons for notifications.

The prototype's complete test suite passes. It also cross-compiles from Linux
to a Windows amd64 console executable with `CGO_ENABLED=0`.

## Integration Boundary

Do not start a second logfile follower inside `eqdps`. The existing engine owns
the logfile and must forward each genuinely new live line to the event matcher:

```text
new live logfile line
  -> existing combat / XP / Sky / EQLDB processing
  -> configured event matching
       -> sound queue
       -> desktop-notification queue
```

History replay, Plane of Sky checkpoint catch-up, initial Plane of Sky scans,
and other processing of old lines must never trigger event sounds or desktop
notifications.

Move the reusable behavior into shared packages under the root module. Keep
Tview and Gio widgets in their frontend modules. Likely shared responsibilities
are event definitions and matching, spell catalogue, persistence, audio
decoding/playback, spell-icon extraction, and desktop notification delivery.

Oto does not pull Gio into the TUI dependency graph and does not require CGO on
Windows. Preserve the existing separation in which building the TUI does not
require Gio or its Linux window-system development packages.

## TUI Design

Use `n` from the DPS screen to open the Events page. Existing keys can keep
their eqlogevents meanings because they are scoped to that page:

| Key | Events action |
| --- | --- |
| `a` | Activate or deactivate the selected event |
| `Enter` | Edit the selected event |
| `d` | Delete the selected event |
| `s` | Add a spell event |
| `t` | Add a text event |
| `r` | Add a regular-expression event |
| `q` / `Esc` | Return to the DPS screen |

The separate scopes allow `r` to remain combat reset on the DPS screen.

## GUI Design

Add an Events workspace to the existing left rail:

```text
DPS
SKY
EVENTS
SET
```

Start with the same columns as the prototype:

| Active | Title | Type | Notification | Sound |
| --- | --- | --- | --- | --- |

Provide actions for adding spell, text, and regular-expression events. Reuse
the bounded floating selector pattern developed for EQLDB inventory metadata for
class, spell, and sound choices. Editors must remain usable with small windows
and increased font scaling.

## Spell-Icon Setup

Spell-icon extraction is contextual setup, not an application-start prompt.
Do not add it to the existing first-run dialog sequence.

When the user enters Events:

1. Open the Events page.
2. If icon setup is `unknown` and a logfile is selected, ask whether to extract
   spell icons.
3. If no logfile is selected, show the page without prompting.
4. Selecting a logfile later must not open the prompt immediately. Ask the next
   time Events is entered.
5. If setup is `enabled` or `declined`, do not ask automatically.
6. Keep a manual setup action so a user who declined can enable icons later.

The TUI always starts with a logfile, so its first press of `n` can ask
immediately when the state is `unknown`.

This decision means no generalized startup-dialog queue is currently needed.
The existing EQLDB introduction already waits while Plane of Sky setup is open.

## Audio

The prototype uses:

- `github.com/ebitengine/oto/v3`;
- `github.com/hajimehoshi/go-mp3`;
- embedded MP3 sounds;
- user MP3 and WAV files below its configuration directory.

It creates one Oto context, decodes and caches PCM, and performs playback away
from the UI and log-processing goroutines. Preserve those properties. Audio
initialization or playback failure must remain non-fatal.

The embedded sounds come from:

<https://github.com/akx/Notifications>

That repository is dual-licensed under CC BY 3.0 or CC0. Use the CC0 option for
eqdps, document the source as a courtesy, and state that the original WAV files
were converted to MP3. Replace the placeholder attribution currently present
in the eqlogevents README before publishing the sounds through eqdps.

## Desktop Notifications

The prototype's `internal/notify` package is currently Linux-only and invokes
`notify-send`. On Windows and macOS it returns an unsupported-platform error.

The proposed replacement is `github.com/gen2brain/beeep`, which provides:

- Linux D-Bus notifications with a `notify-send` fallback;
- Windows 10/11 WinRT notifications with a PowerShell fallback;
- older Windows fallback behavior;
- macOS notification support.

Verify the final dependency with the existing CGO-disabled Windows cross-build
and perform a real Windows 11 notification test before release.

Notification duration is not portable. Desktop environments may override the
requested timeout. Rename or describe the prototype's `Permanent` option as a
request for persistence rather than a guarantee. The current Linux code only
omits its explicit ten-second timeout and therefore does not truly request a
permanent notification.

## Persistence

Both frontends must share one event configuration and one audio/icon directory
below the eqdps user configuration directory. Do not create separate TUI and
GUI state.

Decide during implementation whether to import an existing
`eqlogevents/events.json` once or leave the prototype configuration separate.
Do not let two running applications concurrently rewrite the same file without
an explicit locking strategy.

## Suggested Implementation Order

1. Move and adapt the shared event model, matcher, catalogue, persistence,
   audio, notification, and icon-extraction code.
2. Add a live-line-only event dispatch hook to the shared eqdps runtime and
   regression tests proving replayed lines do not notify.
3. Add the TUI Events page and contextual icon prompt.
4. Add the Gio Events workspace and editors.
5. Replace Linux-only notification delivery and test Linux plus Windows.
6. Update user documentation, licenses/attribution, build files, and release
   checks.

The prototype configuration remains separate; no automatic import from
`eqlogevents/events.json` is performed. Both eqdps frontends use the shared
locked configuration below the eqdps user-configuration directory.
