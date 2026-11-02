#!/bin/sh
# Build once for PUSH_PLATFORMS into a local OCI layout, scan every platform of that layout with Docker Scout,
# and only then copy those exact scanned bytes to the registry (make push). Any failed scan publishes nothing.
# Publishing is the owner's act.
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/lib.sh"
need_docker
docker scout version >/dev/null 2>&1 || die "push: refusing to publish: the docker scout plugin is not installed"
command -v skopeo >/dev/null 2>&1 || die "push: refusing to publish: skopeo is not installed (it copies the scanned layout)"
docker buildx version >/dev/null 2>&1 || die "docker buildx is not available"
docker buildx inspect skills-builder >/dev/null 2>&1 || docker buildx create --name skills-builder --driver docker-container >/dev/null
version=$(oci_version)
case $version in *dirty*) die "push: refusing to publish a dirty tree ($version)";; esac
name="$SKILLS_REGISTRY/$SKILLS_IMAGE"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
echo "push: $name:latest and $name:$version, platforms $PUSH_PLATFORMS; the registry is written only after every scan is green."
docker buildx build --builder skills-builder --file "$ROOT/Dockerfile" --platform "$PUSH_PLATFORMS" \
  --provenance=mode=max --sbom=true --output "type=oci,dest=$work/skills.tar,name=$name:scan" \
  --build-arg "GOLANG_IMAGE=$GOLANG_IMAGE" --build-arg "RUNTIME_IMAGE=$RUNTIME_IMAGE" \
  --build-arg "CREATED=$(oci_created)" --build-arg "REVISION=$(oci_revision)" --build-arg "VERSION=$version" "$ROOT"
mkdir "$work/skills" && tar -xf "$work/skills.tar" -C "$work/skills" && rm -f "$work/skills.tar"
digest=$(tr -d ' \n\t' <"$work/skills/index.json" | grep -o '"digest":"sha256:[0-9a-f]*"' | head -n 1 | cut -d '"' -f 4)
[ -n "$digest" ] || die "push: refusing: the layout has no index digest"
refused=0
for platform in $(printf '%s' "$PUSH_PLATFORMS" | tr ',' ' '); do
  echo "scout: $platform $digest"
  docker scout cves "oci-dir://$work/skills" --platform "$platform" --only-severity "$SCOUT_REFUSE_SEVERITIES" --exit-code \
    || { echo "scout: refusing $platform" >&2; refused=1; }
done
[ "$refused" -eq 0 ] || die "push: refusing to publish: a Docker Scout scan did not pass. Nothing was pushed."
if [ -n "${PUSH_DRY_RUN:-}" ]; then
  # The rehearsal runs the same copy into a second local layout, so the source reference is proven without a registry.
  skopeo copy --all --preserve-digests "oci:$work/skills" "oci:$work/copy-check:check" >/dev/null \
    || die "push: PUSH_DRY_RUN — the scanned layout could not be copied"
  echo "push: PUSH_DRY_RUN — scanned $digest and copied it locally; would copy it to $name:$version and $name:latest"
  exit 0
fi
for tag in "$version" latest; do
  skopeo copy --all --preserve-digests "oci:$work/skills" "docker://$name:$tag"
  got=$(docker buildx imagetools inspect "$name:$tag" --format '{{.Manifest.Digest}}')
  [ "$got" = "$digest" ] || die "push: $name:$tag carries $got, not the scanned $digest"
done
echo "push: published $name:$version and $name:latest at $digest"
