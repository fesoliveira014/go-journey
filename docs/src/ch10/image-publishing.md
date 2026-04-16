# 10.5 Image Publishing & Versioning

Every push to `main` in this project builds five Docker images and pushes them to the GitHub Container Registry (GHCR). This section covers the tagging strategy, the registry itself, a line-by-line walkthrough of the `build-and-push` job, and the deliberate choice to use `docker/build-push-action` for publishing rather than Earthly.

---

## Image Tagging Strategy

A Docker image tag is just a name pointing to an image digest. Tags are mutable---you can move them at any time. This creates a fundamental tension: human-readable convenience versus deployment safety.

This project pushes two tags per image on every `main` build.

### `latest` — Mutable, Convenient, Dangerous in Production

```
ghcr.io/myorg/library/catalog:latest
```

`latest` always points to the most recent build. It is what you get when you `docker pull` without specifying a tag. This is convenient during local development — `docker compose pull` fetches the newest image without needing to know a specific identifier.

The problem is that `latest` is not reproducible. Pull on Monday, pull again on Wednesday after a new push, and you will be running different code under the same name. In production, a deployment controller with `imagePullPolicy: Always` may pull a different image than other replicas when a pod restarts — a support and debugging nightmare.

Use `latest` for: local development, demo environments, quick pull-and-run.

Avoid `latest` for: production deployments, Kubernetes manifests committed to version control.

### `sha-<commit>` — Immutable, Traceable, Production-Safe

```
ghcr.io/myorg/library/catalog:sha-a3f8c21b9d04e6f1c7b8d3a5e2f9c0d1b4a7e8f2
```

The `sha-<commit>` tag is constructed from `sha-` plus the full 40-character Git SHA (`${{ github.sha }}`). It is immutable: once pushed, the tag is never updated. Given a running container, you can:

1. Read the image tag from the container or the Kubernetes pod spec
2. Strip the `sha-` prefix to get the Git commit hash
3. Run `git show <hash>` to see the exact code that built the image

This is the tag to use in Kubernetes manifests committed to your GitOps repository. A code review on the manifest clearly shows which commit is being deployed. Rolling back is a `git revert` followed by applying the previous SHA tag.

### Semantic Versioning — Deferred

