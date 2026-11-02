#!/bin/sh
# Build the skills image for this machine's platform and load it into the local engine (make image).
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/lib.sh"
need_docker
docker buildx version >/dev/null 2>&1 || die "docker buildx is not available"
version=$(oci_version)
platform=$(docker version --format '{{.Server.Os}}/{{.Server.Arch}}')
echo "image: one platform ($platform); a local engine cannot load a manifest list."
docker buildx build --file "$ROOT/Dockerfile" --load --platform "$platform" \
  --build-arg "GOLANG_IMAGE=$GOLANG_IMAGE" --build-arg "RUNTIME_IMAGE=$RUNTIME_IMAGE" \
  --build-arg "CREATED=$(oci_created)" --build-arg "REVISION=$(oci_revision)" --build-arg "VERSION=$version" \
  --tag "$SKILLS_IMAGE:latest" --tag "$SKILLS_IMAGE:$version" "$ROOT"
