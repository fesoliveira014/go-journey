# 10.3 GitHub Actions Workflows

<!-- [STRUCTURAL] Opening locks the Earthly-vs-GHA responsibility division clearly. Progression: concepts → marketplace → PR workflow → main workflow → why two workflows → JVM comparisons → exercises. Strong. -->
<!-- [LINE EDIT] "It answers one question: *when* should something run, and with what environment?" — crisp. Keep. -->
GitHub Actions is the orchestration layer of our pipeline. It answers one question: *when* should something run, and with what environment? The build logic itself lives in the Earthfile (section 10.2). GitHub Actions triggers that logic at the right moment, supplies secrets, and runs parallel jobs across services.

<!-- [COPY EDIT] "section 10.2" — capitalize "Section" for cross-references (consistency with 10.5). CMOS 8.179 permits either; internal consistency matters. -->
This section walks through the two workflow files in this project, explains every line, and shows what the equivalent pure-GHA approach would look like so you can understand the trade-off.

<!-- [COPY EDIT] "pure-GHA approach" — compound adjective, hyphenated before noun (CMOS 7.81). Good. -->
<!-- [COPY EDIT] "trade-off" — hyphenated; consistent with CMOS and Merriam-Webster. -->

---

## GitHub Actions Concepts

<!-- [LINE EDIT] "GitHub detects these files automatically -- no registration step, no webhook configuration. Any file in that directory matching the `on:` trigger criteria runs on GitHub's infrastructure." — good two-beat rhythm. -->
A **workflow** is a YAML file placed in `.github/workflows/`. GitHub detects these files automatically -- no registration step, no webhook configuration. Any file in that directory matching the `on:` trigger criteria runs on GitHub's infrastructure.[^1]

The key vocabulary:

<!-- [STRUCTURAL] Vocabulary table is useful. Format columns are consistent. -->
| Term | Meaning |
|---|---|
<!-- [COPY EDIT] "(e.g., `pr.yml`, `main.yml`)" — "e.g.," with comma per CMOS 6.43. Good. -->
| **Workflow** | A YAML file defining one automated process (e.g., `pr.yml`, `main.yml`) |
| **Trigger** (`on:`) | The event that starts the workflow: `push`, `pull_request`, `schedule`, `workflow_dispatch` |
| **Job** | A group of steps that runs on a single runner VM. Jobs run in parallel by default |
| **Step** | A single shell command or Action within a job |
| **Runner** | The VM that executes a job. `ubuntu-latest` is GitHub's managed Ubuntu image, reset to a clean state on every run |
| **Action** | A reusable step published to the Actions Marketplace (e.g., `actions/checkout@v4`) |
<!-- [COPY EDIT] "`github.sha`, `github.repository`, `github.actor`, `secrets.*`" — serial comma used correctly in lists separated by commas. Good. -->
| **Context** | Variables injected by GitHub at runtime: `github.sha`, `github.repository`, `github.actor`, `secrets.*` |

<!-- [LINE EDIT] "If you come from Jenkins, a workflow maps to a Jenkinsfile, a job maps to a Jenkins `stage`, and a step maps to a `sh` block. If you come from TeamCity, a workflow maps to a Build Configuration." — good parallel structure. -->
If you come from Jenkins, a workflow maps to a Jenkinsfile, a job maps to a Jenkins `stage`, and a step maps to a `sh` block. If you come from TeamCity, a workflow maps to a Build Configuration. The structure is similar; the execution model is cloud-managed rather than self-hosted.

<!-- [COPY EDIT] "cloud-managed" — hyphenated compound adjective (CMOS 7.81). Good. -->

---

## The Actions Marketplace

<!-- [LINE EDIT] "Instead of writing shell commands to install a tool or authenticate with a registry, you reference a published Action:" — good. -->
Actions are reusable steps. Instead of writing shell commands to install a tool or authenticate with a registry, you reference a published Action:

```yaml
- uses: actions/checkout@v4
```