`v1.2.3` tags following [Semantic Versioning](https://semver.org/) are the standard for public releases. This requires a release process: tagging commits, generating changelogs, and triggering a separate release pipeline. That is out of scope for this chapter. When you reach the point of shipping software, add a release workflow that triggers on `git tag v*` and pushes the SemVer tag alongside the SHA tag.

---

## GHCR — GitHub Container Registry

GHCR is GitHub's built-in Docker registry, available at `ghcr.io`.[^1] Images live at:

```
ghcr.io/<github-owner>/<github-repo>/<service-name>:<tag>
```

For example, if the repo is `acme-corp/library`, the catalog image is:

```
ghcr.io/acme-corp/library/catalog:latest
```

### Authentication via `GITHUB_TOKEN`

Workflows authenticating with GHCR use the `GITHUB_TOKEN` secret, which GitHub creates automatically for every workflow run. No manual secret configuration is needed. The token is scoped to the repository that owns the workflow.

The permissions block at the top of `main.yml` explicitly grants write access to packages:

```yaml
permissions:
  contents: read
  packages: write
```

Without `packages: write`, the push would fail with a 403. The `contents: read` permission allows checking out the repository. Both are set at the minimum necessary level — this is the principle of least privilege applied to CI tokens.

### Package Visibility

New packages created via GHCR default to private. To make images publicly pullable (useful for open-source projects), navigate to the package settings on GitHub and change the visibility to Public. Public packages are still tied to the repository — they appear under the repository's "Packages" tab — but anyone can `docker pull` them without authentication.

---

## The `build-and-push` Job Walkthrough

The `build-and-push` job (shown in full in Section 10.3) runs on each merge to `main`. The key steps are summarized below; refer to the `main.yml` listing in the GitHub Actions section for the complete YAML.

### `needs: ci`

The job only runs after the `ci` job completes successfully. The `ci` job runs `earthly +ci`, which executes lint and tests for all services (see Sections 10.2 and 10.4). If any lint or test fails, `build-and-push` is skipped entirely. You never publish an image from broken code.

In GitHub Actions, `needs` creates a dependency edge in the job graph. Jobs without `needs` run in parallel. Jobs with `needs` wait for all listed jobs to succeed before starting.

### `strategy.matrix`

```yaml
    strategy:
      matrix:
        service: [auth, catalog, gateway, reservation, search]
```

GitHub Actions expands this matrix into five parallel jobs, one per service. Each job receives `matrix.service` as a variable available via `${{ matrix.service }}`. The five jobs start concurrently once `ci` passes, so total publish time is the maximum of the five individual build times, not their sum.

If one matrix job fails (for example, the `search` Dockerfile has a bug), GitHub Actions' default `fail-fast: true` behavior cancels any in-flight matrix jobs that have not yet completed. Jobs that had already finished still pushed their images. GitHub marks the overall `build-and-push` job as failed. Set `fail-fast: false` if you want every matrix job to run to completion regardless of failures.

### `docker/login-action`

```yaml
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
```

This action authenticates the Docker daemon running in the GHA runner against GHCR. `github.actor` is the username of whoever or whatever triggered the run (a human pusher or another automation). `GITHUB_TOKEN` is the auto-created workflow token. After this step, any `docker push` in subsequent steps will be authenticated.

### `docker/build-push-action`

```yaml
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: services/${{ matrix.service }}/Dockerfile
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:sha-${{ github.sha }}
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:latest
```

`context: .` sets the Docker build context to the repository root. It has to be, because the Dockerfiles `COPY` files from outside their service directory — specifically `gen/` (generated protobuf code) and `pkg/` (shared libraries). With a narrower context like `services/catalog/`, `COPY gen/ ./gen/` would fail with "file not found".

`file: services/${{ matrix.service }}/Dockerfile` points to each service's Dockerfile without changing the context root. This is the standard pattern for a monorepo with a shared build context.

`push: true` builds and pushes in one operation. Setting it to `false` would build locally without pushing, which is useful for testing the build step in a pull-request workflow.

`tags` lists two tags separated by a newline. Both are pushed in the same operation.

---

## Design Decision: Dockerfiles vs Earthly for Publishing

The `ci` job uses Earthly (`earthly +ci`) for lint and testing. The `build-and-push` job uses `docker/build-push-action`. This is intentional, and the reasoning matters.

### Why `docker/build-push-action` for Publishing

`docker/build-push-action` is built on [BuildKit](https://github.com/moby/buildkit) and integrates directly with the GHA runner environment.[^2] It provides:

**Build caching** — the action can export and import build cache from GHCR itself (or GitHub Actions cache). Subsequent builds reuse cached layers for unchanged stages, cutting build times significantly on large images.

**Provenance attestation** — by default, the action generates SLSA (Supply-chain Levels for Software Artifacts) provenance metadata and pushes it as an attestation alongside the image. This is a supply-chain security feature: it records what runner, what commit, and what workflow produced the image, cryptographically linked to the image digest.

**Multi-platform support** — combined with `docker/setup-qemu-action`, the same action can build `linux/amd64` and `linux/arm64` images in the same step and push a multi-platform manifest. This matters when deploying to ARM-based Kubernetes nodes (e.g., AWS Graviton).

**OIDC integration** — GHA runners support OpenID Connect tokens for keyless signing with Sigstore/Cosign. `docker/build-push-action` fits naturally into this ecosystem.

Earthly can push images with `earthly --push ./services/catalog+docker`, but it does not (as of early 2026) participate in GHA's built-in provenance and attestation pipeline. For production publishing, the Docker actions ecosystem is the more mature and feature-complete path.

### Why Earthly for Local Building

The Earthly `+docker` target in each service's Earthfile builds a minimal production image:

```earthfile
docker:
    FROM alpine:3.19
    COPY +build/catalog /usr/local/bin/catalog
    EXPOSE 50052
    ENTRYPOINT ["/usr/local/bin/catalog"]
    SAVE IMAGE catalog:latest
```

Running `earthly ./services/catalog+docker` locally produces a `catalog:latest` image you can load into Docker and test with Docker Compose. The build logic is identical to what the Dockerfile does, and it benefits from Earthly's layer caching for fast iteration.

The Earthly alternative for publishing is:

```bash
earthly --push ./services/catalog+docker --image ghcr.io/myorg/library/catalog:sha-abc123
```

This works, but it bypasses GHA's provenance features and requires manual secret injection for registry authentication. For a team project, the docker actions are a better fit.

### Summary: When to Use Which

| Scenario | Tool |
|---|---|
| Local development build | `earthly ./services/<svc>+docker` |
| Local smoke test against Docker Compose | `earthly ./services/<svc>+docker` |
| CI lint + test | `earthly +ci` |
| Publishing to GHCR | `docker/build-push-action` |
| Multi-platform release builds | `docker/build-push-action` + QEMU |

---

## JVM Comparison

If you have published artifacts to Maven Central, Artifactory, or Nexus, publishing Docker images to GHCR maps directly to that mental model.

**Registry = Artifact repository.** GHCR is to Docker images what Artifactory is to JARs. Both authenticate callers, store versioned artifacts (immutable in the case of release versions), and provide a pull endpoint for consumers.

**`sha-<commit>` tag = SNAPSHOT with commit hash.** In Maven, `-SNAPSHOT` artifacts are mutable — every deploy overwrites the previous one. Some teams append commit hashes to snapshot versions: `1.0.0-SNAPSHOT-a3f8c21`. The Docker `sha-<commit>` tag does the same, but with a stronger convention of immutability: you cannot overwrite an existing tag (when GHCR is configured to prevent it), and in practice no one does.

**`latest` tag = `-SNAPSHOT`.** Both are mutable, both are convenient for development, and both are dangerous in production for the same reason: you cannot pin a running system to a specific version without checking what "latest" resolved to at deployment time.

**Matrix builds = Gradle multi-module `publish`.** In a Gradle multi-module project, `./gradlew publish` publishes all submodules to the remote repository. The GHA matrix strategy is the CI equivalent: five parallel jobs, one per service, each publishing its artifact. The parallelism is explicit in the YAML rather than implicit in the build tool.

**`GITHUB_TOKEN` = CI machine credentials.** In JVM CI pipelines, you configure Artifactory credentials as environment variables or secrets (`ARTIFACTORY_USERNAME`, `ARTIFACTORY_PASSWORD`). `GITHUB_TOKEN` is the equivalent for GHCR, but GitHub manages the credential lifecycle automatically — you never create or rotate it.

---

## Exercises

1. **Inspect a published image.** After a `main` push, navigate to the GitHub repository's Packages page. Find the `catalog` package. Examine the tags listed. Click into the `sha-<hash>` tag and find the provenance attestation. What information does it record about the build?

2. **Add build caching.** Modify the `build-and-push` job to use GHA's built-in cache with `cache-from` and `cache-to` in `docker/build-push-action`. Refer to the action documentation.[^2] Push a change and compare the build time on the second run versus the first.

3. **Pin a deployment.** Write a minimal Kubernetes `Deployment` manifest for the `catalog` service that uses the `sha-<commit>` tag rather than `latest`. Set `imagePullPolicy: IfNotPresent`. Explain why `IfNotPresent` is appropriate with immutable tags but `Always` might be needed with `latest`.

4. **Add a release workflow.** Create a new file `.github/workflows/release.yml` that triggers on `push: tags: ['v*']`. It should run the same `build-and-push` logic but push a third tag derived from the Git tag name (e.g., `v1.2.3`). Use `github.ref_name` to extract the tag. Do not push the `latest` tag from the release workflow — that should remain the job of the `main` workflow.

---

## References

[^1]: [GitHub Container Registry documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry) — Official docs covering authentication, visibility settings, and package management for GHCR.
[^2]: [docker/build-push-action documentation](https://github.com/docker/build-push-action) — README and reference for all inputs, including `cache-from`, `cache-to`, `platforms`, and provenance options.
[^3]: [SLSA provenance with GitHub Actions](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/using-artifact-attestations-to-establish-provenance-for-builds) — GitHub's guide to build provenance and attestations, showing what `docker/build-push-action` generates automatically.
