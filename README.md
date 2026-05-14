![OpenCloud logo](https://raw.githubusercontent.com/opencloud-eu/opencloud/refs/heads/main/opencloud_logo.png)

[![status-badge](https://ci.opencloud.rocks/api/badges/3/status.svg)](https://ci.opencloud.rocks/repos/3)
 [![Matrix](https://img.shields.io/matrix/opencloud%3Amatrix.org?logo=matrix)](https://app.element.io/#/room/#opencloud:matrix.org)
 [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
 [![Docker image](https://img.shields.io/badge/ghcr.io-enclave--projects%2Fopencloud-2496ED?logo=docker&logoColor=white)](https://github.com/enclave-projects/opencloud/pkgs/container/opencloud)

# enclave-projects fork of OpenCloud

> [!IMPORTANT]
> **This is the [enclave-projects](https://github.com/enclave-projects) fork of [opencloud-eu/opencloud](https://github.com/opencloud-eu/opencloud).**
> It is fully API-compatible with upstream OpenCloud — every existing client, configuration file and CLI command works unchanged — and adds a small, focused set of additional capabilities listed below. If you do not need the fork-specific features, run upstream; if you do, this repository ships ready-to-use container images via the [`ghcr.io/enclave-projects/opencloud`](https://github.com/enclave-projects/opencloud/pkgs/container/opencloud) registry.

This is the main repository of the OpenCloud server, a Go microservices platform for file sync, share and collaboration. It contains the Go codebase for the backend services. For general information about upstream OpenCloud please visit [opencloud-eu on GitHub](https://github.com/opencloud-eu/) and [OpenCloud GmbH](https://opencloud.eu).

---

## Table of contents

1. [Features added by enclave-projects](#features-added-by-enclave-projects)
2. [Deploy on your own server](#deploy-on-your-own-server)
3. [Container image](#container-image)
4. [Build from source](#build-from-source)
5. [Technology](#technology)
6. [Security](#security)
7. [Getting involved](#getting-involved)

---

## Features added by enclave-projects

The following features exist **in this fork but not in upstream OpenCloud**. Each one is API-compatible — disabling the feature reverts to the upstream behavior byte-for-byte.

### Video previews and thumbnails

The thumbnails service now generates a preview image for video files by extracting a single representative frame with `ffmpeg`. The extracted frame flows through the existing image pipeline (resize → encode → checksum-keyed cache), so subsequent requests for the same file at the same resolution are served from cache.

| Property | Value |
| --- | --- |
| Supported formats (default) | `video/mp4`, `video/webm`, `video/quicktime` (mov), `video/x-matroska` (mkv), `video/x-msvideo` (avi) |
| Enabled by default? | Yes, when `ffmpeg` is on `PATH` at service startup |
| Container image | `ffmpeg` is preinstalled in `ghcr.io/enclave-projects/opencloud` |
| Cache key | File checksum (same path as image thumbnails) |
| API surface | None — existing `?preview=1` URL just returns 200 instead of 404 for video files |
| Permission model | Unchanged — `InitiateFileDownload` is enforced before any decoding |

**Tunables** (all optional; safe defaults applied):

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `THUMBNAILS_VIDEO_ENABLED` | `true` | Master switch for the feature |
| `THUMBNAILS_VIDEO_FFMPEG_BINARY` | `ffmpeg` | Path to (or name of) the ffmpeg binary |
| `THUMBNAILS_VIDEO_FFMPEG_TIMEOUT` | `30s` | Wall-clock cap on the ffmpeg subprocess |
| `THUMBNAILS_VIDEO_MAX_INPUT_FILE_SIZE` | `512MB` | Files larger than this are rejected with 403 |
| `THUMBNAILS_VIDEO_SEEK_OFFSET` | `00:00:01` | Position to extract the representative frame from |
| `THUMBNAILS_VIDEO_MIMETYPES` | five defaults above | Comma-separated MIME allow-list |
| `THUMBNAILS_VIDEO_MAX_OUTPUT_BYTES` | `134217728` (128 MiB) | Hard cap on ffmpeg stdout |

Full security model and operational notes live in [`services/thumbnails/README.md`](services/thumbnails/README.md). To disable the feature entirely, set `THUMBNAILS_VIDEO_ENABLED=0` — the rest of the thumbnails service is unaffected.

---

## Deploy on your own server

The fastest path to running the enclave-projects fork on your own machine is the prebuilt container image. A complete, copy-pasteable Docker Compose recipe with reverse-proxy, persistent volumes and security defaults lives in [`DEPLOYMENT.md`](DEPLOYMENT.md). The condensed version follows.

### Prerequisites

- A Linux server with at least 2 GB RAM and 10 GB free disk for the smallest demo. Production sizing depends on the number of users and the size of uploaded files.
- Docker 24+ and the Compose plugin (`docker compose ...`). Verify with `docker compose version`.
- A DNS name resolving to the server's public IP, e.g. `cloud.example.com`. The image expects TLS in production; you can terminate it at a reverse proxy or use the bundled flag `OC_INSECURE=true` for local testing only.
- Ports 80 and 443 open on the host (or whatever ports your reverse proxy uses).

### One-command demo (single host, self-signed)

The official upstream `opencloud-compose` recipe also works against this fork; just swap the image name. The smallest possible runnable invocation is:

```bash
docker run --rm -it \
  --name opencloud-demo \
  -p 9200:9200 \
  -e OC_URL=https://localhost:9200 \
  -e OC_INSECURE=true \
  -e IDM_CREATE_DEMO_USERS=true \
  -e PROXY_HTTP_ADDR=0.0.0.0:9200 \
  -v opencloud-data:/var/lib/opencloud \
  -v opencloud-config:/etc/opencloud \
  ghcr.io/enclave-projects/opencloud:latest \
  init server
```

Then open `https://localhost:9200` — the default admin password printed during `init` will appear in the container log. The demo users (e.g. `alan / demo`) are also created. **This mode is not suitable for production**: it self-signs a certificate, runs without any reverse proxy, and uses default secrets.

### Production-grade deploy

See [`DEPLOYMENT.md`](DEPLOYMENT.md) for:

- A complete `docker-compose.yml` covering OpenCloud, Traefik (TLS termination + Let's Encrypt), Collabora (web office), persistent volumes and named networks.
- Recommended environment variables, including the `THUMBNAILS_VIDEO_*` tunables for the fork-specific feature.
- Backup and upgrade procedures.
- Health-check, log-rotation and observability tips.

---

## Container image

Every push to `main` of this fork triggers the GitHub Actions workflow in [`.github/workflows/docker.yml`](.github/workflows/docker.yml). The workflow:

- Builds a multi-arch image (`linux/amd64`, `linux/arm64`).
- Includes the `ffmpeg` runtime needed for the video-thumbnail feature.
- Publishes to `ghcr.io/enclave-projects/opencloud` with the following tags:
    - `latest` — most recent commit on `main`.
    - `main-<short-sha>` — pinned to a specific commit.
    - `vX.Y.Z`, `X.Y`, `X` — when a `v*` tag is pushed.
    - `pr-<number>` — built but **not pushed** for pull requests (verification only).

You can pin to a specific SHA in production:

```bash
docker pull ghcr.io/enclave-projects/opencloud:main-46dc43d
```

Pull requests get a clean build verification without polluting the registry.

---

## Build from source

If you would rather build the binary yourself (for example to run on bare metal or to cross-compile), the upstream build instructions apply unchanged.

Generate the assets needed by e.g. the web UI and the builtin IDP:

```console
make generate
```

Then compile the `opencloud` binary:

```console
make -C opencloud build
```

That will produce the binary `opencloud/bin/opencloud`. It can be started as a local test instance with a two-step command:

```bash
opencloud/bin/opencloud init && opencloud/bin/opencloud server
```

This creates a server configuration (by default in `$HOME/.opencloud`) and starts the server.

> [!NOTE]
> To exercise the **video thumbnails** feature when building from source you also need `ffmpeg` available on `PATH` at runtime. On Debian/Ubuntu: `sudo apt-get install ffmpeg`. On Alpine: `apk add ffmpeg`. On macOS: `brew install ffmpeg`. The service detects ffmpeg at startup; if it is absent, video MIME types stay unregistered and the rest of the service behaves identically to upstream.

For more setup- and installation options consult the upstream [Development Documentation](https://docs.opencloud.eu/).

---

## Technology

Important information for contributors about the technology in use.

### Authentication

The OpenCloud backend authenticates users via [OpenID Connect](https://openid.net/connect/) using either an external IdP like [Keycloak](https://www.keycloak.org/) or the embedded [LibreGraph Connect](https://github.com/libregraph/lico) identity provider.

### Database

The OpenCloud backend does not use a database. It stores all data in the filesystem. By default, the root directory of the backend is `$HOME/.opencloud/`.

### Video thumbnail subsystem

The video pipeline added by this fork is documented in detail in [`services/thumbnails/README.md`](services/thumbnails/README.md). Highlights:

- `ffmpeg` is invoked directly via `exec.CommandContext` — never through a shell.
- The input is staged into a random-named temporary file (mode `0600`) and removed when the request returns.
- `ffmpeg` is launched with `-protocol_whitelist file,crypto,data` so it cannot reach the network, `-nostdin`, `-frames:v 1 -an -sn -dn` and `-f mjpeg -q:v 4 pipe:1` to do the minimum amount of work.
- Output is read through an `io.LimitReader` capped at `THUMBNAILS_VIDEO_MAX_OUTPUT_BYTES`.
- The subprocess is killed when `THUMBNAILS_VIDEO_FFMPEG_TIMEOUT` elapses.

---

## Security

If you find a security-related issue, please follow upstream OpenCloud's responsible-disclosure process: contact [security@opencloud.eu](mailto:security@opencloud.eu) immediately. For fork-specific issues that **only** affect features listed under [Features added by enclave-projects](#features-added-by-enclave-projects), please open a private security advisory on this repository.

---

## Getting involved

This fork tracks upstream OpenCloud closely. Contributions that improve fork-specific features (e.g. the video thumbnail pipeline) are very welcome here; contributions to the upstream codebase should preferably be sent to [opencloud-eu/opencloud](https://github.com/opencloud-eu/opencloud) so they benefit the whole community.

The OpenCloud server is released under [Apache 2.0](LICENSE). Start hacking — there are many ways to get involved:

- Reporting [issues or bugs](https://github.com/enclave-projects/opencloud/issues) (for fork-specific issues) or [upstream issues](https://github.com/opencloud-eu/opencloud/issues)
- [Writing documentation](https://github.com/opencloud-eu/docs)
- [Writing code or extending tests](https://github.com/enclave-projects/opencloud/pulls)
- [Reviewing code](https://github.com/enclave-projects/opencloud/pulls)
- Helping others in the upstream [community](https://app.element.io/#/room/#opencloud:matrix.org)

Every contribution is meaningful and appreciated! Please refer to the [Contribution Guidelines](CONTRIBUTING.md) before opening a pull request.