<!-- [COPY EDIT] "here, `github.com/actions/checkout` at tag `v4`" — parenthetical commas; CMOS 6.28. Good. -->
The `uses:` key pulls code from a GitHub repository (here, `github.com/actions/checkout` at tag `v4`) and runs it as a step. Anyone can publish an Action; the official `actions/*` namespace is maintained by GitHub.

The Actions we use in this project:

<!-- [STRUCTURAL] Table of Actions is useful. Consider footnoting the Marketplace URLs. -->
| Action | Publisher | Purpose |
|---|---|---|
| `actions/checkout@v4` | GitHub | Clone the repository into the runner workspace |
| `earthly/actions-setup@v1` | Earthly | Install the `earthly` binary at a specific version |
| `docker/login-action@v3` | Docker | Authenticate with a container registry |
| `docker/build-push-action@v6` | Docker | Build a Docker image and push it to a registry |

<!-- [COPY EDIT] Please verify: action versions are current at 2026-04-15. `actions/checkout@v4` (v4 released 2023-09; v5 may be available). `earthly/actions-setup@v1`, `docker/login-action@v3`, `docker/build-push-action@v6` — verify all latest major tags. -->
<!-- [LINE EDIT] "If you come from Gradle, think of Actions as Gradle plugins: community-maintained, versioned, and composable. If you come from Jenkins, they are the equivalent of Jenkins plugins, but without a plugin manager UI -- you declare them directly in the workflow file." — good. -->
If you come from Gradle, think of Actions as Gradle plugins: community-maintained, versioned, and composable. If you come from Jenkins, they are the equivalent of Jenkins plugins, but without a plugin manager UI -- you declare them directly in the workflow file.

<!-- [STRUCTURAL] "Always pin Actions to a version tag or commit SHA" — strong advice, but the prior text doesn't say which is safer. Briefly note: "For maximum supply-chain safety, pin to a full commit SHA; tags can be moved." -->
The `@v4` suffix pins the Action to a specific tag. Omitting it or using `@main` would pull the latest version on every run, which is a reproducibility risk. Always pin Actions to a version tag or commit SHA.

---

## PR Workflow

<!-- [LINE EDIT] "The PR workflow runs whenever a pull request targets the `main` branch. Its only job is validation: does this change compile, lint, and test cleanly?" — good. -->
<!-- [COPY EDIT] "compile, lint, and test cleanly" — serial comma (CMOS 6.19). Good. -->
The PR workflow runs whenever a pull request targets the `main` branch. Its only job is validation: does this change compile, lint, and test cleanly?

<!-- [COPY EDIT] Please verify: Earthly version `v0.8.15` is the current stable release at 2026-04-15. -->
```yaml
name: PR Check
on:
  pull_request:
    branches: [main]

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Earthly
        uses: earthly/actions-setup@v1
        with:
          version: v0.8.15
      - name: Run CI
        run: earthly +ci
```

Walking through each part:

<!-- [COPY EDIT] Inline YAML field like **`name: PR Check`** — consistent bold+code treatment. Good. -->
**`name: PR Check`** — The display name shown in the GitHub UI on the pull request's checks tab.

<!-- [COPY EDIT] "on: pull_request: branches: [main]" — collapsed YAML in inline code OK; though reading as a single key chain is awkward. Leave as is for brevity. -->
**`on: pull_request: branches: [main]`** — Fires when a PR is opened, updated (new commit pushed), or reopened, but only if the PR targets `main`. A PR targeting a feature branch does not trigger this workflow.

<!-- [LINE EDIT] "`ubuntu-latest` is GitHub's managed Ubuntu image. It includes Docker, Git, and common system tools. You do not need to install Go -- the Earthfile manages the Go version inside a container." — good. -->
**`runs-on: ubuntu-latest`** — Every job needs a runner. `ubuntu-latest` is GitHub's managed Ubuntu image. It includes Docker, Git, and common system tools. You do not need to install Go -- the Earthfile manages the Go version inside a container.

