# tippecanoe for the places archive of task `nen-ban-do` (web-admin/scripts/build-basemap.sh).
#
# WHY A DOCKERFILE: felt/tippecanoe publishes no image and no binary (checked 12/10/2026: no package on
# ghcr.io or Docker Hub; GitHub releases carry no assets), and the build machine is CentOS 7 whose glibc
# cannot run a modern build. So it is compiled here from a PINNED COMMIT on a base PINNED BY DIGEST — both
# values live in deploy/basemap/basemap.py (BASEMAP_IMAGES, TIPPECANOE_COMMIT) and reach this file only as
# build args, which basemap.py refuses to emit while unpinned. Built with no context:
#   docker build -t <tag> --build-arg BASE_IMAGE=<ubuntu@sha256:…> --build-arg TIPPECANOE_COMMIT=<40 hex> - < tippecanoe.Dockerfile
#
# Ubuntu's apt packages are not version-pinned; apt verifies them against Ubuntu's archive keys shipped in
# the pinned base. The tippecanoe source is checked to be exactly the pinned commit before it is compiled.

ARG BASE_IMAGE
FROM ${BASE_IMAGE} AS build
ARG TIPPECANOE_COMMIT
RUN test -n "$TIPPECANOE_COMMIT" \
 && apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends \
      ca-certificates git make gcc g++ libsqlite3-dev zlib1g-dev \
 && rm -rf /var/lib/apt/lists/*
RUN git init -q /src \
 && git -C /src fetch -q --depth 1 https://github.com/felt/tippecanoe.git "$TIPPECANOE_COMMIT" \
 && git -C /src checkout -q --detach FETCH_HEAD \
 && test "$(git -C /src rev-parse HEAD)" = "$TIPPECANOE_COMMIT" \
 && make -C /src -j"$(nproc)" tippecanoe tile-join

FROM ${BASE_IMAGE}
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get -y install --no-install-recommends libsqlite3-0 zlib1g \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/tippecanoe /src/tile-join /usr/local/bin/
# The task runs it as the Jenkins user with --network none; this is the fallback for anyone else.
USER 65534:65534
WORKDIR /work
