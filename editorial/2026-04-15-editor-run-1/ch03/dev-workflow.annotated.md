# 3.4 Development Workflow

<!-- [STRUCTURAL] Arc: motivate (rebuild pain) → override pattern → Air tool intro → .air.toml → Dockerfile.dev → volume mounts → full command → debugging → exercise → summary. Good. -->
<!-- [STRUCTURAL] Consider a small sequence-of-events visual earlier (before introducing Air) so the reader knows what "hot reload" will look like mechanically. The sequence diagram exists but sits near the end. Moving it after "How Air Works" might cement the mental model earlier. Author's call. -->
<!-- [STRUCTURAL] "When to Rebuild vs. Restart" table is gold — don't lose it in a future rev. -->

The production Compose stack from the previous section builds optimized images and runs compiled binaries. That is correct for deployment, but painful for development: every code change requires rebuilding the Docker image and restarting the container. In this section, we set up a development workflow where code changes are automatically detected and rebuilt inside the running container.
<!-- [LINE EDIT] "That is correct for deployment, but painful for development" → "Correct for deployment, painful for development." — shorter, punchier. -->
<!-- [LINE EDIT] "are automatically detected and rebuilt" — passive. "...where your code changes trigger automatic rebuilds inside the running container." -->
<!-- [COPY EDIT] Replace `--` with em dash `—` throughout per CMOS 6.85. -->

---

## The Dev Override File Pattern

<!-- [STRUCTURAL] Clear heading; explanation of Compose file merging is precisely the scaffolding readers need. -->
Docker Compose supports **file merging**. When you pass multiple `-f` flags, Compose deep-merges the YAML files in order. The second file overrides matching keys from the first:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

This lets you keep the production Compose file clean and layer development-specific changes on top. You don't duplicate the entire service definition -- you only specify what changes.
<!-- [COPY EDIT] "development-specific" — hyphenated compound adjective before "changes." Correct (CMOS 7.81). -->

Here is `deploy/docker-compose.dev.yml`:

```yaml
services:
  catalog:
    build:
      context: ..
      dockerfile: services/catalog/Dockerfile.dev
    volumes:
      - ../services/catalog:/app/services/catalog
      - ../gen:/app/gen

  gateway:
    build:
      context: ..
      dockerfile: services/gateway/Dockerfile.dev
    volumes:
      - ../services/gateway:/app/services/gateway
```

This override changes two things per service:

1. **`dockerfile`** -- switches from the production Dockerfile to a development variant (e.g., `Dockerfile.dev`)
<!-- [COPY EDIT] "e.g.," — correct with following comma (CMOS 6.43). -->
2. **`volumes`** -- mounts your local source directory into the container

Everything else (environment variables, ports, networks, depends_on, healthchecks) is inherited from the base `docker-compose.yml`. You don't repeat it.
<!-- [LINE EDIT] "Everything else ... is inherited from the base" — passive. Rewrite: "Everything else (environment variables, ports, networks, `depends_on`, healthchecks) comes from the base `docker-compose.yml`." -->
<!-- [COPY EDIT] "depends_on" in prose — format as inline code, consistent with how `environment`, `ports`, `networks` would be formatted if they were. Currently only `healthchecks` is unbacked; keep consistent. -->

---

## Air for Hot-Reload

<!-- [COPY EDIT] Heading "Air for Hot-Reload" — compound adjective "hot-reload" is hyphenated before noun; "Hot-Reload" uppercase follows heading case (CMOS 8.159). OK. -->
Go compiles to a static binary. Unlike Python or Node.js, where the runtime reads source files directly, changing a `.go` file does nothing until you recompile. **Air** is a live-reload tool for Go that watches for file changes, rebuilds the binary, and restarts the process automatically.
<!-- [LINE EDIT] "changing a `.go` file does nothing until you recompile" — good, vivid. Keep. -->
<!-- [COPY EDIT] "Python or Node.js" — product names correct. -->
<!-- [COPY EDIT] "live-reload tool" — hyphenated compound adjective before noun. Correct (CMOS 7.81). -->

### How Air Works

<!-- [STRUCTURAL] Numbered list is good step-by-step. Consider making step 5 ("Repeat") less of an orphan — fold into step 4: "Kill the old process and start the new binary; then wait for the next change." -->
1. Air watches the directory for changes to files matching configured extensions (`.go` in our case)
2. When a change is detected, it waits for a configurable delay (to batch rapid saves)
3. It runs `go build` to compile the binary
4. It kills the old process and starts the new binary
5. Repeat
<!-- [COPY EDIT] Bullet parallelism: items 1, 3, 4 start with subject ("Air" / "It"); item 2 begins "When..."; item 5 is a one-word imperative. Consider reworking for parallelism (all subject-verb). -->

