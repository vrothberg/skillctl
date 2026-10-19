FROM golangci/golangci-lint:v2.15.0 AS lint
FROM golang:1.27

COPY --from=lint /usr/bin/golangci-lint /usr/bin/golangci-lint

WORKDIR /src/skillctl

RUN apt-get update && apt-get install -y \
    bats \
    btrfs-progs \
    libbtrfs-dev \
    libgpgme-dev \
    libdevmapper-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Go embeds VCS info (commit, dirty flag) into binaries at build time.
# The bind-mounted repo is owned by the host user, not the container's
# root, so git refuses to read it without safe.directory.
RUN git config --global --add safe.directory /src/skillctl
