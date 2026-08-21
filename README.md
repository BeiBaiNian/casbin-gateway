<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">An open-source gateway for the AI coding agents and the web traffic on your machine, developed by Go and React.</h3>
<p align="center">
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml">
    <img alt="Lint" src="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml/badge.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/build.yml">
    <img alt="Build" src="https://github.com/apache/casbin-gateway/actions/workflows/build.yml/badge.svg">
  </a>
  <a href="https://pkg.go.dev/github.com/apache/casbin-gateway">
    <img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/apache/casbin-gateway.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/releases/latest">
    <img alt="GitHub Release" src="https://img.shields.io/github/v/release/apache/casbin-gateway.svg">
  </a>
</p>

<p align="center">
  <a href="https://github.com/apache/casbin-gateway/blob/master/LICENSE">
    <img alt="license" src="https://img.shields.io/github/license/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/issues">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/stargazers">
    <img alt="GitHub stars" src="https://img.shields.io/github/stars/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/network">
    <img alt="GitHub forks" src="https://img.shields.io/github/forks/apache/casbin-gateway">
  </a>
  <a href="https://discord.gg/S5UjpzGZjN">
    <img alt="Discord" src="https://img.shields.io/discord/1022748306096537660?logo=discord&label=discord&color=5865F2">
  </a>
</p>

<p align="center">
  <b>English</b> | <a href="./README_zh.md">中文</a>
</p>

## Run it

One command. No database, no Go, no Node, no configuration.

On Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
```

On Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
```

Either one downloads the build for this machine, unpacks it into `~/.local/share/casbin-gateway` (`%LOCALAPPDATA%\casbin-gateway` on Windows), puts a `casbin-gateway` command on your PATH, and starts it. Then open:

**http://localhost:17000** — sign in as `admin` with the password `123`, and change it from **My Account**.

That is the whole installation. Gateway keeps its data in a SQLite file inside its own directory, and signs you in against its own user table.

### What to do next

