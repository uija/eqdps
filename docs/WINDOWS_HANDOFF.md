# Windows 11 GUI Status

The first Windows 11 implementation and validation pass is complete. The GUI
ran stably during direct Windows testing, and a manually generated executable
was published with the GitHub `v0.1.0` release for wider testing.

To continue development from the active branch:

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
  internal top-right drag handle, and remembers its size and Windows position.
- Native Windows code applies the saved overlay opacity and position without a
  C compiler dependency.
- Overlay target selection follows the latest direct local-player target.
  Passive damage, ripostes, DoT ticks, unmatched procs, and rapid mirrored hits
  do not cause immediate focus changes.
- The combat/overlay idle timeout defaults to 15 seconds and is configurable
  from 5 to 60 seconds.
- `gui/sky_runtime.go` provides first-use Plane of Sky scanning, saved-offset
  catch-up, live updates, persistence, cancellation, and ready notifications.
- `gui/settings.go` stores state below the directory returned by
  `os.UserConfigDir()`; on Windows this is normally under `%AppData%`. Its
  replacement fallback supports repeated saves on Windows.
- Windows application icons are embedded in the generated executable.

The stable overlay title is:

```text
eqdps — Current Fight
```

## Windows Regression Checklist

Recheck the following when changing window or settings behavior:

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

## Implemented Windows Work

### Settings Replacement

`gui/settings.go` falls back to removing the destination before renaming when
Windows refuses to replace an existing `gui.json`. Regression tests cover
initial and repeated replacement.

### Native Overlay Opacity

The Windows implementation captures the native handle from Gio's
`app.Win32ViewEvent`, applies layered-window opacity to the overlay only, and
updates a visible overlay as its preference changes. The value is restored on
reopen and restart. It uses `golang.org/x/sys/windows`, with no C compiler
requirement.

### Window Behavior

The borderless, topmost overlay, internal drag handle, saved position, opacity,
and settings restoration worked during the initial Windows 11 validation.
Continue checking:

- whether `Decorated(false)` removes all standard decoration;
- whether the internal `system.ActionMove` drag handle moves the window;
- how an undecorated window can be resized on each edge;
- focus behavior when the overlay opens and while EverQuest is active;
- multi-monitor DPI and restored-size behavior.

Windows position handling remains platform-specific. Wayland positioning is
still compositor-managed.

### Windows Artifact

Build a GUI-subsystem executable that does not open a console window:

```powershell
go build -ldflags="-H=windowsgui" -o eqdps-gui.exe ./gui
```

Application icons are embedded through generated Windows resources. The
`v0.1.0` executable was built and uploaded manually for volunteer testing.
Version metadata, packaging, and release automation remain future work; a
Linux-only release workflow is not required first.

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
