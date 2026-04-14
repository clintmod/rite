# scripts/coexistence/Dockerfile
#
# One-command v1.0 coexistence gate: builds an image with both `task`
# (upstream go-task) and `rite` (built from local source) installed, then
# runs smoke.sh to assert that the two tools do not corrupt each other's
# caches or configs in a shared working directory.
#
# Usage (from repo root):
#   docker build -t rite-coexistence -f scripts/coexistence/Dockerfile .
#   docker run --rm rite-coexistence
#
# Both steps are wired into the `verify:coexistence` task in Ritefile.yml.

# ---- build stage: compile rite from the local source tree ----
FROM golang:1.26-bookworm AS rite-build
WORKDIR /src
# Cache modules first for faster incremental builds.
COPY go.mod go.sum ./
RUN go mod download
# Then the rest of the source. .dockerignore scopes what actually ships.
COPY . .
RUN CGO_ENABLED=0 go build -o /out/rite ./cmd/rite

# ---- runtime stage: minimal image with both tools on PATH ----
FROM debian:bookworm-slim

# Install curl + ca-certificates so we can pull the upstream task installer,
# and bash for the smoke script.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        curl \
    && rm -rf /var/lib/apt/lists/*

# Install upstream go-task via the official installer script. Pinned to a
# known release tag so the smoke is reproducible across runs.
ARG TASK_VERSION=v3.45.4
RUN curl -fsSL https://taskfile.dev/install.sh \
    | sh -s -- -d -b /usr/local/bin "${TASK_VERSION}" \
    && task --version

# Drop the locally-built rite binary in place.
COPY --from=rite-build /out/rite /usr/local/bin/rite
RUN rite --version

# Smoke script runs as the entrypoint; exit code carries the verdict.
COPY scripts/coexistence/smoke.sh /usr/local/bin/smoke.sh
RUN chmod +x /usr/local/bin/smoke.sh

WORKDIR /work
ENTRYPOINT ["/usr/local/bin/smoke.sh"]