### The `.air.toml` Configuration

Both services use the same Air configuration. Here is `services/catalog/.air.toml`:

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/"
  bin = "./tmp/main"
  delay = 1000
  exclude_dir = ["tmp", "vendor"]
  include_ext = ["go"]
  kill_delay = "0s"

[log]
  time = false

[misc]
  clean_on_exit = true
```

Key settings:

- **`cmd`** -- the build command. Same as what you would run manually: `go build -o ./tmp/main ./cmd/`.
- **`bin`** -- path to the compiled binary that Air should execute.
- **`delay`** -- milliseconds to wait after detecting a change before rebuilding. 1000ms debounces rapid multi-file saves.
<!-- [COPY EDIT] "1000ms" — space before unit; CMOS 9.16: "1,000 ms" or "1000 ms". Number with unit. -->
- **`include_ext`** -- only watch `.go` files. Changes to `.md`, `.toml`, or other files are ignored.
- **`exclude_dir`** -- ignore the `tmp` directory (where the binary is written) and `vendor` to avoid infinite rebuild loops.
- **`clean_on_exit`** -- delete the `tmp` directory when Air stops.
<!-- [STRUCTURAL] Nice: readers get both "what" and "why" per setting. -->

### The Dev Dockerfile

Here is `services/catalog/Dockerfile.dev`:

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Disable workspace mode — same reason as production
ENV GOWORK=off

# Copy shared modules and service source
COPY gen/ ./gen/
COPY services/catalog/ ./services/catalog/

WORKDIR /app/services/catalog
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```
<!-- [STRUCTURAL] The Dockerfile.dev copies only gen/ and services/catalog/, but the production Dockerfile copies pkg/auth and pkg/otel too. If the Catalog service imports those (it does per 3.2), the dev image won't build. Query author: is Dockerfile.dev correct as shown, or is this an error in the manuscript? -->
<!-- [COPY EDIT] Please verify: The `Dockerfile.dev` shown omits `pkg/auth` and `pkg/otel`, yet the production Dockerfile copies them. If the Catalog service imports these modules, the dev build will fail with missing imports when the bind mount hasn't taken effect at `go mod download` time. Confirm the file content matches the real file in the repo. -->
<!-- [COPY EDIT] Please verify: `github.com/air-verse/air@latest` — the Air repo was renamed from `cosmtrek/air` to `air-verse/air` (2024). Confirm canonical module path. -->

Key differences from the production Dockerfile:

- **No multi-stage build.** We need the Go toolchain at runtime because Air calls `go build` on every change.
- **Air is installed** with `go install`. This adds the `air` binary to the Go toolchain's bin directory.
<!-- [LINE EDIT] "Air is installed" — passive. "Air installs via `go install`." Minor. -->
- **`CMD` instead of `ENTRYPOINT`.** `CMD` is easier to override if you want to debug something (e.g., `docker compose exec catalog sh`).
<!-- [COPY EDIT] "e.g., ..." — correct comma per CMOS 6.43. -->
- **No `CGO_ENABLED=0`.** The development build doesn't need to be fully static since the container already has the necessary libraries.
<!-- [LINE EDIT] "since the container already has the necessary libraries" — "already" mild filler; "since the container has the necessary libraries" reads as cleanly. Keep or cut. -->

The Gateway's `Dockerfile.dev` follows the same pattern, minus the `GOWORK=off` and `gen/` copy:
<!-- [COPY EDIT] Factual check: the Gateway's `Dockerfile.dev` shown below *does not* have `GOWORK=off` — confirmed. But it also doesn't copy `gen/`, `pkg/auth/`, or `pkg/otel/`. If the Gateway imports these (per 3.2, it does), the dev build will also break. Query author. -->
<!-- [COPY EDIT] Please verify: Gateway Dockerfile.dev as shown omits all shared-module copies. Flagging for consistency with the production Dockerfile shown in 3.2. -->

```dockerfile
FROM golang:1.26-alpine

RUN go install github.com/air-verse/air@latest

WORKDIR /app
COPY services/gateway/ ./services/gateway/

WORKDIR /app/services/gateway
RUN go mod download

CMD ["air", "-c", ".air.toml"]
```

---

## Volume Mounts and How They Enable Hot-Reload

The magic is in the volume mounts from `docker-compose.dev.yml`:
<!-- [LINE EDIT] "The magic is in" → "The key is in" — tutor voice, more precise. Optional. -->

```yaml
volumes:
  - ../services/catalog:/app/services/catalog
  - ../gen:/app/gen
```

