#!/usr/bin/env bash
# Casbin Gateway one-step install for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
#
# This downloads the nightly build, which is an automated build of master and
# not an official release. Use it for testing and development only.
#
# Optional environment variables:
#   INSTALL_DIR   where the executable and its data live
#                 (default: $HOME/.local/share/casbin-gateway)
#   BIN_DIR       where the "casbin-gateway" command is placed
#                 (default: $HOME/.local/bin)
#   NO_START      set to any value to install without starting Gateway
#   NO_AUTOSTART  set to any value to skip the login-time startup entry
#   NO_SHORTCUT   set to any value to skip the macOS bundle or the desktop entry

set -euo pipefail

REPO="apache/casbin-gateway"
TAG="nightly"
BASENAME="casbin-gateway-nightly"

INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/share/casbin-gateway}"
BIN_DIR="${BIN_DIR:-${HOME}/.local/bin}"
NO_START="${NO_START:-}"
NO_AUTOSTART="${NO_AUTOSTART:-}"
NO_SHORTCUT="${NO_SHORTCUT:-}"

info() { printf '%s\n' "$*"; }
die() { printf 'casbin-gateway: %s\n' "$*" >&2; exit 1; }

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need_cmd curl
need_cmd tar

# ── pick the archive for this machine ─────────────────────────────────────────
os="$(uname -s)"
case "${os}" in
	Linux)  osName="linux" ;;
	Darwin) osName="darwin" ;;
	*) die "unsupported operating system \"${os}\", build from source instead: https://github.com/${REPO}" ;;
esac

# Only x86_64 archives are published. macOS runs them under Rosetta 2, so an
# Apple Silicon machine still gets a working install; on Linux arm64 there is
# nothing to fall back to.
arch="$(uname -m)"
case "${arch}" in
	x86_64|amd64) archName="x86_64" ;;
	aarch64|arm64)
		[[ "${osName}" == "darwin" ]] || die "unsupported architecture \"${arch}\", build from source instead: https://github.com/${REPO}"
		archName="x86_64"
		info "No arm64 build is published, installing the x86_64 one to run under Rosetta 2"
		;;
	*) die "unsupported architecture \"${arch}\", build from source instead: https://github.com/${REPO}" ;;
esac

archive="${BASENAME}-${osName}-${archName}.tar.gz"
url="https://github.com/${REPO}/releases/download/${TAG}/${archive}"

# ── download ──────────────────────────────────────────────────────────────────
tmpDir="$(mktemp -d)"
trap 'rm -rf "${tmpDir}"' EXIT

info "Downloading ${url}"
curl -fsSL -o "${tmpDir}/${archive}" "${url}"

# ── install ───────────────────────────────────────────────────────────────────
info "Installing to ${INSTALL_DIR}"
mkdir -p "${INSTALL_DIR}" || die "cannot create ${INSTALL_DIR}, set INSTALL_DIR to a directory you can write to"

# Unpacking inside INSTALL_DIR keeps the final move on one filesystem, so it is
# a rename: an already running Gateway holds the old executable open, and
# writing over it in place would fail with "text file busy".
stage="${INSTALL_DIR}/.install"
rm -rf "${stage}"
mkdir -p "${stage}"
tar -xzf "${tmpDir}/${archive}" -C "${stage}" --strip-components=1

for executable in casbin-gateway casbin-gateway-desktop; do
	mv -f "${stage}/${executable}" "${INSTALL_DIR}/${executable}"
	chmod 755 "${INSTALL_DIR}/${executable}"
done
for legalFile in LICENSE NOTICE DISCLAIMER; do
	mv -f "${stage}/${legalFile}" "${INSTALL_DIR}/${legalFile}"
done
rm -rf "${stage}"

desktopExe="${INSTALL_DIR}/casbin-gateway-desktop"
iconPath="$("${desktopExe}" icon)"

# ── put a "casbin-gateway" command on PATH ────────────────────────────────────
# Gateway keeps its database, logs and temporary files in the working
# directory, so the command is a wrapper that always starts it in INSTALL_DIR.
# Without it, running "casbin-gateway" from somewhere else would quietly start
# a second, empty installation.
if [[ "${BIN_DIR}" == "${INSTALL_DIR}" ]]; then
	info "BIN_DIR is the install directory, so no wrapper was created"
else
	mkdir -p "${BIN_DIR}" || die "cannot create ${BIN_DIR}, set BIN_DIR to a directory you can write to"
	cat > "${BIN_DIR}/casbin-gateway" <<EOF
#!/usr/bin/env bash
# Written by the Casbin Gateway installer. Gateway reads and writes ./data,
# ./logs and ./tmp, so it always has to start in its own directory.
cd "${INSTALL_DIR}" || exit 1
exec "${INSTALL_DIR}/casbin-gateway" "\$@"
EOF
	chmod 755 "${BIN_DIR}/casbin-gateway"

	case ":${PATH}:" in
		*":${BIN_DIR}:"*) ;;
		*)
			shellRc=""
			case "${SHELL:-}" in
				*/zsh)  shellRc="${HOME}/.zshrc" ;;
				*/bash) shellRc="${HOME}/.bashrc" ;;
			esac

			if [[ -n "${shellRc}" ]]; then
				printf '\nexport PATH="%s:$PATH"\n' "${BIN_DIR}" >> "${shellRc}"
				info "Added ${BIN_DIR} to PATH in ${shellRc}, which takes effect in your next shell"
			else
				info "${BIN_DIR} is not on your PATH, add it to run Gateway by name"
			fi
			;;
	esac
fi

