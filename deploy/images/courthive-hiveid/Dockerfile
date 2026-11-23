# courthive-hiveid (the CourtHive HiveID identity provider) for the jim-dot-tennis bundle.
#
# Built from the UNMODIFIED upstream repository at one exact commit. There is no
# fork: this Dockerfile fetches the source itself, checks that the commit it got
# is the one asked for, installs from upstream's own lockfile and builds.
# Build with ./build.sh (it passes the arguments below). Nothing is pushed.
#
#   UPSTREAM_COMMIT  full 40-character SHA of CourtHive/courthive-hiveid to build
#   VERSION          image version, <upstream package.json version>-jdt.<n>
#
# No source change is needed at commit 7931efa1 (0.5.0): pnpm-workspace.yaml has
# no link:../ overrides and the lockfile installs from the npm registry with
# --frozen-lockfile. The guard step below fails the build if a later commit
# introduces link:/file:/portal: specifiers, rather than silently editing them.

ARG NODE_IMAGE=node:26-alpine

# ---------------------------------------------------------------- source ----
FROM ${NODE_IMAGE} AS source
ARG UPSTREAM_REPO=https://github.com/CourtHive/courthive-hiveid.git
ARG UPSTREAM_COMMIT
ARG VERSION
RUN apk add --no-cache git
WORKDIR /src
RUN set -eu; \
    [ -n "${UPSTREAM_COMMIT:-}" ] || { echo "build argument UPSTREAM_COMMIT is required (use build.sh)"; exit 1; }; \
    [ -n "${VERSION:-}" ] || { echo "build argument VERSION is required (use build.sh)"; exit 1; }; \
    echo "$UPSTREAM_COMMIT" | grep -Eq '^[0-9a-f]{40}$' || { echo "UPSTREAM_COMMIT must be a full 40-character SHA"; exit 1; }; \
    git init -q .; \
    git remote add origin "$UPSTREAM_REPO"; \
    git fetch -q --depth 1 origin "$UPSTREAM_COMMIT"; \
    git -c advice.detachedHead=false checkout -q FETCH_HEAD; \
    got="$(git rev-parse HEAD)"; \
    [ "$got" = "$UPSTREAM_COMMIT" ] || { echo "checked out $got, expected $UPSTREAM_COMMIT"; exit 1; }; \
    upstream_version="$(node -p "require('./package.json').version")"; \
    case "$VERSION" in \
      "$upstream_version"-jdt.*) ;; \
      *) echo "VERSION $VERSION does not match upstream version $upstream_version (want $upstream_version-jdt.<n>)"; exit 1 ;; \
    esac; \
    echo "$upstream_version" > /upstream-version; \
    node -p "require('./package.json').packageManager" > /package-manager; \
    rm -rf .git; \
    echo "source: $UPSTREAM_REPO @ $got (version $upstream_version)"

# Guard: the bundle builds only from the npm registry. Refuse any local-path
# dependency or override (link:, file:, portal:) in package.json,
# pnpm-workspace.yaml or the lockfile importers.
RUN set -eu; \
    if grep -nE "(link|file|portal):" package.json pnpm-workspace.yaml; then \
      echo "local-path specifier found above: the bundle cannot build this commit as is"; exit 1; fi; \
    if grep -nE "^\s+(specifier|version): (link|file|portal):" pnpm-lock.yaml; then \
      echo "local-path specifier found in pnpm-lock.yaml"; exit 1; fi; \
    echo "no local-path specifiers"

# ----------------------------------------------------------------- build ----
FROM ${NODE_IMAGE} AS build
ENV CI=true
WORKDIR /app
COPY --from=source /package-manager /package-manager
# node 26 images ship no corepack: install the pnpm version upstream pins in
# package.json "packageManager" (pnpm@x.y.z) with npm, and check it.
RUN set -eu; pm="$(cat /package-manager)"; \
    case "$pm" in pnpm@*) ;; *) echo "unexpected packageManager '$pm'"; exit 1 ;; esac; \
    npm install -g --no-fund --no-audit "$pm"; \
    [ "pnpm@$(pnpm --version)" = "$pm" ] || { echo "installed pnpm $(pnpm --version), wanted $pm"; exit 1; }
