Nanolight for Linux, rebuilt on mygo: the Mac's panel, settings, welcome,
results page, tray menu, Codegraff sync and Raycast extensions.

**Install**

- Ubuntu, Debian: download the `.deb` for your machine and open it, or
  `sudo apt install ./nanolight_<version>_amd64.deb`. apt updates it.
- Anywhere else: `curl -fsSL <this release's install.sh link> | sh`
  installs it in `~/.local` and it updates itself.
- Arch: `makepkg -si` with the `PKGBUILD` next to the archive.

**Coming from the earlier Linux build**

Install this one once by hand: the earlier build can't update to it. Your
settings, launch counts, Raycast extensions and sign-in carry over; a copy
of the earlier build that is still running is asked to quit, and its
"Open at Login" entry is replaced.

**Needs**

GTK 3 and WebKitGTK (for Google results in the panel; without it searches
open in the browser). Raycast extensions need Node.js 20 or later. The
tray icon needs a desktop that shows tray icons (KDE, or GNOME with the
AppIndicator extension).