# ── the desktop application ───────────────────────────────────────────────────
# A macOS bundle and a Linux desktop entry, both pointing at the launcher rather
# than the server: it is what shows the window, and it starts the server itself.
appNote=""

install_macos_app() {
	appDir="${HOME}/Applications/Casbin Gateway.app"
	rm -rf "${appDir}"
	mkdir -p "${appDir}/Contents/MacOS" "${appDir}/Contents/Resources" || return

	# The launcher is copied in rather than symlinked or wrapped: a bundle whose
	# executable runs from outside it gets neither the Dock icon nor the name.
	cp "${desktopExe}" "${appDir}/Contents/MacOS/casbin-gateway-desktop"
	chmod 755 "${appDir}/Contents/MacOS/casbin-gateway-desktop"
	cp "${iconPath}" "${appDir}/Contents/Resources/appicon.icns"

	# Which is also why the data directory is recorded here: the copy inside the
	# bundle cannot find it by looking next to itself.
	printf '%s\n' "${INSTALL_DIR}" > "${appDir}/Contents/Resources/home"

	cat > "${appDir}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>Casbin Gateway</string>
	<key>CFBundleDisplayName</key><string>Casbin Gateway</string>
	<key>CFBundleIdentifier</key><string>org.apache.casbin-gateway</string>
	<key>CFBundleExecutable</key><string>casbin-gateway-desktop</string>
	<key>CFBundleIconFile</key><string>appicon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF

	# The download is not signed or notarized. Nothing curl writes is
	# quarantined, so this only matters for an archive that arrived some other
	# way, and an ad-hoc signature is the most this can do without a developer
	# certificate. Both are best-effort.
	xattr -dr com.apple.quarantine "${appDir}" >/dev/null 2>&1 || true
	command -v codesign >/dev/null 2>&1 && codesign --force --deep --sign - "${appDir}" >/dev/null 2>&1

	# Finder caches the bundle by path, so a reinstall over the old one keeps
	# showing the old icon until it is told the directory changed.
	touch "${appDir}"
	desktopExe="${appDir}/Contents/MacOS/casbin-gateway-desktop"
	appNote="${appDir}"
}

install_linux_app() {
	entryDir="${XDG_DATA_HOME:-${HOME}/.local/share}/applications"
	iconDir="${XDG_DATA_HOME:-${HOME}/.local/share}/icons/hicolor/512x512/apps"
	mkdir -p "${entryDir}" "${iconDir}" || return
	cp "${iconPath}" "${iconDir}/org.apache.casbin-gateway.png"

	entry="${entryDir}/org.apache.casbin-gateway.desktop"
	cat > "${entry}" <<EOF
[Desktop Entry]
Type=Application
Name=Casbin Gateway
Comment=Local gateway for AI agents
Exec=${desktopExe}
Path=${INSTALL_DIR}
Icon=org.apache.casbin-gateway
Terminal=false
Categories=Development;Network;
StartupWMClass=casbin-gateway-desktop
EOF
	chmod 644 "${entry}"
	command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "${entryDir}" >/dev/null 2>&1

	# Most desktops only show an icon on ~/Desktop if the file is executable and
	# marked trusted; the chmod is the half that is portable.
	if [[ -d "${HOME}/Desktop" ]]; then
		cp "${entry}" "${HOME}/Desktop/org.apache.casbin-gateway.desktop"
		chmod 755 "${HOME}/Desktop/org.apache.casbin-gateway.desktop"
		command -v gio >/dev/null 2>&1 && gio set "${HOME}/Desktop/org.apache.casbin-gateway.desktop" metadata::trusted true >/dev/null 2>&1
	fi
	appNote="${entry}"
}

if [[ -z "${NO_SHORTCUT}" ]]; then
	if [[ "${osName}" == "darwin" ]]; then
		install_macos_app
	else
		install_linux_app
	fi
fi

# ── start at login ────────────────────────────────────────────────────────────
# The launcher owns this entry so that the tray's "Start at Login" checkbox and
# the installer are never out of step. Older installs started the server on its
# own; those entries would now bring up a second copy.
rm -f "${HOME}/Library/LaunchAgents/org.apache.casbin-gateway.plist"
if command -v systemctl >/dev/null 2>&1 && [[ -f "${HOME}/.config/systemd/user/casbin-gateway.service" ]]; then
	systemctl --user disable --now casbin-gateway.service >/dev/null 2>&1 || true
	rm -f "${HOME}/.config/systemd/user/casbin-gateway.service"
	systemctl --user daemon-reload >/dev/null 2>&1 || true
fi

autostartNote=""
if [[ -z "${NO_AUTOSTART}" ]]; then
	if "${desktopExe}" autostart on; then
		autostartNote="yes"
	fi
fi

info ""
info "Casbin Gateway is installed in ${INSTALL_DIR}"
info "Its database, logs and temporary files stay in that directory."
info "It serves this machine only, and signs you in there as admin without a password."
info "Closing its window leaves it running in the tray; quit it from there."
info "Without the window: \"casbin-gateway start\", \"casbin-gateway stop\", \"casbin-gateway status\"."
if [[ -n "${appNote}" ]]; then
	info "The application is ${appNote}"
fi
if [[ -n "${autostartNote}" ]]; then
	info "It will start when you log in; turn that off from the tray menu."
fi
info ""

if [[ -n "${NO_START}" ]]; then
	info "Start it from the Casbin Gateway application, or with: casbin-gateway start"
	exit 0
fi

# The launcher forks the server and the window, so installing does not occupy
# this terminal beyond the startup it waits for.
cd "${INSTALL_DIR}"
"${desktopExe}" >/dev/null 2>&1 &
disown 2>/dev/null || true
