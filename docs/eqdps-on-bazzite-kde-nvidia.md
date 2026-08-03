# Running eqdps-gui on Bazzite (KDE, Nvidia)

This guide covers getting eqdps working on **Bazzite**, an immutable/atomic Linux distro (Fedora Atomic-based). Tested on the KDE Plasma + Nvidia variant. The same general approach should apply to other atomic/immutable distros (Fedora Silverblue/Kinoite, etc.).

## Why the normal Linux instructions don't directly work

Bazzite is an immutable OS — you can't just `sudo dnf install` build dependencies onto the base system. The base is read-only by design; package changes normally go through `rpm-ostree`, which requires a reboot per change and is discouraged for one-off dev/build tools.

The fix is to use **Distrobox**, which is preinstalled on Bazzite. It creates a normal, mutable Fedora container where you can install and build things exactly like a traditional Linux distro, without touching the host OS.

## Step 1: Create a build container

```bash
distrobox create -n eqdps-build -i fedora:latest
distrobox enter eqdps-build
```

You'll know you're inside the container because your shell prompt gets an `[eqdps-build]` prefix.

## Step 2: Install build dependencies (inside the container)

```bash
sudo dnf install -y golang make git \
  gcc pkgconf-pkg-config libxkbcommon-devel wayland-devel \
  vulkan-loader-devel libX11-devel libglvnd-devel \
  libxkbcommon-x11-devel libXcursor-devel libXfixes-devel
```

## Step 3: Clone and build eqdps

```bash
git clone https://github.com/uija/eqdps.git
cd eqdps
make
```

This builds both `dist/eqdps-gui` and `dist/eqdps` (TUI).

## Step 4: Install it properly (menu entry + icon)

```bash
sudo make install
```

**Known issue:** on Bazzite, this may partially fail with:
```
install: cannot create regular file '/usr/local/share/icons/hicolor/scalable/apps/eqdps.svg': Read-only file system
make: *** [Makefile:38: install] Error 1
```
The binaries and `.desktop` file install fine — only the icon step fails, because that particular system icon directory is read-only inside the container. Everything still works; the icon just needs to go somewhere writable instead:

```bash
mkdir -p ~/.local/share/icons/hicolor/scalable/apps
cp img/eqdps-icon.svg ~/.local/share/icons/hicolor/scalable/apps/eqdps.svg
```

## Step 5: Export to the host desktop

Still inside the container:

```bash
distrobox-export --bin "$(pwd)/dist/eqdps-gui"
distrobox-export --bin "$(pwd)/dist/eqdps"
distrobox-export --app eqdps
```

- The `--bin` exports let you run `eqdps-gui` / `eqdps` from any normal host terminal.
- The `--app` export adds eqdps to the KDE application launcher/menu with its icon, just like a natively installed app.

> **Tip:** Use `"$(pwd)/dist/..."` (absolute path) rather than a relative path like `./dist/eqdps-gui` — a relative path can produce a broken launcher script that fails with "file not found" once you're back on the host.

Exit the container when done:
```bash
exit
```

## Step 6: Fix the file picker (the big one)

On Bazzite, clicking **File → Open logfile → (any option)** in eqdps-gui appears to do nothing — no dialog, no error, no log output anywhere.

**Cause:** eqdps-gui doesn't have its own built-in file dialog. On Linux it shells out to an external file-picker program — specifically `zenity` (or `matedialog`/`qarma` as fallbacks). If none of these are installed *inside the same environment eqdps-gui is running in*, the click silently does nothing.

Since eqdps-gui runs inside the Distrobox container, having `zenity` on the host isn't enough — it needs to be installed **inside the container**:

```bash
distrobox enter eqdps-build
sudo dnf install -y zenity
exit
```

After that, relaunch `eqdps-gui` and the file picker works normally.

## Step 7: Enable EverQuest logging (in-game)

EverQuest doesn't log combat to a file by default. In-game, type:

```
/log
```

This needs to be re-enabled each login session. Once active, a file appears in your EQ install's `Logs` folder named like:
```
eqlog_CharacterName_ServerName.txt
```

## Step 8: Locating the log file (Steam/Proton installs)

If EverQuest Legends runs through Steam/Proton, the log lives deep inside the Proton prefix, something like:
```
~/.local/share/Steam/steamapps/compatdata/<APPID>/pfx/drive_c/users/Public/Daybreak Game Company/Installed Games/EverQuest Legends/Logs
```
(APPID will vary per install.) You can find the exact path via right-click → Properties on the log file in your file manager.

Since this is deeply nested and easy to lose track of, a convenience symlink helps:
```bash
ln -s "/full/path/to/EverQuest Legends/Logs" ~/EQ_Logs
```
Then in eqdps-gui's file picker (now working via zenity), just navigate to `~/EQ_Logs` to grab the log quickly.

**Note on external drives:** if the game/logs are on a separate mounted drive rather than under `/home`, the container may not see it at its normal host path — Distrobox exposes external mounts under `/run/host/<original path>` instead. In that case, point the symlink at the `/run/host/...` version of the path.

## Summary checklist

- [ ] Create a Distrobox Fedora container
- [ ] Install Go + build deps in the container
- [ ] Clone + `make` the project
- [ ] `sudo make install`, then manually copy the icon if the icon step fails (read-only fs)
- [ ] `distrobox-export --bin` (x2) and `--app eqdps`
- [ ] **`sudo dnf install zenity` inside the container** — this is the fix for the "Open logfile" doing nothing
- [ ] `/log` in-game to generate the log file
- [ ] Point eqdps-gui at the log file via the now-working file picker
