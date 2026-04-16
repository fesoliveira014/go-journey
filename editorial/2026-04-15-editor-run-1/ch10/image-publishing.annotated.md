# 10.5 Image Publishing & Versioning

<!-- [STRUCTURAL] Progression: tagging strategy → GHCR → job walkthrough → design decision (Dockerfile vs Earthly) → JVM analogy → exercises. Strong. -->
<!-- [LINE EDIT] "Every push to `main` in this project builds five Docker images and pushes them to the GitHub Container Registry (GHCR)." — good opener. -->
Every push to `main` in this project builds five Docker images and pushes them to the GitHub Container Registry (GHCR). This section covers the tagging strategy, the registry itself, a line-by-line walkthrough of the `build-and-push` job, and the deliberate choice to use `docker/build-push-action` for publishing rather than Earthly.

---

## Image Tagging Strategy

<!-- [LINE EDIT] "A Docker image tag is just a name pointing to an image digest. Tags are mutable — you can move them at any time. This creates a fundamental tension: human-readable convenience versus deployment safety." — clean, punchy. -->
<!-- [COPY EDIT] "human-readable" — hyphenated compound adjective before noun (CMOS 7.81). Good. -->
A Docker image tag is just a name pointing to an image digest. Tags are mutable — you can move them at any time. This creates a fundamental tension: human-readable convenience versus deployment safety.

This project pushes two tags per image on every `main` build.

### `latest` — Mutable, Convenient, Dangerous in Production

```
ghcr.io/myorg/library/catalog:latest
```

<!-- [LINE EDIT] "`latest` always points to the most recent build. It is what you get when you `docker pull` without specifying a tag." — good. -->
`latest` always points to the most recent build. It is what you get when you `docker pull` without specifying a tag. This is convenient during local development — `docker compose pull` fetches the newest image without needing to know a specific identifier.

<!-- [LINE EDIT] "The problem is that `latest` is not reproducible. If you pull on Monday and a colleague pulls on Wednesday after a new push, you are running different code under the same tag. In production, if a deployment controller reads `imagePullPolicy: Always` and the pod restarts, it may pull a different image than the one running in other replicas. This is a support and debugging nightmare." (67 words) — split. "The problem is that `latest` is not reproducible. Pull on Monday, pull on Wednesday after a new push, and you are running different code under the same name. In production, a deployment controller with `imagePullPolicy: Always` may pull a different image than other replicas when a pod restarts — a support and debugging nightmare." -->
The problem is that `latest` is not reproducible. If you pull on Monday and a colleague pulls on Wednesday after a new push, you are running different code under the same tag. In production, if a deployment controller reads `imagePullPolicy: Always` and the pod restarts, it may pull a different image than the one running in other replicas. This is a support and debugging nightmare.

<!-- [COPY EDIT] "Use `latest` for: ... Avoid `latest` for: ..." — concise two-item framing. Good. -->
Use `latest` for: local development, demo environments, quick pull-and-run.

Avoid `latest` for: production deployments, Kubernetes manifests committed to version control.

### `sha-<commit>` — Immutable, Traceable, Production-Safe

```
ghcr.io/myorg/library/catalog:sha-a3f8c21b9d04e6f1c7b8d3a5e2f9c0d1b4a7e8f2
```

<!-- [LINE EDIT] "The `sha-<commit>` tag is constructed from `sha-` plus the full 40-character Git SHA (`${{ github.sha }}`). It is immutable: once pushed, the tag is never updated." — good. -->
<!-- [COPY EDIT] "40-character Git SHA" — hyphenated compound adjective + numeral; CMOS 9.13 (numerals with units). Good. -->
The `sha-<commit>` tag is constructed from `sha-` plus the full 40-character Git SHA (`${{ github.sha }}`). It is immutable: once pushed, the tag is never updated. Given a running container, you can:

1. Read the image tag from the container or the Kubernetes pod spec
2. Strip the `sha-` prefix to get the Git commit hash
3. Run `git show <hash>` to see the exact code that built the image

