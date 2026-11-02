#!/bin/sh
# Check the image make image built against a local build of the same commit (make image-check):
# the same version line, and the same output and exit code for the README's first command on a sample skill.
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/lib.sh"
need_docker
version=$(oci_version)
bin=$(mktemp -d)/skills
trap 'rm -rf "$(dirname "$bin")"' EXIT
( cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$bin" ./cmd/skills )
sample=examples/valid/read-files
want_v=$("$bin" version); got_v=$(docker run --rm "$SKILLS_IMAGE:latest" version)
[ "$want_v" = "$got_v" ] || die "image-check: version differs: local '$want_v', image '$got_v'"
set +e
want=$(cd "$ROOT" && "$bin" verify "$sample" 2>&1); want_rc=$?
got=$(docker run --rm -v "$ROOT:/work:ro" "$SKILLS_IMAGE:latest" verify "/work/$sample" 2>&1); got_rc=$?
set -e
want=$(printf '%s' "$want" | sed "s#$sample#SAMPLE#g"); got=$(printf '%s' "$got" | sed "s#/work/$sample#SAMPLE#g")
[ "$want_rc" = "$got_rc" ] && [ "$want" = "$got" ] || die "image-check: verify differs: local rc=$want_rc, image rc=$got_rc"
echo "image-check: $got_v; verify $sample exit $got_rc, same output as the local binary"
docker image inspect "$SKILLS_IMAGE:latest" --format 'image-check: size {{.Size}} bytes, user {{.Config.User}}'