A **bind mount** maps a host directory to a container directory. The container sees your local filesystem in real time -- when you save a file on your host, the change is immediately visible inside the container. Air detects the change and triggers a rebuild.

Without the volume mount, the container would only have the source code that was `COPY`ed during the image build. Changes on your host would not be reflected.
<!-- [LINE EDIT] "would only have" → "would have only"; more idiomatic placement of "only." Minor. -->
<!-- [LINE EDIT] "would not be reflected" — passive; "would not appear in the container" active. -->

```mermaid
sequenceDiagram
    participant You as Your Editor
    participant Host as Host Filesystem
    participant Mount as Volume Mount
    participant Air as Air (in container)
    participant Go as go build (in container)
    participant Svc as Service Process

    You->>Host: Save handler.go
    Host->>Mount: File change propagated
    Mount->>Air: inotify event detected
    Air->>Air: Wait 1000ms (debounce)
    Air->>Go: go build -o ./tmp/main ./cmd/
    Go-->>Air: Build complete
    Air->>Svc: Kill old process
    Air->>Svc: Start new binary
    Svc-->>Air: Running on :50052
```
<!-- [COPY EDIT] "inotify" — Linux kernel API; lowercase. Correct. On macOS Docker uses a different file-event bridge through its virtualization layer; worth a footnote for Mac users, especially given the WSL2 note later. -->

---

## The Full Dev Command

From the `deploy/` directory:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

This:
1. Merges the base and dev Compose files
2. Builds dev images (with Air installed)
3. Starts PostgreSQL (with healthcheck)
4. Waits for PostgreSQL to be healthy
5. Starts catalog and gateway with volume mounts and Air

You will see Air's output in the logs:
<!-- [LINE EDIT] "You will see" → "Air's output appears in the logs:" — slightly more active. -->

```
catalog-1  | running...
catalog-1  | watching .
catalog-1  | building...
catalog-1  | running ./tmp/main
```

When you edit a `.go` file in `services/catalog/`, Air detects it and rebuilds automatically.

---

## Debugging Tips

### Viewing Logs

```bash
# All services
docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

# Specific service
docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f catalog
```

The `-f` flag follows the log stream (like `tail -f`). Without it, you see a snapshot.

### Executing Commands in a Running Container

```bash
# Open a shell in the catalog container
docker compose -f docker-compose.yml -f docker-compose.dev.yml exec catalog sh

# Run a one-off command
docker compose -f docker-compose.yml -f docker-compose.dev.yml exec postgres-catalog psql -U postgres -d catalog
```

`exec` runs a command in an already-running container. This is useful for:
- Inspecting the filesystem inside the container
- Running database queries directly
- Checking environment variables (`env | grep DATABASE`)
- Testing network connectivity (`ping postgres-catalog`)
<!-- [COPY EDIT] Bullet parallelism: all start with gerunds (Inspecting, Running, Checking, Testing). Good (CMOS 6.130). -->

### Inspecting Networks

```bash
# List networks
docker network ls

# Inspect the bridge network
docker network inspect deploy_library-net
```

The network name is prefixed with the Compose project name (derived from the directory name, `deploy`). The inspect command shows all connected containers and their IP addresses.

### Port Conflicts

If you see "port is already allocated," another process on your host is using the same port. Common culprits:

- A local PostgreSQL installation on port 5433
<!-- [COPY EDIT] Inconsistency check: earlier chapter content notes port 5432 is the default the reader might already have locally, and 5433 is the chosen host port. A "local PostgreSQL installation on port 5433" is unusual — readers usually have PostgreSQL on 5432. Did you mean: "A local PostgreSQL installation on port 5432 (conflicting only if you change GATEWAY/CATALOG mapping)" or "another service using port 5433"? Consider rephrasing for accuracy. -->
- A previous Compose stack that wasn't fully stopped
- Another development server on port 8080

Solutions:
1. Change the port in `deploy/.env` (e.g., `GATEWAY_PORT=8081`)
2. Stop the conflicting process
3. Run `docker compose down` to clean up stale containers
<!-- [COPY EDIT] Numbered-list parallelism: imperatives ("Change," "Stop," "Run"). Good. -->

### When to Rebuild vs. Restart

| Situation | Action |
|---|---|
| Changed Go source code | Nothing -- Air handles it |
| Changed `go.mod` (new dependency) | `docker compose up --build` (rebuild the image) |
| Changed `Dockerfile.dev` | `docker compose up --build` |
| Changed `docker-compose.yml` or `.dev.yml` | `docker compose up` (re-reads config) |
| Changed `.air.toml` | `docker compose restart catalog` (Air re-reads config on start) |
| Database needs resetting | `docker compose down -v && docker compose up --build` |
<!-- [STRUCTURAL] This table is one of the most useful artifacts in the chapter. Keep. -->

