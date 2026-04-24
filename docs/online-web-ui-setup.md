# Running the Web UI online

This guide covers what you need to host the **Google Maps Scraper web UI** on a server (VPS, home lab, or PaaS with a long‑running process and enough RAM for Chromium). It is **not** the multi‑tenant SaaS stack; for that, see [SaaS Edition](saas.md).

---

## What you are running

- A single Go process that serves the **HTML UI + REST API** and **runs scrape jobs in-process** using **Playwright (Chromium)**.
- **Job metadata and live results** go to either:
  - **Local SQLite** inside your configured data directory (default), or  
  - **Turso (libsql)** if you set both Turso environment variables (optional).
- Per-job **CSV files** are still written under the same data directory.

The binary does **not** automatically load a `.env` file. Set environment variables in your process manager, container, or shell.

---

## Server requirements

| Requirement | Notes |
|-------------|--------|
| **RAM** | Prefer **2 GB minimum**, **4 GB+** if you run larger jobs or email extraction. Chromium is memory-heavy. |
| **CPU** | 2+ vCPUs recommended for smoother Playwright runs. |
| **Disk** | Enough space for SQLite/CSVs; Turso reduces reliance on local DB size but CSVs still grow. |
| **OS** | Linux x86_64 or arm64 with Playwright/Chromium dependencies (Docker image bundles these). |
| **Network** | Outbound HTTPS to Google Maps; inbound HTTP(S) on your chosen port for the UI. |

**Playwright:** First run may download browser assets unless your image or install step already includes them (the published Docker image does).

---

## 1. Create a Turso database (optional)

Turso is **optional**. Skip this section to use **local SQLite only** (simplest for a single server).

1. Sign up at [Turso](https://turso.tech/) and install the [Turso CLI](https://docs.turso.tech/cli/introduction) if you use it.
2. Create a database and note:
   - **Database URL** — must use the `libsql://` scheme (example: `libsql://your-db-yourorg.turso.io`).
   - **Auth token** — create an API token with access to that database.

3. Set **both** variables in the environment of the scraper process:

```bash
export TURSO_DATABASE_URL='libsql://your-db-yourorg.turso.io'
export TURSO_AUTH_TOKEN='your-token-here'
```

If either variable is missing or empty, the app uses **local SQLite** at `{data-folder}/jobs.db`.

---

## 2. Data directory and listen address

| Mechanism | Purpose |
|-----------|---------|
| **`-data-folder`** | Directory for `jobs.db`, per-job CSVs, and related files. **Must be persistent** (mounted volume on Docker/VPS). Default: `webdata`. |
| **`-addr`** | Listen address. Default `:8080` (all interfaces). For a specific interface: e.g. `127.0.0.1:8080` behind a reverse proxy. |

Example (bind all interfaces on port 8080):

```bash
./google-maps-scraper -web -data-folder /var/lib/gmapsdata -addr :8080
```

`-web` is explicit; if you start with **no** `-input` and **no** `-dsn`, the tool also selects web mode automatically.

---

## 3. Run with Docker (typical VPS)

Match the published image flow from the main README: mount a **writable** data volume and publish the port.

```bash
mkdir -p gmapsdata

docker run -d --name gmaps-scraper --restart unless-stopped \
  -v "$PWD/gmapsdata:/gmapsdata" \
  -p 8080:8080 \
  -e TURSO_DATABASE_URL='libsql://...' \
  -e TURSO_AUTH_TOKEN='...' \
  gosom/google-maps-scraper \
  -data-folder /gmapsdata \
  -addr :8080
```

Then open `http://YOUR_SERVER_IP:8080` (or your domain after TLS, below).

**Optional environment variables:**

| Variable | Effect |
|----------|--------|
| `TURSO_DATABASE_URL` + `TURSO_AUTH_TOKEN` | Use Turso instead of local SQLite for the web datastore. |
| `DISABLE_TELEMETRY=1` | Disable anonymous telemetry (see project README). |
| `SCRAPER_WEB_STATIC_DIR` | Absolute path to a checkout’s `web/static` folder to serve templates/CSS from disk (dev/custom branding; restart required). |

---

## 4. Run from source (build on the server)

```bash
git clone https://github.com/gosom/google-maps-scraper.git
cd google-maps-scraper
go mod download
go build -o google-maps-scraper .
# Install Playwright browsers for your OS (see project README / Playwright docs)
./google-maps-scraper -web -data-folder ./gmapsdata -addr :8080
```

If you use Turso, export `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN` before starting the same binary.

---

## 5. Put it on the public internet safely

The OSS web UI **does not ship with login or API keys**. Anyone who can reach the port can create jobs and access data.

**Recommended:**

1. **Do not** expose `:8080` directly to the world without protection.
2. Put **Caddy** or **nginx** (or a cloud load balancer) in front with:
   - **TLS** (Let’s Encrypt).
   - **HTTP basic auth**, **OAuth2 proxy**, **VPN**, or **IP allowlist** so only you (or your team) can reach it.
3. Bind the app to **localhost** only and let the proxy talk to it:

```bash
./google-maps-scraper -web -data-folder /var/lib/gmapsdata -addr 127.0.0.1:8080
```

Proxy `https://maps.example.com` → `http://127.0.0.1:8080`.

4. Open only needed ports in the firewall (e.g. 443 from anywhere; 8080 not public).

---

## 6. API and live results (after deployment)

With the web server running, useful endpoints include:

| URL | Description |
|-----|-------------|
| `/` | Web UI |
| `/api/docs` | OpenAPI / Redoc |
| `GET /api/v1/jobs` | List jobs |
| `GET /api/v1/jobs/{id}/results?since=...` | Incremental JSON results (polling) |
| `GET /api/v1/jobs/{id}/stats` | Job stats |
| `GET /api/v1/jobs/{id}/download` | CSV download |

Use **HTTPS** in production so tokens and job data are not sent in clear text.

---

## 7. Checklist before you call it “done”

- [ ] Persistent volume (or disk path) for **`-data-folder`**.
- [ ] **`-addr`** matches how you reverse-proxy (often `127.0.0.1:8080`).
- [ ] **TLS + access control** in front of the app if it is reachable from the internet.
- [ ] Turso: **both** env vars set **or** both unset (local SQLite).
- [ ] Enough **RAM** for Chromium; watch OOM kills on small instances.
- [ ] **Outbound** connectivity to Google allowed from the host.

---

## 8. Troubleshooting

| Symptom | Things to check |
|---------|-------------------|
| Container exits immediately | Logs: Playwright/browser deps, disk permissions on data folder. |
| “Using Turso” but connection errors | URL scheme `libsql://`, token not expired, outbound TLS to Turso allowed. |
| UI works but scrapes fail | RAM, proxies (see [proxies.md](proxies.md)), Google blocking datacenter IPs. |
| macOS Docker | See `MacOS instructions.md` in the repo root if the stock Docker command misbehaves. |

---

## Related docs

- Main README: [Web UI](../README.md#web-ui), [REST API](../README.md#rest-api), installation.
- Multi-user platform: [SaaS Edition](saas.md).
- Proxies for scale or blocking: [proxies.md](proxies.md).
