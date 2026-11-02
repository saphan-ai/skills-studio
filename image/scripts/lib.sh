# Shared by the image scripts. ROOT is the repository root; the Makefile exports it.
set -eu
if [ -z "${ROOT:-}" ]; then
  ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
  export ROOT
fi
die() { echo "$*" >&2; exit 1; }
need_docker() { command -v docker >/dev/null 2>&1 || die "docker is not on PATH"; }

# load_versions: export every KEY=VALUE of image/versions.env that the environment does not already set.
load_versions() {
  file=$ROOT/image/versions.env
  [ -f "$file" ] || die "missing $file"
  while IFS= read -r line || [ -n "$line" ]; do
    case $line in '' | \#*) continue ;; esac
    key=${line%%=*}; val=${line#*=}
    case $key in *[!A-Za-z0-9_]*) die "image/versions.env: refused key" ;; esac
    eval "isset=\${$key+x}"
    [ -n "$isset" ] || export "$key=$val"
  done <"$file"
}

# require_pinned: refuse a base image that is not pinned by a tag AND a sha256 digest.
require_pinned() {
  for var in GOLANG_IMAGE RUNTIME_IMAGE; do
    eval "ref=\${$var}"
    case $ref in
      *:*@sha256:????????????????????????????????????????????????????????????????) ;;
      *) die "image: refusing $var=$ref: a base image must be pinned by tag and sha256 digest (image/versions.env)" ;;
    esac
  done
}

oci_created() { date -u +%Y-%m-%dT%H:%M:%SZ; }
oci_revision() { git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown; }
oci_version() { git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev; }

load_versions
require_pinned
