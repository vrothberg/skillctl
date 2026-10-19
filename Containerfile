FROM golang:1.27

WORKDIR /src/skillctl

RUN apt-get update && apt-get install -y \
    bats \
    btrfs-progs \
    libbtrfs-dev \
    libgpgme-dev \
    libdevmapper-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*