The key insight: volume-mounted source changes are instant (Air catches them). But dependency or configuration changes require rebuilding or restarting because they affect the image or container setup, not just the mounted files.
<!-- [COPY EDIT] "volume-mounted" — hyphenated compound adjective. Correct (CMOS 7.81). -->

---

## Exercise: Watch Air in Action

1. Start the dev stack:
   ```bash
   cd deploy
   docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
   ```

2. Wait for all services to start. You should see Air's "running" messages for both catalog and gateway.

3. In your editor, open `services/gateway/cmd/main.go` (or whichever file contains an HTTP handler). Add a new endpoint or modify the response of an existing one -- for example, change the health check response body.
<!-- [COPY EDIT] "health check" vs. "healthcheck" — chapter uses "healthcheck" (one word) for the Docker concept and "health check response body" here. Consistency check: as an HTTP endpoint concept, "health check" (open) is standard (Google SRE book), while "healthcheck" is specifically the Compose YAML key. Either defensible; flag for editorial consistency. -->

4. Watch the terminal. Within ~2 seconds, you should see Air detect the change, rebuild, and restart the service.
<!-- [COPY EDIT] "~2 seconds" — numeral for time, OK (CMOS 9.16). -->

5. Test the modified endpoint with `curl` to confirm the change is live:
   ```bash
   curl http://localhost:8080/healthz
   ```

6. Try editing `services/catalog/` source code and observe Air rebuild that service independently.

<details>
<summary>Solution</summary>

After saving the file, the Compose logs show something like:

```
gateway-1  | services/gateway/cmd/main.go has changed
gateway-1  | building...
gateway-1  | running ./tmp/main
```

The rebuild typically takes 1-3 seconds for a small service. The curl request to `localhost:8080/healthz` returns the modified response immediately.
<!-- [COPY EDIT] "1-3 seconds" — en dash for ranges (CMOS 6.78): "1–3 seconds". -->

If you don't see Air detecting the change:
- Verify the volume mount is working: `docker compose exec gateway ls /app/services/gateway/cmd/` -- the file should show your latest modification timestamp.
- Check that the file extension is `.go` -- Air only watches extensions listed in `include_ext`.
- On some systems (notably Docker Desktop with WSL2), file change notification can be delayed. Try saving twice or waiting a few seconds.
<!-- [STRUCTURAL] The WSL2 caveat is user-relevant (the reader's memory profile indicates WSL2 usage). Keep. -->
<!-- [COPY EDIT] "WSL2" — Microsoft style is "WSL 2" (with space). Verify project style; either is commonly seen. -->

If the build fails (you introduced a syntax error), Air reports the error in the logs and keeps running. Fix the error, save again, and Air retries the build.

</details>

---

## Summary

- The dev override pattern layers development-specific configuration (dev Dockerfiles, volume mounts) on top of the production Compose file, avoiding duplication.
- Air watches for Go source file changes and automatically rebuilds and restarts the service binary.
- Bind mounts map your host filesystem into the container, enabling real-time code synchronization.
- Use `docker compose logs -f` and `docker compose exec` for debugging.
- Source code changes are handled by Air automatically. Dependency, Dockerfile, or Compose config changes require a rebuild or restart.
<!-- [LINE EDIT] "are handled by Air automatically" — passive. "Air handles source-code changes automatically." -->

---

## References

[^1]: [Air -- Live reload for Go apps](https://github.com/air-verse/air) -- Air's GitHub repository with configuration documentation.
<!-- [COPY EDIT] Please verify: `github.com/air-verse/air` (renamed from cosmtrek/air). -->
[^2]: [Docker Compose file merging](https://docs.docker.com/compose/how-tos/multiple-compose-files/merge/) -- how multiple Compose files are merged.
<!-- [COPY EDIT] Please verify URL. -->
[^3]: [Bind mounts](https://docs.docker.com/engine/storage/bind-mounts/) -- Docker documentation on host-to-container file mounting.
<!-- [COPY EDIT] Please verify URL. -->
[^4]: [Docker Compose CLI reference](https://docs.docker.com/reference/cli/docker/compose/) -- complete command reference for `docker compose`.
<!-- [COPY EDIT] Please verify URL. -->
<!-- [FINAL] Cold read: no typos or doubled words. Main open issues: (a) the two Dockerfile.dev snippets appear to omit pkg/auth and pkg/otel which the production Dockerfiles require — possible manuscript bug; (b) "health check" vs. "healthcheck" consistency. -->