| Page | What you get | What it needs |
| --- | --- | --- |
| **Agents** | Every AI coding agent installed on this machine — Claude Code, Codex CLI, Cursor and more. Click **Patch** on one and its activity streams into the page live. | Nothing |
| **Skills & MCP** | Every skill and MCP server of every agent in one table. Add an MCP server to one agent or to several at once, open one, delete it, or copy it into another agent. | Nothing |
| **Channels** | One endpoint in front of your model vendors. Gateway holds the API key, so the agents never have it. | A vendor API key |
| **LLM Records** | Every request an agent relayed: the full system prompt, every message and tool call, the schema of every tool the model was offered, plus tokens and cost. | A channel, and `llmRecordMode` — see [Recording prompts](#recording-prompts) |
| **Advanced → Sites** | The reverse-proxy WAF: per-site routing, rules, certificates and analytics. | Turning the proxy on — see [Turning the WAF proxy on](#turning-the-waf-proxy-on) |

Agents are found by reading the user accounts, home directories and install paths of **the machine Gateway runs on**, so run it on the machine whose agents you want to watch.

### Send an agent's traffic through Gateway

This is what fills **LLM Records**, and what lets Gateway keep the vendor key instead of the agent.

1. **Channels** → **Add**: pick the type (OpenAI- or Anthropic-compatible), paste the vendor base URL and API key, and list the models it serves.
2. **Agents** → open an agent → pick that channel.
3. Copy the environment snippet the page shows, and start the agent from a shell that has it:

```bash
export ANTHROPIC_BASE_URL="http://localhost:17000/v1/agents/claude-code"
export ANTHROPIC_AUTH_TOKEN="casbin-gateway"
```

The token is a placeholder — the agent refuses to start without one, and Gateway authenticates upstream with the channel's own key.

### Stopping, upgrading, removing

- **Stop**: `Ctrl-C`. **Start again**: `casbin-gateway`, from any directory — the command is a wrapper that always starts Gateway in its install directory, where its data lives.
- **Upgrade**: run the install command again. Your database and settings are untouched.
- **Remove**: delete `~/.local/share/casbin-gateway` and `~/.local/bin/casbin-gateway` (on Windows, `%LOCALAPPDATA%\casbin-gateway` and its PATH entry).

Set `INSTALL_DIR` to install somewhere else, or `NO_START=1` to install without starting.

**These are nightly builds**, rebuilt from `master` on every push and published as the [`nightly`](https://github.com/apache/casbin-gateway/releases/tag/nightly) pre-release. They exist so that Gateway can be tried without a Go and Node toolchain; anything else should be built from a source release.

### Running in Docker or Podman

**A container cannot see the agents on your machine.** Agents are discovered by reading the home directories and install paths of the machine Gateway runs on, and inside a container that is the container's own filesystem. **Agents**, **Skills & MCP** and agent monitoring therefore stay empty there, and the pages say so rather than pretending nothing is installed. Everything that does not depend on the host works normally: **Channels**, **LLM Records** and the reverse-proxy WAF.

So run the one-command install above on the machine whose agents you want to watch, and use a container when Gateway is only a model endpoint or a reverse proxy for other machines.

No image is published, so the compose file builds one from a checkout of this repository:

```bash
docker compose up -d
```

Podman reads the same file:

```bash
podman compose up -d
```

Either way the UI is on http://localhost:17000, the SQLite database lives in a named volume that survives `down`, and `conf/app.conf` is mounted from the repository, so settings can be changed and the container restarted without rebuilding it.

To serve the WAF proxy from a container, set `gatewayEnabled = true` in that mounted `conf/app.conf` and publish its ports too, by adding them next to `17000:17000` in `docker-compose.yml`:

```yaml
    ports:
      - "17000:17000"
      - "8080:80"
      - "8443:443"
```

## Configuration

Everything is optional. Settings live in `conf/app.conf`, next to the executable, and each one is explained in the file itself. The ones people actually change:

| Setting | Default | What it does |
| --- | --- | --- |
| `httpport` | `17000` | Port of the web UI and the REST API |
| `driverName` / `dataSourceName` | `sqlite` / `./data/casbin-gateway.db` | Where data is stored |
| `gatewayEnabled` | `false` | Turns the reverse-proxy WAF on |
| `gatewayHttpPort` / `gatewayHttpsPort` | `80` / `443` | Ports the proxy listens on |
| `llmRecordMode` | `off` | How much of each relayed LLM request is kept |
| `apiKeyEncryptionKey` | empty | Encrypts channel API keys at rest (AES-256-GCM) |
| `casdoorEndpoint` | empty | Switches sign-in over to [Casdoor](https://casdoor.org) SSO |

Gateway prints what it is actually doing when it starts, so the result can be checked instead of the file:

```
+----------------------------------------------------------------------------+
| Casbin Gateway                                                              |
+----------------+-----------------------------------------------------------+
| Management UI  | http://localhost:17000                                     |
| Settings       | conf/app.conf                                              |
| Web UI files   | web/build                                                  |
| Reverse proxy  | enabled                                                    |
| Gateway HTTP   | :8080                                                      |
| Gateway HTTPS  | :8443                                                      |
| Database       | sqlite, file "./data/casbin-gateway.db" (connected)        |
| Sign-in        | built-in user table, Casdoor is not configured             |
| App dir        | ./data/apps                                                |
+----------------+-----------------------------------------------------------+
```

If a port is taken, Gateway says which process holds it and stops, rather than starting half-configured.

### Recording prompts

Nothing about a relayed request is stored until you ask for it, because a prompt can carry anything that was pasted into it:

```ini
; "off" keeps nothing, "metadata" records who called which model with which
; outcome, "full" also stores the request body — which is what LLM Records needs
; to show prompts, messages and tool schemas.
llmRecordMode = "full"
llmRecordRetentionDays = 30
llmRecordMaxRecords = 10000
llmRecordMaxPayloadBytes = 1048576
```

Bodies are sanitized before they are stored: anything that looks like a credential is replaced, and the number of replacements is shown with the record. Request headers, which is where the inbound API key is, never reach a record at all. A body over `llmRecordMaxPayloadBytes` keeps its structure and loses only its longest strings, so a large conversation is still listed message by message.

The cost next to each record uses built-in list prices, which vendors change and resellers do not follow. Point `llmPricingFile` at a JSON file of your own rates to correct them.

### Turning the WAF proxy on

The reverse proxy is off by default, so installing Gateway does not take over ports 80 and 443. To use it:

1. **Advanced → Sites → Add**. Set **Domain** to the hostname clients will use (`test.example.com`), **Host** and **Port** to where the traffic goes (`127.0.0.1` and `8000`), and **Mode** to `HTTP` — `HTTPS Only`, the default, redirects plain HTTP away before it reaches the backend.
2. Set `gatewayEnabled = true` in `conf/app.conf` and restart. Ports 80 and 443 need root on Linux and macOS, so for a first try set `gatewayHttpPort = 8080`.
3. Start anything on the backend port, e.g. `python -m http.server 8000`, then ask for the site by `Host` header — the gateway routes on it, so no DNS or `hosts` entry is needed:

```bash
curl -H "Host: test.example.com" http://127.0.0.1:8080/
```

You should get your backend's response. A `site not found for host` reply means the request reached Gateway but no site matches that `Host` value.

### Connecting Casdoor

[Casdoor](https://casdoor.org) is optional and takes over member management. Create an organization and an application for Gateway in a Casdoor instance, then fill in `casdoorEndpoint`, `clientId`, `clientSecret`, `casdoorOrganization` and `casdoorApplication`. Sign-in redirects to Casdoor as soon as `casdoorEndpoint` is set, which also enables [OAuth logins](https://casdoor.org/docs/provider/oauth/overview), health-check alerts, the `CAPTCHA` rule action, per-site SSO and cloud file storage.

## Development

### Prerequisites

Go 1.20+, and Node.js with Yarn.

### Run from source

The backend serves the compiled frontend out of `web/build`, so build it once first:

```bash
cd web && yarn install && yarn build
```

```bash
go run main.go
```

Then open http://localhost:17000 and sign in as `admin` with the password `123`, same as an installed Gateway. The SQLite database is created on first start; there is no database server to install.

### Frontend development

```bash
cd web && yarn dev
```

That serves the UI on http://localhost:16002 with hot reload and proxies API calls to the backend on port 17000, so both have to be running.

### Using MySQL instead of SQLite

XORM is used, so every database it supports works. Point Gateway at your server and it creates `dbName` on first start if it does not exist:

```ini
driverName = mysql
dataSourceName = root:123@tcp(localhost:3306)/
dbName = casbin_gateway
```

### Building a single binary

Gateway normally reads three things from disk: `conf/app.conf`, the compiled UI in `web/build`, and the IP location database `ip/17monipdb.dat`. The `embed` build tag bakes all three into the executable, which is what the install scripts ship:

```bash
cd web && yarn install && yarn build
```

```bash
go build -tags embed -o casbin-gateway .
```

Build the frontend first — everything under `web/build` goes into the binary, so `go build -tags embed` fails to compile while that directory is missing.

Files on disk always win over the embedded copies, so a single binary can still be configured and developed against without rebuilding it:

| Embedded asset | Overridden by |
| --- | --- |
| `conf/app.conf` | `conf/app.conf` in the working directory, or next to the executable |
| `web/build` | `web/build/index.html` in the working directory, which then serves the whole UI |
| `ip/17monipdb.dat` | `ip/17monipdb.dat` in the working directory |

The startup summary reports which source each one came from.

### Where the data goes

Being self-contained is about startup, not about staying read-only. A running Gateway writes `./data` (the SQLite database, deployed apps, agent patch state), `./logs` and `./tmp` relative to its working directory — which is why the installed `casbin-gateway` command is a wrapper that always starts it in its install directory. Running the executable directly from somewhere else gives you a second, empty installation there.

## Architecture

Casbin Gateway contains 2 parts:

| Name     | Description                            | Language               | Source code                                              |
|----------|----------------------------------------|------------------------|----------------------------------------------------------|
| Frontend | Web frontend UI for Casbin Gateway     | TypeScript + React + shadcn/ui | https://github.com/apache/casbin-gateway/tree/master/web |
| Backend  | RESTful API backend for Casbin Gateway | Golang + Beego + XORM  | https://github.com/apache/casbin-gateway                 |

## Online demo

https://ai.casbin.com

## Documentation

https://caswaf.org

## Contribute

If you have any questions, open an issue, or start a pull request directly — though we recommend opening an issue first to talk it through with the community.

## License

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
