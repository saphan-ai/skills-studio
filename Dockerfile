# The skills image: one static binary on a distroless base, run as a non-root user.
# `skills verify` and `skills check` read files only: no shell, no git and no network are needed at run time.
# The base images are pinned by version and digest in image/versions.env; make passes them and they win over
# these defaults. image/scripts/lib.sh refuses a base that is not pinned by digest.

ARG GOLANG_IMAGE=golang:1.26.10@sha256:670bb5aa5349ae03b8516764718486f4f16399134bdc9acd0c46a23f043ab05c
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
FROM ${GOLANG_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum* ./
COPY cmd ./cmd
COPY internal ./internal
COPY spec ./spec
ARG VERSION=unset
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/skills ./cmd/skills

FROM ${RUNTIME_IMAGE}
ARG CREATED=unset
ARG REVISION=unset
ARG VERSION=unset
LABEL org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.title="skills" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/saphan-ai/skills-studio"
COPY --from=build /out/skills /usr/local/bin/skills
USER 65532:65532
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/skills"]