<!-- [STRUCTURAL] The numbered-list pattern for "given X, you can Y" is effective pedagogy. Keep. -->
<!-- [LINE EDIT] "This is the tag to use in Kubernetes manifests committed to your GitOps repository. A code review on the manifest clearly shows which commit is being deployed. Rollback is `git revert` followed by applying the previous SHA tag." — good. -->
<!-- [COPY EDIT] "GitOps" — product/methodology noun; capitalized as proper noun. Good. -->
This is the tag to use in Kubernetes manifests committed to your GitOps repository. A code review on the manifest clearly shows which commit is being deployed. Rollback is `git revert` followed by applying the previous SHA tag.

### Semantic Versioning — Deferred

<!-- [STRUCTURAL] Honest framing — the book doesn't implement SemVer release, and it says so. Good. -->
<!-- [LINE EDIT] "`v1.2.3` tags following [Semantic Versioning](https://semver.org/) are the standard for public releases. This requires a release process: tagging commits, generating changelogs, and triggering a separate release pipeline. That is out of scope for this chapter." — good. -->
<!-- [COPY EDIT] "Semantic Versioning" — capitalized as title of the spec. CMOS 8.2 (proper names of works/specs). Good. -->
`v1.2.3` tags following [Semantic Versioning](https://semver.org/) are the standard for public releases. This requires a release process: tagging commits, generating changelogs, and triggering a separate release pipeline. That is out of scope for this chapter. When you reach the point of shipping software, add a release workflow that triggers on `git tag v*` and pushes the SemVer tag alongside the SHA tag.

<!-- [COPY EDIT] "SemVer" — common abbreviation for Semantic Versioning; first use here. Consider defining on first use: "the SemVer tag (Semantic Versioning)". -->

---

## GHCR — GitHub Container Registry

<!-- [LINE EDIT] "GHCR is GitHub's built-in Docker registry, available at `ghcr.io`." — good. -->
GHCR is GitHub's built-in Docker registry, available at `ghcr.io`.[^1] Images live at:

```
ghcr.io/<github-owner>/<github-repo>/<service-name>:<tag>
```

For example, if the repo is `acme-corp/library`, the catalog image is:

```
ghcr.io/acme-corp/library/catalog:latest
```

### Authentication via `GITHUB_TOKEN`

<!-- [LINE EDIT] "Workflows authenticating with GHCR use the `GITHUB_TOKEN` secret, which GitHub creates automatically for every workflow run." — good. -->
Workflows authenticating with GHCR use the `GITHUB_TOKEN` secret, which GitHub creates automatically for every workflow run. No manual secret configuration is needed. The token is scoped to the repository that owns the workflow.

The permissions block at the top of `main.yml` explicitly grants write access to packages:

```yaml
permissions:
  contents: read
  packages: write
```

<!-- [STRUCTURAL] Some duplication with github-actions.md section on permissions. That's acceptable — each section should stand alone for readers jumping in. -->
<!-- [LINE EDIT] "Without `packages: write`, the push would fail with a 403. The `contents: read` permission allows checking out the repository. Both are set at the minimum necessary level — this is the principle of least privilege applied to CI tokens." — clean. -->
Without `packages: write`, the push would fail with a 403. The `contents: read` permission allows checking out the repository. Both are set at the minimum necessary level — this is the principle of least privilege applied to CI tokens.

### Package Visibility

<!-- [LINE EDIT] "New packages created via GHCR default to private. To make images publicly pullable (useful for open-source projects), navigate to the package settings on GitHub and change the visibility to Public." — good. -->
<!-- [COPY EDIT] "open-source" — hyphenated compound adjective (CMOS 7.81). Good. -->
New packages created via GHCR default to private. To make images publicly pullable (useful for open-source projects), navigate to the package settings on GitHub and change the visibility to Public. Public packages are still tied to the repository — they appear under the repository's "Packages" tab — but anyone can `docker pull` them without authentication.

---

## The `build-and-push` Job Walkthrough

<!-- [STRUCTURAL] Walkthrough duplicates the main.yml job already shown in 10.3. That's fine for a spec section, but consider cross-referencing rather than restating. Suggest opening with "This is the same job shown in Section 10.3; here we walk through the publishing-specific fields in depth." -->
Here is the full job from `.github/workflows/main.yml`:

<!-- [COPY EDIT] Please verify: Same action versions as 10.3 (checkout@v4, login-action@v3, build-push-action@v6). Consistency confirmed with that file. -->
```yaml
  build-and-push:
    needs: ci
    runs-on: ubuntu-latest
    strategy:
      matrix:
        service: [auth, catalog, gateway, reservation, search]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: services/${{ matrix.service }}/Dockerfile
          push: true
          tags: |
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:sha-${{ github.sha }}
            ghcr.io/${{ github.repository }}/${{ matrix.service }}:latest
```

### `needs: ci`

<!-- [LINE EDIT] "The job only runs after the `ci` job completes successfully. The `ci` job runs `earthly +ci`, which executes lint and tests for all services (see Section 10.4). If any lint or test fails, `build-and-push` is skipped entirely. You never publish an image from broken code." — good. -->
<!-- [COPY EDIT] "(see Section 10.4)" — lint is covered in 10.4, but the GHA workflow is covered in 10.3. The `+ci` earthly target is defined in 10.2. The forward reference is to the linter chapter, not the workflow. Consider: "(see Sections 10.2 and 10.4)." -->
The job only runs after the `ci` job completes successfully. The `ci` job runs `earthly +ci`, which executes lint and tests for all services (see Section 10.4). If any lint or test fails, `build-and-push` is skipped entirely. You never publish an image from broken code.

In GitHub Actions, `needs` creates a dependency edge in the job graph. Jobs without `needs` run in parallel. Jobs with `needs` wait for all listed jobs to succeed before starting.

### `strategy.matrix`

```yaml
    strategy:
      matrix:
        service: [auth, catalog, gateway, reservation, search]
```

<!-- [LINE EDIT] "GitHub Actions expands this matrix into five parallel jobs, one per service. Each job receives `matrix.service` as a variable available via `${{ matrix.service }}`. The five jobs start concurrently once `ci` passes, so total publish time is the maximum of the five individual build times, not their sum." (45 words) — good. -->
GitHub Actions expands this matrix into five parallel jobs, one per service. Each job receives `matrix.service` as a variable available via `${{ matrix.service }}`. The five jobs start concurrently once `ci` passes, so total publish time is the maximum of the five individual build times, not their sum.

<!-- [LINE EDIT] "If one matrix job fails (e.g., the `search` Dockerfile has a bug), the other four continue. GitHub marks the overall `build-and-push` job as failed when any matrix job fails, but the other images are still pushed. This is the default `fail-fast: true` behavior — GHA will cancel remaining matrix jobs if one fails. You can set `fail-fast: false` if you want all matrix jobs to always run to completion regardless." (75 words) — contradiction. First sentence says "the other four continue", which describes `fail-fast: false` behavior. Third sentence correctly says `fail-fast: true` *cancels* remaining jobs. These contradict. Please verify and rewrite. -->
<!-- [FINAL] This is a factual contradiction within one paragraph. The default is `fail-fast: true`, which cancels in-flight matrix jobs when any fails. The correct text should state that `fail-fast: true` cancels the others; `fail-fast: false` lets them run to completion. Rewrite suggestion: "If one matrix job fails (e.g., the `search` Dockerfile has a bug), the default `fail-fast: true` behavior cancels any in-flight matrix jobs that have not completed. GitHub marks the overall `build-and-push` job as failed. Jobs that had already finished successfully have still pushed their images. If you want all five to run to completion regardless of failures, set `fail-fast: false`." -->
If one matrix job fails (e.g., the `search` Dockerfile has a bug), the other four continue. GitHub marks the overall `build-and-push` job as failed when any matrix job fails, but the other images are still pushed. This is the default `fail-fast: true` behavior — GHA will cancel remaining matrix jobs if one fails. You can set `fail-fast: false` if you want all matrix jobs to always run to completion regardless.

### `docker/login-action`

```yaml
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
```

<!-- [LINE EDIT] "This action authenticates the Docker daemon running in the GHA runner against GHCR." — good. -->
<!-- [COPY EDIT] "`github.actor` is the username of whoever or whatever triggered the run (a human pusher or another automation)." — clear. -->
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

<!-- [LINE EDIT] "`context: .` sets the Docker build context to the repository root. This is required because the Dockerfiles COPY files from directories outside their own service directory — specifically `gen/` (generated protobuf code) and `pkg/` (shared libraries). If the context were set to `services/catalog/`, the `COPY gen/ ./gen/` instruction would fail with a 'file not found' error." (60 words) — split: "`context: .` sets the Docker build context to the repository root. It has to be, because the Dockerfiles `COPY` files from outside their service directory — specifically `gen/` (generated protobuf code) and `pkg/` (shared libraries). With a narrower context like `services/catalog/`, `COPY gen/ ./gen/` would fail with 'file not found'." -->
`context: .` sets the Docker build context to the repository root. This is required because the Dockerfiles COPY files from directories outside their own service directory — specifically `gen/` (generated protobuf code) and `pkg/` (shared libraries). If the context were set to `services/catalog/`, the `COPY gen/ ./gen/` instruction would fail with a "file not found" error.

<!-- [STRUCTURAL] The chapter has made a strong case for Earthly throughout, but here the publishing step is Dockerfile-based. Explain earlier (at start of the chapter) that publishing uses Dockerfiles for supply-chain features and reference this section. -->
<!-- [LINE EDIT] "`file: services/${{ matrix.service }}/Dockerfile` points to each service's Dockerfile without changing the context root. This is the standard pattern for a monorepo with a shared build context." — good. -->
`file: services/${{ matrix.service }}/Dockerfile` points to each service's Dockerfile without changing the context root. This is the standard pattern for a monorepo with a shared build context.

<!-- [LINE EDIT] "`push: true` builds and pushes in one operation. Setting it to `false` would build locally without pushing, which is useful for testing the build step in a pull-request workflow." — good. -->
`push: true` builds and pushes in one operation. Setting it to `false` would build locally without pushing, which is useful for testing the build step in a pull-request workflow.

<!-- [COPY EDIT] "pull-request" — hyphenated compound adjective before noun (CMOS 7.81). Good. -->
`tags` lists two tags separated by a newline. Both are pushed in the same operation.

---

## Design Decision: Dockerfiles vs Earthly for Publishing

<!-- [STRUCTURAL] This is the most important section. Answers the implicit reader question. Strong teaching moment. -->
<!-- [COPY EDIT] "Dockerfiles vs Earthly" — "vs" without period per CMOS 10.42 (some styles; "vs." with period is more common in the U.S. tradition). Earlier in chapter "vs." with period is used. Be consistent. Recommend "vs." (with period). -->
The `ci` job uses Earthly (`earthly +ci`) for lint and testing. The `build-and-push` job uses `docker/build-push-action`. This is intentional, and the reasoning matters.

### Why `docker/build-push-action` for Publishing

<!-- [LINE EDIT] "`docker/build-push-action` is built on [BuildKit](https://github.com/moby/buildkit) and integrates directly with the GHA runner environment." — good. -->
`docker/build-push-action` is built on [BuildKit](https://github.com/moby/buildkit) and integrates directly with the GHA runner environment.[^2] It provides:

<!-- [STRUCTURAL] Four-point list (Build caching, Provenance, Multi-platform, OIDC) cleanly lays out the trade-offs. Keep. -->
<!-- [COPY EDIT] "**Build caching**" — bolded feature heading, clean. -->
**Build caching** — the action can export and import build cache from GHCR itself (or GitHub Actions cache). Subsequent builds reuse cached layers for unchanged stages, cutting build times significantly on large images.

<!-- [COPY EDIT] "SLSA" — acronym; introduced on first use. CMOS 10.3: spell out on first use. Suggest: "SLSA (Supply-chain Levels for Software Artifacts)". -->
**Provenance attestation** — by default, the action generates SLSA provenance metadata and pushes it as an attestation alongside the image. This is a supply-chain security feature: it records what runner, what commit, and what workflow produced the image, cryptographically linked to the image digest.

<!-- [COPY EDIT] "AWS Graviton" — proper noun, correct capitalization. -->
<!-- [LINE EDIT] "combined with `docker/setup-qemu-action`, the same action can build `linux/amd64` and `linux/arm64` images in the same step and push a multi-platform manifest. This matters when deploying to ARM-based Kubernetes nodes (e.g., AWS Graviton)." — good. -->
**Multi-platform support** — combined with `docker/setup-qemu-action`, the same action can build `linux/amd64` and `linux/arm64` images in the same step and push a multi-platform manifest. This matters when deploying to ARM-based Kubernetes nodes (e.g., AWS Graviton).

<!-- [COPY EDIT] "OIDC (OpenID Connect)" — acronym glossed. Good. "Sigstore/Cosign" — Cosign is part of Sigstore project; slash formulation OK. -->
**OIDC integration** — GHA runners support OpenID Connect tokens for keyless signing with Sigstore/Cosign. `docker/build-push-action` fits naturally into this ecosystem.

<!-- [LINE EDIT] "Earthly can push images with `earthly --push ./services/catalog+docker`, but it does not (as of writing) participate in GHA's built-in provenance and attestation pipeline. For production publishing, the Docker actions ecosystem is the standard and more feature-complete path." — good. -->
<!-- [COPY EDIT] "(as of writing)" — typical disclaimer for a technical book. Consider an explicit date stamp: "(as of early 2026)" or "(as of Earthly 0.8.x)". -->
Earthly can push images with `earthly --push ./services/catalog+docker`, but it does not (as of writing) participate in GHA's built-in provenance and attestation pipeline. For production publishing, the Docker actions ecosystem is the standard and more feature-complete path.

### Why Earthly for Local Building

<!-- [LINE EDIT] "The Earthly `+docker` target in each service's Earthfile builds a minimal production image:" — good. -->
The Earthly `+docker` target in each service's Earthfile builds a minimal production image:

<!-- [COPY EDIT] Please verify: `alpine:3.19` tag (also used in earthly.md). Confirm current at 2026-04-15. -->
```earthfile
docker:
    FROM alpine:3.19
    COPY +build/catalog /usr/local/bin/catalog
    EXPOSE 50052
    ENTRYPOINT ["/usr/local/bin/catalog"]
    SAVE IMAGE catalog:latest
```

Running `earthly ./services/catalog+docker` locally produces a `catalog:latest` image you can load into Docker and test with `docker compose`. The build logic is identical to what the Dockerfile does, and it benefits from Earthly's layer caching for fast iteration.

<!-- [STRUCTURAL] Claim "The build logic is identical to what the Dockerfile does" — this is an assertion but the Dockerfiles are not shown in the book. Consider: show one Dockerfile or drop the "identical" claim in favor of "similar". -->
The Earthly alternative for publishing is:

```bash
earthly --push ./services/catalog+docker --image ghcr.io/myorg/library/catalog:sha-abc123
```

<!-- [COPY EDIT] Please verify: Earthly `--image` flag syntax for overriding SAVE IMAGE target name at push time. Confirm correct in Earthly 0.8.x. -->
This works, but it bypasses GHA's provenance features and requires manual secret injection for registry authentication. For a team project, the docker actions are a better fit.

### Summary: When to Use Which

<!-- [STRUCTURAL] Summary table: excellent wrap for this section. -->
| Scenario | Tool |
|---|---|
| Local development build | `earthly ./services/<svc>+docker` |
| Local smoke test against docker compose | `earthly ./services/<svc>+docker` |
| CI lint + test | `earthly +ci` |
| Publishing to GHCR | `docker/build-push-action` |
| Multi-platform release builds | `docker/build-push-action` + QEMU |

<!-- [COPY EDIT] "docker compose" — canonical Docker convention writes "Docker Compose" for the product, "`docker compose`" (lowercase, in code) for the CLI command. Distinguish. -->

---

## JVM Comparison

<!-- [STRUCTURAL] JVM analogy section continues the book's pattern of bridging to the target reader's background. Four bold-leading paragraphs — good structure. -->
If you have published artifacts to Maven Central, Artifactory, or Nexus, publishing Docker images to GHCR maps directly to that mental model.

<!-- [LINE EDIT] "**Registry = Artifact repository.** GHCR is to Docker images what Artifactory is to JARs. Both authenticate callers, store immutable versioned artifacts, and provide a pull endpoint for consumers." — good. -->
<!-- [COPY EDIT] "immutable versioned artifacts" — Maven does support mutable -SNAPSHOT artifacts, and GHCR tags are mutable by default. Claim of "immutable" needs nuance. Suggest: "Both authenticate callers, store versioned artifacts (immutable in the case of release versions), and provide a pull endpoint." -->
**Registry = Artifact repository.** GHCR is to Docker images what Artifactory is to JARs. Both authenticate callers, store immutable versioned artifacts, and provide a pull endpoint for consumers.

<!-- [LINE EDIT] "**`sha-<commit>` tag = SNAPSHOT with commit hash.** In Maven, `-SNAPSHOT` artifacts are mutable — every deploy overwrites the previous one. Some teams append commit hashes to snapshot versions: `1.0.0-SNAPSHOT-a3f8c21`. The `sha-<commit>` Docker tag does the same thing but enforces immutability: you cannot overwrite an existing tag if you configure GHCR to prevent it, and in practice no one does." (66 words) — split: "**`sha-<commit>` tag = SNAPSHOT with commit hash.** In Maven, `-SNAPSHOT` artifacts are mutable — every deploy overwrites the previous one. Some teams append commit hashes to snapshot versions, e.g., `1.0.0-SNAPSHOT-a3f8c21`. The Docker `sha-<commit>` tag does the same thing but enforces immutability: you cannot overwrite an existing tag (if GHCR is configured to prevent it), and in practice no one does." -->
**`sha-<commit>` tag = SNAPSHOT with commit hash.** In Maven, `-SNAPSHOT` artifacts are mutable — every deploy overwrites the previous one. Some teams append commit hashes to snapshot versions: `1.0.0-SNAPSHOT-a3f8c21`. The `sha-<commit>` Docker tag does the same thing but enforces immutability: you cannot overwrite an existing tag if you configure GHCR to prevent it, and in practice no one does.

<!-- [COPY EDIT] Text above says "enforces immutability: you cannot overwrite an existing tag **if you configure GHCR to prevent it**". This is a conditional immutability — not enforcement by default. The "enforces" verb is too strong. Recommend: "encourages immutability in convention (and can enforce it with GHCR's package settings)". -->
**`latest` tag = `-SNAPSHOT`.** Both are mutable, both are convenient for development, and both are dangerous in production for the same reason: you cannot pin a running system to a specific version without checking what "latest" resolved to at deployment time.

<!-- [LINE EDIT] "**Matrix builds = Gradle multi-module `publishAll`.** In a Gradle multi-module project, `./gradlew publishAll` publishes all submodules to the remote repository. The GHA matrix strategy is the CI equivalent: five parallel jobs, one per service, each publishing its artifact. The parallelism is explicit in the YAML rather than implicit in the build tool." — good. -->
**Matrix builds = Gradle multi-module `publishAll`.** In a Gradle multi-module project, `./gradlew publishAll` publishes all submodules to the remote repository. The GHA matrix strategy is the CI equivalent: five parallel jobs, one per service, each publishing its artifact. The parallelism is explicit in the YAML rather than implicit in the build tool.

<!-- [COPY EDIT] "`publishAll`" — verify that `publishAll` is a canonical Gradle task name. Gradle's Maven Publish plugin generates `publish`, `publishToMavenLocal`, and per-publication tasks; `publishAll` is an aggregate task users typically define themselves. Consider: "`./gradlew publish` (which triggers all `publishXxxPublicationToYyyRepository` tasks)". -->
<!-- [LINE EDIT] "**`GITHUB_TOKEN` = CI machine credentials.** In JVM CI pipelines, you configure Artifactory credentials as environment variables or secrets (`ARTIFACTORY_USERNAME`, `ARTIFACTORY_PASSWORD`). `GITHUB_TOKEN` is the equivalent for GHCR, but GitHub manages the credential lifecycle automatically — you never create or rotate it." — good. -->
**`GITHUB_TOKEN` = CI machine credentials.** In JVM CI pipelines, you configure Artifactory credentials as environment variables or secrets (`ARTIFACTORY_USERNAME`, `ARTIFACTORY_PASSWORD`). `GITHUB_TOKEN` is the equivalent for GHCR, but GitHub manages the credential lifecycle automatically — you never create or rotate it.

---

## Exercises

<!-- [STRUCTURAL] Four exercises: inspect image, add cache, pin deployment, release workflow. Excellent progression from observation to construction. -->

1. **Inspect a published image.** After a `main` push, navigate to the GitHub repository's Packages page. Find the `catalog` package. Examine the tags listed. Click into the `sha-<hash>` tag and find the provenance attestation. What information does it record about the build?

<!-- [COPY EDIT] "Modify the `build-and-push` job to use GHA's built-in cache with `cache-from` and `cache-to` in `docker/build-push-action`. Refer to the action documentation.[^2]" — good. -->
2. **Add build caching.** Modify the `build-and-push` job to use GHA's built-in cache with `cache-from` and `cache-to` in `docker/build-push-action`. Refer to the action documentation.[^2] Push a change and compare the build time on the second run versus the first.

<!-- [LINE EDIT] "Write a minimal Kubernetes `Deployment` manifest for the `catalog` service that uses the `sha-<commit>` tag rather than `latest`. Set `imagePullPolicy: IfNotPresent`. Explain why `IfNotPresent` is appropriate with immutable tags but `Always` might be needed with `latest`." — good. -->
3. **Pin a deployment.** Write a minimal Kubernetes `Deployment` manifest for the `catalog` service that uses the `sha-<commit>` tag rather than `latest`. Set `imagePullPolicy: IfNotPresent`. Explain why `IfNotPresent` is appropriate with immutable tags but `Always` might be needed with `latest`.

<!-- [LINE EDIT] "Create a new file `.github/workflows/release.yml` that triggers on `push: tags: ['v*']`. It should run the same `build-and-push` logic but push a third tag derived from the Git tag name (e.g., `v1.2.3`). Use `github.ref_name` to extract the tag. Do not push the `latest` tag from the release workflow — that should remain the job of the `main` workflow." — good. -->
4. **Add a release workflow.** Create a new file `.github/workflows/release.yml` that triggers on `push: tags: ['v*']`. It should run the same `build-and-push` logic but push a third tag derived from the Git tag name (e.g., `v1.2.3`). Use `github.ref_name` to extract the tag. Do not push the `latest` tag from the release workflow — that should remain the job of the `main` workflow.

---

## References

<!-- [COPY EDIT] Please verify URLs: GitHub Container Registry docs; docker/build-push-action README; SLSA provenance with GitHub Actions. All likely valid but GitHub docs URLs have shifted since 2024. -->
[^1]: [GitHub Container Registry documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry) — Official docs covering authentication, visibility settings, and package management for GHCR.
[^2]: [docker/build-push-action documentation](https://github.com/docker/build-push-action) — README and reference for all inputs, including `cache-from`, `cache-to`, `platforms`, and provenance options.
[^3]: [SLSA provenance with GitHub Actions](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/using-artifact-attestations-to-establish-provenance-for-builds) — GitHub's guide to build provenance and attestations, showing what `docker/build-push-action` generates automatically.
