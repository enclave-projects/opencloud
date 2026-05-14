# Deploying the enclave-projects fork of OpenCloud

This guide walks you, step by step, through running the [enclave-projects fork](https://github.com/enclave-projects/opencloud) of OpenCloud on your own server. The deployment uses the prebuilt container image at `ghcr.io/enclave-projects/opencloud` — which already includes `ffmpeg` so the video-thumbnail feature works out of the box — and Docker Compose for orchestration.

> [!NOTE]
> Everything in this guide is **fork-compatible**: the same Compose file works for upstream `opencloud-eu/opencloud` if you swap the `image:` line. The only fork-specific extras are the prebuilt image and the `THUMBNAILS_VIDEO_*` environment variables documented in [section 6](#6-fork-specific-tunables).

---

## Table of contents

1. [Before you start](#1-before-you-start)
2. [Quick start — five-minute local demo](#2-quick-start--five-minute-local-demo)
3. [Production deployment with Docker Compose](#3-production-deployment-with-docker-compose)
    - [3.1 Server preparation](#31-server-preparation)
    - [3.2 DNS and TLS](#32-dns-and-tls)
    - [3.3 The Compose file](#33-the-compose-file)
    - [3.4 The .env file](#34-the-env-file)
    - [3.5 First boot](#35-first-boot)
    - [3.6 Creating users](#36-creating-users)
4. [Upgrading](#4-upgrading)
5. [Backups and disaster recovery](#5-backups-and-disaster-recovery)
6. [Fork-specific tunables](#6-fork-specific-tunables)
7. [Observability and logs](#7-observability-and-logs)
8. [Troubleshooting](#8-troubleshooting)

---

## 1. Before you start

You'll need:

| Requirement | Minimum | Recommended |
| --- | --- | --- |
| OS | Any modern Linux (kernel ≥ 5.10) | Debian 12, Ubuntu 22.04 LTS, Fedora 39, Alpine 3.20 |
| RAM | 2 GB | 4 GB (more for many concurrent users) |
| Disk | 10 GB | 100 GB+ on the volume that will hold user data |
| CPU | 2 cores | 4 cores |
| Docker | 24+ | 24+ with the Compose plugin |
| DNS | A name resolving to the server's IP | Plus a wildcard or per-subdomain record if you want Collabora |
| Ports | 80 + 443 open inbound | Same, with no other service bound to them |

Verify Docker is ready:

```bash
docker --version
docker compose version
```

Both commands should print a version ≥ 24.

> [!WARNING]
> Do **not** run any of the commands in this guide as `root` unless explicitly noted. Create a non-privileged user (`adduser opencloud`) and add it to the `docker` group instead.

---

## 2. Quick start — five-minute local demo

This is the fastest way to confirm the image runs on your machine. It is **not suitable for production**: it self-signs a certificate, binds to `localhost`, and creates demo users with well-known passwords.

```bash
docker run --rm -it \
  --name opencloud-demo \
  -p 9200:9200 \
  -e OC_URL=https://localhost:9200 \
  -e OC_INSECURE=true \
  -e IDM_CREATE_DEMO_USERS=true \
  -e PROXY_HTTP_ADDR=0.0.0.0:9200 \
  -v opencloud-demo-data:/var/lib/opencloud \
  -v opencloud-demo-config:/etc/opencloud \
  ghcr.io/enclave-projects/opencloud:latest \
  init server
```

The first `init server` call prints an `admin` password — copy it. Then open <https://localhost:9200>, accept the self-signed certificate, and log in as `admin` with that password or as one of the demo users (`alan / demo`, `marie / demo`, …).

To confirm the **video thumbnail feature** is working, upload an `.mp4` (or `.webm`, `.mov`, `.mkv`, `.avi`) file to your home space and look at the file list — a generated poster image should appear next to the filename.

Stop the demo with <kbd>Ctrl</kbd>+<kbd>C</kbd>. Volumes survive container removal; drop them with `docker volume rm opencloud-demo-data opencloud-demo-config` if you want a fresh start.

---

## 3. Production deployment with Docker Compose

This is the recommended path for a real instance. We use [Traefik](https://traefik.io/) as the reverse proxy because it auto-renews Let's Encrypt certificates and integrates with Docker labels, but any reverse proxy (Caddy, Nginx, HAProxy) works.

### 3.1 Server preparation

1. Install Docker per the [official documentation](https://docs.docker.com/engine/install/) for your distro.
2. Create the deploy directory and a non-root owner:

    ```bash
    sudo mkdir -p /opt/opencloud
    sudo chown "$USER:$USER" /opt/opencloud
    cd /opt/opencloud
    ```

3. Open ports 80 and 443 in your firewall. On UFW: `sudo ufw allow 80,443/tcp`.
4. Make sure your DNS A/AAAA record (`cloud.example.com`) points to the server. Verify with `dig +short cloud.example.com`.

### 3.2 DNS and TLS

Traefik will obtain a free Let's Encrypt certificate the first time it serves your hostname. For that to work:

- The DNS name **must** already resolve to the server.
- Ports **80 and 443 must be reachable** from the public internet.
- You must accept Let's Encrypt's Terms of Service by providing an email in the Compose file.

If your environment cannot satisfy any of these (for example, you are on a private LAN), use Traefik's [DNS-01 challenge](https://doc.traefik.io/traefik/https/acme/#dnschallenge) or terminate TLS at a different proxy. Both are out of scope here, but the OpenCloud part of the configuration does not change.

### 3.3 The Compose file

Save the following as `/opt/opencloud/docker-compose.yml`. Substitute your hostname for `cloud.example.com` in the `traefik.http.routers.opencloud.rule` label or — better — keep it as `${OC_DOMAIN}` and set the value in `.env` (see [3.4](#34-the-env-file)).

```yaml
name: opencloud

services:
  traefik:
    image: traefik:v3.1
    container_name: traefik
    restart: unless-stopped
    command:
      - "--log.level=INFO"
      - "--providers.docker=true"
      - "--providers.docker.exposedByDefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.web.http.redirections.entrypoint.to=websecure"
      - "--entrypoints.web.http.redirections.entrypoint.scheme=https"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=${LE_EMAIL}"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "traefik-letsencrypt:/letsencrypt"
    networks:
      - opencloud

  opencloud:
    image: ghcr.io/enclave-projects/opencloud:latest
    container_name: opencloud
    restart: unless-stopped
    entrypoint: /bin/sh
    command: ["-c", "opencloud init || true; opencloud server"]
    environment:
      # --- Core ---
      OC_URL: "https://${OC_DOMAIN}"
      OC_LOG_LEVEL: "info"
      OC_LOG_COLOR: "false"
      OC_LOG_PRETTY: "false"
      OC_INSECURE: "false"
      # --- Admin password is auto-generated on first boot and printed in the log ---
      # Or set it explicitly here once you've rotated it through the web UI:
      # IDM_ADMIN_PASSWORD: "${IDM_ADMIN_PASSWORD}"
      # --- Storage ---
      STORAGE_USERS_DRIVER: "decomposed"
      STORAGE_SYSTEM_DRIVER: "decomposed"
      OC_BASE_DATA_PATH: "/var/lib/opencloud"
      # --- Search / antivirus ---
      SEARCH_EXTRACTOR_TYPE: "basic"
      # --- Fork-specific: video thumbnails (see section 6) ---
      THUMBNAILS_VIDEO_ENABLED: "true"
      THUMBNAILS_VIDEO_MAX_INPUT_FILE_SIZE: "1GB"
      THUMBNAILS_VIDEO_FFMPEG_TIMEOUT: "30s"
    volumes:
      - "opencloud-data:/var/lib/opencloud"
      - "opencloud-config:/etc/opencloud"
    networks:
      - opencloud
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.opencloud.rule=Host(`${OC_DOMAIN}`)"
      - "traefik.http.routers.opencloud.entrypoints=websecure"
      - "traefik.http.routers.opencloud.tls=true"
      - "traefik.http.routers.opencloud.tls.certresolver=letsencrypt"
      - "traefik.http.services.opencloud.loadbalancer.server.port=9200"
    healthcheck:
      test: ["CMD", "curl", "-fk", "https://localhost:9200/status.php"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s

networks:
  opencloud:
    driver: bridge

volumes:
  opencloud-data:
  opencloud-config:
  traefik-letsencrypt:
```

### 3.4 The `.env` file

Compose reads variables from a `.env` file in the same directory. Save the following as `/opt/opencloud/.env` and edit the placeholders.

```bash
# Your fully qualified hostname. The DNS record must point at this server.
OC_DOMAIN=cloud.example.com

# Email used for Let's Encrypt registration. Renewal failures will be sent here.
LE_EMAIL=admin@example.com

# Pin a specific image tag in production. `latest` follows main, but you can
# also pin to a sha-suffixed tag for reproducible deploys:
# OPENCLOUD_TAG=main-46dc43d
OPENCLOUD_TAG=latest
```

If you'd like the Compose file to honor `${OPENCLOUD_TAG}` instead of hard-coding `:latest`, change the `image:` line to `ghcr.io/enclave-projects/opencloud:${OPENCLOUD_TAG}`.

Tighten file permissions:

```bash
chmod 600 /opt/opencloud/.env
```

### 3.5 First boot

```bash
cd /opt/opencloud
docker compose pull
docker compose up -d
```

Watch the OpenCloud log for the auto-generated `admin` password:

```bash
docker compose logs -f opencloud | grep -i "Initial admin user"
```

The first start can take 30 to 60 seconds while Traefik fetches a certificate and OpenCloud completes `init`. Once you see `serving HTTP requests`, open `https://cloud.example.com` in a browser and sign in as `admin`.

> [!IMPORTANT]
> The first thing to do in the web UI is **rotate the admin password**, then save the new one in your password manager or a secrets vault. Capturing it from the container log is fine for setup, but the log is shared with every operator who has Docker socket access.

### 3.6 Creating users

OpenCloud uses the embedded LibreGraph Connect IdP by default. Add or remove users via the **Admin Settings → Users** screen in the web UI. For programmatic provisioning, the Microsoft Graph-compatible API at `https://${OC_DOMAIN}/graph/v1.0/users` is the supported entry point — see the [upstream docs](https://docs.opencloud.eu/) for details.

If you'd rather plug in an external IdP (Keycloak, Authentik, Okta, Google Workspace…), set the `IDP_*` and `OAUTH_*` environment variables before first boot. The embedded IdP is the easiest path for a single-tenant home server; an external IdP is recommended for multi-tenant or team setups.

---

## 4. Upgrading

Because every commit to `main` of this fork rebuilds and pushes a new image (see [`.github/workflows/docker.yml`](.github/workflows/docker.yml)), upgrades are just a `pull` + `up -d`:

```bash
cd /opt/opencloud
docker compose pull
docker compose up -d
```

For production we recommend pinning to a sha-suffixed tag (for example `main-46dc43d`) rather than `latest` so you control exactly when an upgrade happens. Roll forward by editing `OPENCLOUD_TAG` in `.env` and re-running the two commands above; roll back by setting it to the previous tag.

> [!NOTE]
> Changelog entries for fork-specific changes live under [`changelog/unreleased/`](changelog/). Read them before upgrading to spot any new environment variables you may want to set.

---

## 5. Backups and disaster recovery

OpenCloud stores everything in the filesystem; there is no database. To take a consistent backup, **stop the service first** so no writes are in flight:

```bash
cd /opt/opencloud
docker compose stop opencloud
sudo tar -C /var/lib/docker/volumes -czf /backups/opencloud-$(date +%F).tgz \
  opencloud_opencloud-data \
  opencloud_opencloud-config
docker compose start opencloud
```

Move the resulting tarball off the host (S3, Backblaze, restic, borgbackup…). To restore:

1. Stop the service: `docker compose stop opencloud`.
2. Delete the existing volumes: `docker volume rm opencloud_opencloud-data opencloud_opencloud-config`.
3. Re-create them and untar the backup into `/var/lib/docker/volumes/`.
4. Start the service: `docker compose start opencloud`.

Test restore on a staging host **before** you rely on backups in production.

---

## 6. Fork-specific tunables

The video thumbnail pipeline added by this fork exposes the following environment variables. All are optional; the defaults are safe for most deployments.

| Variable | Default | Description |
| --- | --- | --- |
| `THUMBNAILS_VIDEO_ENABLED` | `true` | Master switch. Set to `false` to fall back to upstream behavior (no video previews). |
| `THUMBNAILS_VIDEO_FFMPEG_BINARY` | `ffmpeg` | Path to the ffmpeg binary. Resolved via `PATH` at startup. If unresolvable, the feature self-disables. |
| `THUMBNAILS_VIDEO_FFMPEG_TIMEOUT` | `30s` | Wall-clock cap on the subprocess. Killed when exceeded. |
| `THUMBNAILS_VIDEO_MAX_INPUT_FILE_SIZE` | `512MB` | Files larger than this are rejected with 403 **before** any extraction. |
| `THUMBNAILS_VIDEO_SEEK_OFFSET` | `00:00:01` | Position of the representative frame. Server-controlled; never derived from user input. |
| `THUMBNAILS_VIDEO_MIMETYPES` | five defaults | Comma-separated MIME allow-list. Defaults to `mp4`, `webm`, `quicktime`, `x-matroska`, `x-msvideo`. |
| `THUMBNAILS_VIDEO_MAX_OUTPUT_BYTES` | `134217728` | Hard cap on bytes read from ffmpeg stdout. |

Apply changes with `docker compose up -d` — Compose will only restart the affected service.

> [!TIP]
> If your users frequently upload very long videos and the default `00:00:01` offset lands on a black intro frame, set `THUMBNAILS_VIDEO_SEEK_OFFSET=00:00:05`. The change applies on the next request that misses the cache; existing cached thumbnails are not regenerated.

---

## 7. Observability and logs

- **Container logs:** `docker compose logs -f opencloud` (or `traefik`). Logs are JSON when `OC_LOG_PRETTY=false`, so they pipe cleanly into Loki, Vector, or any other log shipper.
- **Health endpoint:** `https://${OC_DOMAIN}/status.php` returns a JSON status block. The Compose file uses it for the Docker health check.
- **Metrics:** OpenCloud exposes Prometheus metrics at `:9205/metrics` per service. To make these reachable, add an extra `ports:` entry or a separate Traefik route — `:9200` is the only port currently exposed externally.

---

## 8. Troubleshooting

**`docker compose pull` fails with `unauthorized`.** GHCR images for this repository are public; if you hit `unauthorized`, you are pulling a private build (`pr-<n>`). Authenticate with `docker login ghcr.io -u <github-user>` using a personal access token that has the `read:packages` scope.

**Browser shows `your connection is not private`.** Let's Encrypt could not issue a certificate. Check the Traefik log: `docker compose logs traefik | grep -i acme`. The most common causes are DNS not resolving yet, port 80 blocked by a firewall, or rate-limit exhaustion (you used the staging URL too often).

**Video files don't get thumbnails.** Check `THUMBNAILS_VIDEO_ENABLED=true` and that `ffmpeg` is on `PATH` inside the container: `docker compose exec opencloud which ffmpeg`. The official image ships `ffmpeg`; if you swapped to a custom image, install it (`apk add ffmpeg` on Alpine, `apt-get install ffmpeg` on Debian).

**Upload of a large video returns 403.** The file is larger than `THUMBNAILS_VIDEO_MAX_INPUT_FILE_SIZE`. The file upload itself is **not** blocked — only the thumbnail request is — so the video is still stored and playable. Raise the cap if you want previews for larger files (e.g. `THUMBNAILS_VIDEO_MAX_INPUT_FILE_SIZE=2GB`).

**`init` panics on first boot.** Delete the `opencloud_opencloud-config` volume and try again. The `init` step is idempotent only when the config volume is empty.

---

Have fun. If you spot a bug specific to this fork, please open an issue at <https://github.com/enclave-projects/opencloud/issues>. For upstream OpenCloud bugs, please report them at <https://github.com/opencloud-eu/opencloud/issues>.
