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

set -euo pipefail

REPO="apache/casbin-gateway"
TAG="nightly"
BASENAME="casbin-gateway-nightly"

INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/share/casbin-gateway}"
BIN_DIR="${BIN_DIR:-${HOME}/.local/bin}"
NO_START="${NO_START:-}"

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

mv -f "${stage}/casbin-gateway" "${INSTALL_DIR}/casbin-gateway"
chmod 755 "${INSTALL_DIR}/casbin-gateway"
for legalFile in LICENSE NOTICE DISCLAIMER; do
	mv -f "${stage}/${legalFile}" "${INSTALL_DIR}/${legalFile}"
done
rm -rf "${stage}"

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

info ""
info "Casbin Gateway is installed in ${INSTALL_DIR}"
info "Its database, logs and temporary files stay in that directory."
info "Sign in as \"admin\" with the password \"123\", and change it right away."
info ""

if [[ -n "${NO_START}" ]]; then
	info "Start it with: casbin-gateway"
	exit 0
fi

info "Starting Casbin Gateway, press Ctrl-C to stop it..."
cd "${INSTALL_DIR}"
exec "${INSTALL_DIR}/casbin-gateway"