COPY --from=source /src /app
# Full install (dev dependencies are needed for nest build). --ignore-scripts
# skips husky's "prepare" hook (no .git here) and the dependency build scripts;
# nothing this service needs is a native module (bcryptjs is pure JS; @swc/core
# ships prebuilt binaries as optional dependencies and the build uses tsc).
RUN pnpm install --frozen-lockfile --ignore-scripts
RUN pnpm build && test -f build/main.js && ls build/modules/email/templates/*.ejs >/dev/null

# Production dependencies only, into a clean tree, from the same lockfile.
WORKDIR /prod
RUN cp /app/package.json /app/pnpm-lock.yaml /app/pnpm-workspace.yaml /app/.npmrc ./ \
    && pnpm install --frozen-lockfile --prod --ignore-scripts
# Third-party licences of exactly the packages that ship (the production tree).
COPY third-party-licenses.mjs /tmp/third-party-licenses.mjs
RUN mkdir -p /licenses && node /tmp/third-party-licenses.mjs --node-modules /prod /licenses/THIRD-PARTY-LICENSES.txt

# --------------------------------------------------------------- runtime ----
FROM ${NODE_IMAGE}
ARG UPSTREAM_REPO=https://github.com/CourtHive/courthive-hiveid.git
ARG UPSTREAM_COMMIT
ARG VERSION
ENV NODE_ENV=production \
    PORT=3140
WORKDIR /app
# npm and the (absent) corepack are not needed at runtime.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx
COPY --from=build /prod/node_modules ./node_modules
COPY --from=source /src/package.json ./package.json
COPY --from=build /app/build ./build
# Upstream's one-off migration helper (hashes legacy plaintext passwords).
COPY --from=source /src/scripts/hash-plaintext-passwords.mjs ./scripts/hash-plaintext-passwords.mjs
COPY --from=source /src/LICENSE /licenses/LICENSE-courthive-hiveid
COPY --from=build /licenses/THIRD-PARTY-LICENSES.txt /licenses/THIRD-PARTY-LICENSES.txt
COPY --from=source /upstream-version /tmp/upstream-version
RUN set -eu; uv="$(cat /tmp/upstream-version)"; rm /tmp/upstream-version; \
    printf '%s\n' \
      "CourtHive courthive-hiveid (HiveID identity provider), image version $VERSION." \
      "" \
      "Built from the unmodified CourtHive repository $UPSTREAM_REPO" \
      "at commit $UPSTREAM_COMMIT (upstream version $uv)." \
      "No source changes were applied." \
      "" \
      "This is not an official CourtHive build. courthive-hiveid is MIT licensed," \
      "Copyright (c) 2026 Charles Allen; see LICENSE-courthive-hiveid." \
      "Third-party packages installed in the image: THIRD-PARTY-LICENSES.txt." \
      > /licenses/NOTICE
LABEL org.opencontainers.image.title="courthive-hiveid" \
      org.opencontainers.image.description="CourtHive HiveID identity provider for the jim-dot-tennis bundle; built from unmodified upstream; not an official CourtHive build" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$UPSTREAM_COMMIT" \
      org.opencontainers.image.source="$UPSTREAM_REPO" \
      org.opencontainers.image.licenses="MIT" \
      jdt.upstream.source="$UPSTREAM_REPO" \
      jdt.upstream.commit="$UPSTREAM_COMMIT" \
      jdt.upstream.version="${VERSION%-jdt.*}"
USER node
EXPOSE 3140
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:'+(process.env.PORT||3140)+'/health/core-db').then(r=>process.exit(r.ok?0:1),()=>process.exit(1))"
CMD ["node", "build/main.js"]
