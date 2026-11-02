#!/bin/sh
# Scan the local image with Docker Scout (make scan). Fails closed: Scout missing, a scan that cannot run,
# or any finding at SCOUT_REFUSE_SEVERITIES refuses, naming the image.
. "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/lib.sh"
need_docker
docker scout version >/dev/null 2>&1 || die "scan: refusing: the docker scout plugin is not installed"
image=${1:-$SKILLS_IMAGE:latest}
docker scout cves "local://$image" --only-severity "$SCOUT_REFUSE_SEVERITIES" --exit-code \
  || die "scan: refusing $image: the scan failed or a $SCOUT_REFUSE_SEVERITIES finding was reported"
echo "scan: $image has no $SCOUT_REFUSE_SEVERITIES finding"