**`actions/checkout@v4`** — The runner starts with an empty workspace. This step clones your repository. Without it, subsequent steps have no source code.

<!-- [LINE EDIT] "`earthly/actions-setup@v1` with `version: v0.8.15`" — good. -->
**`earthly/actions-setup@v1` with `version: v0.8.15`** — Installs the `earthly` binary. Pinning the version ensures the same Earthly version runs locally and in CI. If you omit the `version:` field, you get whatever the Action considers "latest", which may differ from your local install.

<!-- [COPY EDIT] "'latest'" — straight quotes inside straight quotes; CMOS 6.9 uses curly quotes in prose but tech manuscripts commonly use straight. Consistent throughout the file. Acceptable. -->
<!-- [LINE EDIT] "All the build logic is there; the workflow does not know or care what `+ci` does internally." — good. -->
**`run: earthly +ci`** — Calls the `+ci` target defined in the Earthfile. That target runs lint and tests. All the build logic is there; the workflow does not know or care what `+ci` does internally.

### Why a Separate Workflow for PRs?

<!-- [STRUCTURAL] Good justification. This answers the natural reader question "why two files?". -->
<!-- [LINE EDIT] "If you ran the full pipeline (including pushing Docker images) on every PR update, you would push dozens of images for in-progress work -- most of them from branches that never merge." — 33 words; keep. -->
PRs need validation but not publishing. If you ran the full pipeline (including pushing Docker images) on every PR update, you would push dozens of images for in-progress work -- most of them from branches that never merge. Container registries charge for storage; image tags accumulate noise. Separating the workflows keeps the registry clean: only commits that land on `main` produce published images.

### Pure GitHub Actions Alternative

<!-- [STRUCTURAL] Blockquote "Trade-off: Earthly vs. native GHA steps" is an effective side-panel pattern. Keep. -->
If you did not use Earthly, the same CI job would look like this:

> **Trade-off: Earthly vs. native GHA steps**
>
> ```yaml
> steps:
>   - uses: actions/checkout@v4
>   - uses: actions/setup-go@v5
>     with:
>       go-version: '1.26'
>   - uses: golangci/golangci-lint-action@v6
>   - run: go test ./...
> ```
>
<!-- [COPY EDIT] Please verify: `actions/setup-go@v5`, `golangci/golangci-lint-action@v6` are current versions. `go-version: '1.26'` consistency with the rest of the chapter. -->
<!-- [LINE EDIT] "This is simpler to read and has fewer moving parts. The downside: it only works in CI. You cannot run `actions/setup-go` locally. If lint fails in CI, you reproduce it by pushing another commit and waiting for a runner -- there is no `earthly +lint` equivalent on your terminal. The Earthly approach trades a small amount of workflow complexity for full local reproducibility." (65 words) — split: "This is simpler to read and has fewer moving parts. The downside: it only works in CI. You cannot run `actions/setup-go` on your laptop. If lint fails in CI, you reproduce it by pushing another commit and waiting for a runner. The Earthly approach trades a little workflow complexity for full local reproducibility." -->
> This is simpler to read and has fewer moving parts. The downside: it only works in CI. You cannot run `actions/setup-go` locally. If lint fails in CI, you reproduce it by pushing another commit and waiting for a runner -- there is no `earthly +lint` equivalent on your terminal. The Earthly approach trades a small amount of workflow complexity for full local reproducibility.

---

## Main Workflow

<!-- [LINE EDIT] "The main workflow runs when a commit is pushed directly to `main` -- which in practice means after a PR merges." — good. -->
The main workflow runs when a commit is pushed directly to `main` -- which in practice means after a PR merges. It runs the same CI checks, then builds and pushes Docker images for all five services.

