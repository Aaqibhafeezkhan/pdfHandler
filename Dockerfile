# ── Stage 1: Build ────────────────────────────────────────────────────────────
# Track a maintained Go release: Go only ships security fixes for the two most
# recent majors, so an EOL toolchain bakes known stdlib CVEs into the binary.
# alpine is used here only for compilation; musl libc issues don't affect the
# Go binary because CGO is disabled (pure Go + no CGO dependencies at build time).
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Cache dependency downloads separately from source changes.
# go mod verify checks the downloaded modules against go.sum before they build.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o pdfhandler .

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# debian:bookworm-slim is chosen over alpine to avoid musl libc compatibility
# issues with ghostscript and poppler-utils in edge cases (see DESIGN.md).
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ghostscript \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

# Run as non-root to satisfy Kubernetes PodSecurity restricted policy
RUN groupadd -r -g 1001 pdfhandler && useradd -r -u 1001 -g pdfhandler pdfhandler

WORKDIR /app

COPY --from=builder /app/pdfhandler .
COPY static/ ./static/

# The app never writes into /app, so leave it owned by root and world-readable:
# the runtime user can execute the binary but cannot replace it. Scratch space
# comes from /tmp (an emptyDir in k8s), which is why readOnlyRootFilesystem
# works in deployment.yaml.
RUN chown -R root:root /app && chmod -R a-w /app

USER pdfhandler

# pdfcpu resolves config dir via $HOME — set to /tmp so it works as non-root
ENV HOME=/tmp
# Disable Gin's debug output and its startup warning outside Kubernetes too
ENV GIN_MODE=release

EXPOSE 8080

CMD ["./pdfhandler"]
