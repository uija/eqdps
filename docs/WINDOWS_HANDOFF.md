# Windows 11 GUI Handoff

This is the starting point for the Windows 11 implementation and validation
pass. The active branch is `refactor/frontend-separation`. Pull it before
starting:

```powershell
git switch refactor/frontend-separation
git pull
go version
go run ./gui
```

The module files currently declare Go 1.26.4. The TUI remains independently
buildable with `go build -o eqdps.exe ./tui`; Gio dependencies are isolated in
the `gui` module.

## Current GUI State

- `gui/main.go` owns the main window, menus, workspaces, modal overlays, status
  bar, and main-window size capture.
- `gui/runtime.go` replays history, follows the logfile, closes idle fights,
  and sends immutable combat snapshots to both windows.
- `gui/overlay.go` owns a separate Gio window and a separate text shaper. It
  updates independently when the main window is hidden or minimized.
- The overlay requests `app.Decorated(false)` and `app.TopMost(true)`, has an
  internal top-right drag handle, and remembers its size.
- Overlay target selection follows the latest direct local-player target.
  Passive damage, ripostes, DoT ticks, unmatched procs, and rapid mirrored hits
  do not cause immediate focus changes.
- The combat/overlay idle timeout defaults to 15 seconds and is configurable
  from 5 to 60 seconds.
- `gui/sky_runtime.go` provides first-use Plane of Sky scanning, saved-offset
  catch-up, live updates, persistence, cancellation, and ready notifications.
- `gui/settings.go` stores state below the directory returned by
  `os.UserConfigDir()`; on Windows this is normally under `%AppData%`.

The stable overlay title is:

```text
eqdps — Current Fight
```

## First Windows Run

Validate the existing behavior before adding native code:

1. Start `go run ./gui` and open an EverQuest logfile.
2. Confirm that the file chooser, recent-file menu, history ranges, and replay
   progress modal work.
3. Open the DPS overlay and verify borderless rendering, always-on-top behavior,
   the drag handle, resizing, and close/reopen behavior.
4. Minimize or cover the main window and confirm that the overlay continues to
   update during combat.
5. Fight multiple mobs and verify that mirrored, passive, proc, and riposte
   damage do not make the overlay jump between targets.
6. Confirm that an idle fight clears after 15 seconds and that changing the
   timeout under Preferences takes effect.
7. Restart twice and verify the last logfile, overlay visibility, font scales,
   timeout, and both window sizes.
8. Exercise Plane of Sky consent, catch-up cancellation, saved state, the SKY
   table, and the clickable `PoS: N ready` status segment.

## Known Windows Work

### Settings Replacement

`gui/settings.go` currently finishes an atomic save with `os.Rename` over the
existing `gui.json`. Windows does not replace an existing destination with
`os.Rename`. Plane of Sky persistence already contains a Windows-aware replace
pattern in `internal/skyquest/persistence.go`; apply or extract the same safe
behavior for GUI settings and add a regression test before relying on repeated
preference saves.

### Native Overlay Opacity

`nativeOpacityAvailable()` in `gui/preferences.go` currently returns `false` on
every platform. The opacity value is stored but is not applied by eqdps. During
the Windows pass:

- enable the slider only when native support is actually available;
- apply opacity to the overlay window, not the main window;
- update a visible overlay immediately while the slider moves;
- restore the saved value when reopening or restarting;
- prefer a Go/Win32 implementation that does not introduce a C compiler
  requirement for ordinary Windows builds.

Gio does not expose portable top-level screen coordinates or a documented
native window handle. Inspect the Windows backend and choose the smallest
stable integration before designing position persistence around it.

### Window Behavior

Gio documents `TopMost` support on Windows, but it still needs real validation
over the EverQuest client. Check:

- whether `Decorated(false)` removes all standard decoration;
- whether the internal `system.ActionMove` drag handle moves the window;
- how an undecorated window can be resized on each edge;
- focus behavior when the overlay opens and while EverQuest is active;
- multi-monitor DPI and restored-size behavior.

If native Windows position persistence is added, keep it platform-specific.
Wayland positioning must remain compositor-managed.

### Windows Artifact

After runtime behavior is stable, test a GUI-subsystem build that does not open
a console window:

```powershell
go build -ldflags="-H=windowsgui" -o eqdps-gui.exe ./gui
```

Application icon and version metadata can follow. Release automation is
intentionally deferred until the Windows artifact and packaging choices are
settled; Linux-only automation is not required first.

## Verification Before Committing

Run the shared and both frontend suites:

```powershell
go test ./...
go test ./tui/...
go test ./gui/...
go vet ./...
go vet ./tui/...
go vet ./gui/...
go build -o eqdps.exe ./tui
go build -o eqdps-gui.exe ./gui
git diff --check
```

Also perform the real-window checks above. Unit tests cannot establish native
topmost, opacity, focus, drag, resize, DPI, or console-subsystem behavior.