<!-- [COPY EDIT] Please verify: `docker/build-push-action@v6` is current major tag. v5 and v6 both exist; confirm v6 is GA at 2026-04-15. -->
```yaml
name: CI/CD
on:
  push:
    branches: [main]

permissions:
  contents: read
  packages: write

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install Earthly
        uses: earthly/actions-setup@v1
        with:
          version: v0.8.15
      - name: Run CI
        run: earthly +ci

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

<!-- [STRUCTURAL] Walkthrough structure matches PR workflow section — good parallel. -->

### Trigger

**`on: push: branches: [main]`** — Fires only on direct pushes to `main`. Opening a PR against `main` does not trigger this; it triggers `pr.yml` instead. This is intentional: you want exactly one workflow responsible for publishing images.

### Permissions

```yaml
permissions:
  contents: read
  packages: write
```

<!-- [LINE EDIT] "By default, `GITHUB_TOKEN` (the automatically provisioned token for each workflow run) has broad permissions." — good. -->
<!-- [COPY EDIT] "automatically provisioned" — OK (parenthetical). Not hyphenated because "provisioned" is verbal participle, not compound adjective. -->
By default, `GITHUB_TOKEN` (the automatically provisioned token for each workflow run) has broad permissions. Declaring permissions explicitly follows the principle of least privilege: this workflow needs to read the repository (`contents: read`) and push to the GitHub Container Registry (`packages: write`). Nothing else. If an attacker compromises a step in this workflow, the token cannot create releases, modify secrets, or push to other registries.[^2]

<!-- [COPY EDIT] "GitHub Container Registry" — product name; introduced with acronym "GHCR" in index.md and image-publishing.md. Consistency check: this section does not gloss GHCR. Consider adding: "...the GitHub Container Registry (GHCR)". -->
<!-- [LINE EDIT] "`packages: write` is specifically required for GHCR (GitHub Container Registry, `ghcr.io`). Without it, the `docker/login-action` step authenticates successfully but the push step is rejected with a 403." — good. -->
`packages: write` is specifically required for GHCR (GitHub Container Registry, `ghcr.io`). Without it, the `docker/login-action` step authenticates successfully but the push step is rejected with a 403.

<!-- [COPY EDIT] "rejected with a 403" — HTTP status code; numeral is correct. -->
If you come from Jenkins, this maps to credentials binding -- scoping the available credentials to exactly what the job needs.

### The `ci` Job

<!-- [LINE EDIT] "Identical to the PR workflow. The same lint and test checks run on `main` even after merging. This catches the rare case where a merge introduces a conflict that was not caught in the PR (for example, two PRs that individually pass but conflict when both land on `main`). Running CI again on `main` is a small cost for a meaningful safety net." (60 words) — split: "Identical to the PR workflow. The same lint and test checks run on `main` even after merging. This catches the rare case where a merge introduces a conflict that was not caught in the PR — for example, two PRs that individually pass but conflict when both land on `main`. Running CI again on `main` is a small cost for a meaningful safety net." -->
Identical to the PR workflow. The same lint and test checks run on `main` even after merging. This catches the rare case where a merge introduces a conflict that was not caught in the PR (for example, two PRs that individually pass but conflict when both land on `main`). Running CI again on `main` is a small cost for a meaningful safety net.

### The `build-and-push` Job

<!-- [LINE EDIT] "**`needs: ci`** — This job only starts if the `ci` job succeeds." — good. -->
**`needs: ci`** — This job only starts if the `ci` job succeeds. If lint or tests fail on `main`, no images are published. `needs:` creates an explicit dependency between jobs; without it, jobs run in parallel from the start.[^3]

<!-- [LINE EDIT] "**`strategy: matrix: service: [auth, catalog, gateway, reservation, search]`** — This is a matrix build. GitHub Actions creates one job instance per matrix value and runs all five in parallel. Each instance has access to `${{ matrix.service }}`, which resolves to one of the five service names." — good. -->
**`strategy: matrix: service: [auth, catalog, gateway, reservation, search]`** — This is a matrix build. GitHub Actions creates one job instance per matrix value and runs all five in parallel. Each instance has access to `${{ matrix.service }}`, which resolves to one of the five service names.

<!-- [LINE EDIT] "The alternative would be five separate `build-and-push-auth`, `build-and-push-catalog`, etc. jobs -- identical except for the service name. The matrix eliminates that repetition. If you add a sixth service, you add its name to the list and GitHub handles the rest." — good. -->
<!-- [COPY EDIT] "etc." — CMOS 6.20: use "etc." sparingly and set off with commas. Current usage acceptable. -->
The alternative would be five separate `build-and-push-auth`, `build-and-push-catalog`, etc. jobs -- identical except for the service name. The matrix eliminates that repetition. If you add a sixth service, you add its name to the list and GitHub handles the rest.

If you come from Jenkins, this maps to Jenkins parallel stages. If you come from Gradle, think of it as running the same task across a set of subprojects.

<!-- [LINE EDIT] "**`docker/login-action@v3`** — Authenticates with `ghcr.io` using the workflow's automatic `GITHUB_TOKEN`. You do not create or store this token; GitHub provisions it per run with the permissions declared above." — good. -->
**`docker/login-action@v3`** — Authenticates with `ghcr.io` using the workflow's automatic `GITHUB_TOKEN`. You do not create or store this token; GitHub provisions it per run with the permissions declared above. The `github.actor` context variable is the username of the person (or app) that triggered the workflow -- used as the registry username.

**`docker/build-push-action@v6`** — Builds and pushes the Docker image. The relevant fields:

<!-- [COPY EDIT] "The Docker build context is the repository root." — good. -->
- `context: .` — The Docker build context is the repository root. This is necessary because services may reference shared code or the root `go.mod`.
- `file: services/${{ matrix.service }}/Dockerfile` — Each service has its own Dockerfile at a known path.
<!-- [STRUCTURAL] Note: the chapter has emphasized Earthly for building throughout, but the `build-and-push` job uses Dockerfiles. This is explained later in 10.5, but a forward pointer here would prevent the reader from wondering "wait, we have Earthfiles — why Dockerfiles here?". Suggest: "See section 10.5 for why the publishing step uses Dockerfiles rather than Earthly." -->
- `push: true` — Actually push the image. If `false`, the action builds but discards the result, which is useful for validating the Dockerfile without publishing.
- `tags:` — Two tags are applied to each image:
  - `sha-${{ github.sha }}` — An immutable tag tied to the exact commit. `github.sha` is the full 40-character commit hash. Using a `sha-` prefix avoids collisions with version tags like `v1.0.0` and makes the tag's meaning obvious.
<!-- [COPY EDIT] "40-character" — hyphenated compound adjective with a numeral. CMOS 7.81 and 9.13 (numerals with units/measurements). Good. -->
  - `latest` — A mutable tag always pointing to the most recent build from `main`. Useful for pulling without specifying a SHA, but not suitable for production deployments where you want reproducibility.

### GitHub Context Variables

<!-- [STRUCTURAL] Variable reference table is a nice touch. -->
The workflow uses three context variables:

| Variable | Value | Example |
|---|---|---|
| `github.sha` | Full commit hash of the triggering push | `a3f2d8c1...` |
| `github.repository` | `owner/repo` in lowercase | `acme/library-system` |
| `github.actor` | Username that triggered the workflow | `jsmith` |

<!-- [COPY EDIT] "`owner/repo` in lowercase" — GHCR requires lowercase, but `github.repository` itself preserves case of the repository name. The lowercasing is a registry-side concern, not a context variable concern. Please verify: is `github.repository` itself always lowercase, or does it reflect the repo's actual case? (GitHub repo names are case-insensitive but case-preserving; the context variable preserves whatever case was used.) -->
These are read-only and injected by GitHub. You reference them with the `${{ }}` expression syntax. They are not secrets -- they are metadata about the current run.

### Earthly-Push Alternative

> **Trade-off: `docker/build-push-action` vs. Earthly push**
>
> You could replace the `docker/login-action` and `docker/build-push-action` steps with a single Earthly call:
>
> ```yaml
> - run: earthly --push ./services/${{ matrix.service }}+docker
> ```
>
<!-- [LINE EDIT] "This uses Earthly's `--push` flag to build and push the image in one step, with the registry credentials passed via environment variables. The advantage is consistency: the same `+docker` target works locally and in CI. The disadvantage is that you lose GitHub's built-in integrations: OIDC-based keyless signing, layer cache export to the Actions cache, and the SBOM (Software Bill of Materials) provenance attestations that `docker/build-push-action` can generate automatically. For a learning project those features are optional. For a production system they are worth having." (84 words) — split into two shorter sentences. -->
<!-- [COPY EDIT] "OIDC-based" and "cloud-based" compound adjectives hyphenated before noun. Good. -->
<!-- [COPY EDIT] "SBOM (Software Bill of Materials)" — acronym defined on first use per CMOS 10.3. Good. -->
> This uses Earthly's `--push` flag to build and push the image in one step, with the registry credentials passed via environment variables. The advantage is consistency: the same `+docker` target works locally and in CI. The disadvantage is that you lose GitHub's built-in integrations: OIDC-based keyless signing, layer cache export to the Actions cache, and the SBOM (Software Bill of Materials) provenance attestations that `docker/build-push-action` can generate automatically. For a learning project those features are optional. For a production system they are worth having.

---

## Why Two Workflows

<!-- [STRUCTURAL] Repeats material covered earlier ("Why a Separate Workflow for PRs?") — consider consolidating or cross-referencing. The extra value here is the security framing (permissions scoping for PR runs from forks). -->
The split between `pr.yml` and `main.yml` encodes a policy decision in code:

<!-- [COPY EDIT] "Anyone can open a PR." — plain declarative. Good. -->
- **PRs validate.** Anyone can open a PR. You want fast feedback on whether the change is correct. You do not want to publish anything yet.
- **Main publishes.** Only merged commits land on `main`. At that point, CI has already passed (on the PR), and you want a versioned artifact.

<!-- [LINE EDIT] "If you used a single workflow triggered on both `push` and `pull_request`, you would need conditional logic (`if: github.event_name == 'push'`) to skip publishing on PR runs. Separate files are more readable and easier to audit." (37 words) — good. -->
If you used a single workflow triggered on both `push` and `pull_request`, you would need conditional logic (`if: github.event_name == 'push'`) to skip publishing on PR runs. Separate files are more readable and easier to audit.

<!-- [COPY EDIT] "supply-chain attack" — compound adjective hyphenated (CMOS 7.81). Good. -->
There is also a security reason: the `packages: write` permission needed for GHCR is only declared in `main.yml`. The PR workflow never requests it. Since PRs can come from forks, limiting permissions on PR runs reduces the blast radius of a supply-chain attack embedded in a dependency or Action.[^2]

---

## JVM Comparisons

<!-- [STRUCTURAL] This comparison table is an excellent teaching tool for the target reader. -->
| GitHub Actions concept | JVM equivalent |
|---|---|
| Workflow file (`.github/workflows/*.yml`) | Jenkinsfile or TeamCity build configuration |
| `on: pull_request` / `on: push` | Jenkins branch-based triggers or Multibranch Pipeline |
| Actions Marketplace (`uses:`) | Jenkins plugins or Gradle plugins applied in `build.gradle` |
| `strategy: matrix:` | Jenkins `parallel` stages or Gradle multi-project builds |
| `GITHUB_TOKEN` | Jenkins credentials binding (`withCredentials`) |
<!-- [COPY EDIT] "Jenkins `stage` with `when { expression { ... } }` or explicit dependencies" — Groovy DSL is correct. Good. -->
| `needs: ci` | Jenkins `stage` with `when { expression { ... } }` or explicit dependencies |
| `github.sha` | `env.GIT_COMMIT` in Jenkins |
| `permissions:` block | Jenkins role-based access control on credentials |

<!-- [LINE EDIT] "The biggest structural difference from Jenkins is that GitHub Actions is fully cloud-managed. There is no Jenkins master to maintain, no plugin compatibility matrix to manage, and no node configuration. The trade-off is that you are locked into GitHub's infrastructure and pricing model. For an open-source project or a team already on GitHub, that trade-off is usually worth it." (60 words) — good; acceptable length. -->
<!-- [COPY EDIT] "Jenkins master" — GitHub/Jenkins have adopted "controller" in preference to "master" for inclusive language (Jenkins project renamed "master" → "controller" circa 2020). Suggest "Jenkins controller". -->
<!-- [COPY EDIT] "open-source project" — compound adjective hyphenated before noun (CMOS 7.81). Good. -->
The biggest structural difference from Jenkins is that GitHub Actions is fully cloud-managed. There is no Jenkins master to maintain, no plugin compatibility matrix to manage, and no node configuration. The trade-off is that you are locked into GitHub's infrastructure and pricing model. For an open-source project or a team already on GitHub, that trade-off is usually worth it.

---

## Exercises

<!-- [STRUCTURAL] Exercises are strong — trace-through, multi-arch extension, notification integration, and deployment job. Each escalates in complexity. -->
1. **Trace a PR merge end-to-end.** Starting from when you push a commit to a feature branch and open a PR, list every GitHub Actions step that runs before and after the merge. Which workflow fires at each point? Which steps run in parallel?

<!-- [LINE EDIT] "Modify `main.yml` to also build a `linux/arm64` image for each service alongside the existing `linux/amd64` image." — good. -->
<!-- [COPY EDIT] "linux/arm64" and "linux/amd64" — canonical Docker platform strings; good. -->
2. **Add a matrix dimension.** Modify `main.yml` to also build a `linux/arm64` image for each service alongside the existing `linux/amd64` image. The `docker/build-push-action` supports a `platforms:` field. What changes in the matrix? What changes in the tags? How would you name the images to distinguish architectures?

<!-- [COPY EDIT] "slackapi/slack-github-action" — inline code. Good. -->
3. **Add a notify step.** Add a final step to the `build-and-push` job that posts a Slack message when the build completes -- both on success and on failure. Use `if: always()` to ensure the step runs even if earlier steps fail. Look up the `slackapi/slack-github-action` Action on the Marketplace. What secret do you need to configure?

<!-- [LINE EDIT] "Add a third job, `deploy-staging`, that runs after `build-and-push` and calls a fictional `kubectl set image` command. It should only run on pushes to `main`, use the `sha-${{ github.sha }}` tag (not `latest`), and require the `build-and-push` job for all five services to succeed before starting. How do you express that dependency when `build-and-push` is a matrix job?" (60 words) — acceptable. -->
4. **Implement a staging deploy job.** Add a third job, `deploy-staging`, that runs after `build-and-push` and calls a fictional `kubectl set image` command. It should only run on pushes to `main`, use the `sha-${{ github.sha }}` tag (not `latest`), and require the `build-and-push` job for all five services to succeed before starting. How do you express that dependency when `build-and-push` is a matrix job?

---

## References

<!-- [COPY EDIT] Please verify URLs in references resolve: docs.github.com/en/actions/learn-github-actions/understanding-github-actions ; docs.github.com/en/actions/security-guides/automatic-token-authentication ; docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idneeds -->
<!-- [COPY EDIT] URL paths in GitHub Actions docs have been restructured since 2024. Recommend re-verifying each link. -->
[^1]: [GitHub Actions -- Understanding GitHub Actions](https://docs.github.com/en/actions/learn-github-actions/understanding-github-actions) -- Overview of workflows, jobs, steps, runners, and the event model.
[^2]: [GitHub Actions -- Automatic token authentication](https://docs.github.com/en/actions/security-guides/automatic-token-authentication) -- How `GITHUB_TOKEN` works, what permissions it grants by default, and how to restrict them with the `permissions:` block.
[^3]: [GitHub Actions -- Workflow syntax: `jobs.<job_id>.needs`](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idneeds) -- Reference for expressing dependencies between jobs, including how to handle matrix job dependencies.
