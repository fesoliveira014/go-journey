# 10.1 CI/CD Fundamentals

Before configuring a single Earthly target or GitHub Actions workflow, we need a clear picture of what CI/CD is — and why the two-tool approach in this chapter works the way it does. This section is theory only. By the end you will understand the terms, the feedback loop, and why "it builds on my machine" stopped being acceptable.

---

## Continuous Integration

**Continuous Integration (CI)** means merging your changes to the shared branch frequently — multiple times per day if possible — and running automated checks on every push. The goal is to catch integration problems early, when they are still cheap to fix.

The key word is *integration*. When two developers change the same codebase independently for a week and then try to merge, the resulting conflicts — in code, in behavior, in assumptions — are expensive. CI trades one large painful merge event for many small, low-cost merges. Each merge is automatically validated by the same test suite, so regressions surface within minutes rather than days.

If you have used Jenkins or TeamCity on a JVM project, you have used CI. The pipeline triggered on every push, ran `./gradlew test`, and failed the build if tests broke. That is the essential pattern. Everything else — parallelism, caching, reporting — is refinement.

The operational definition: **every push to the shared branch runs the full automated check suite, and the result is visible to the team within minutes**.

---

## Continuous Delivery

**Continuous Delivery (CD)** extends CI by ensuring that every green build produces a releasable artifact. In our case, that artifact is a Docker image. After the checks pass, the pipeline builds and publishes an image tagged with the commit SHA. The image could be deployed to production at any time — manually, with a click, or on a schedule.

The key distinction:

| Practice | What it validates | What it produces |
|---|---|---|
| CI | Code integrates correctly | Confidence |
| CD | Code is deployable | A shippable artifact |

CD does not mean you deploy to production on every commit. It means you *could*. The artifact exists, it is tagged, and the deployment step is a deliberate human decision — or an automated one, which brings us to the next term.

In the Gradle world, this maps to the distinction between `./gradlew test` (CI: did it compile and pass tests?) and `./gradlew test publish` (CD: did it compile, pass tests, *and* push a versioned artifact to Nexus or Artifactory?).

---

## Continuous Deployment

**Continuous Deployment** takes CD one step further: every green build is automatically deployed to production with no human approval. The deployment step is part of the pipeline, not a separate decision.

This chapter does not implement Continuous Deployment. Deploying to Kubernetes requires a running cluster, rolling update strategies, health checks, and rollback mechanisms — topics covered in the Kubernetes chapter. What we build here lays the groundwork: every merge produces a tagged Docker image that a deployment pipeline could pick up automatically.

---

## The Feedback Loop

Every CI/CD pipeline is fundamentally a feedback loop. Push code, get a signal. The faster and more specific the signal, the cheaper the correction.

Our pipeline has five stages:

```
  push
    │
    ▼
  lint ──── catches: style violations, unused imports,
    │               suspicious constructs (go vet)
    ▼
  test ──── catches: logic errors, regressions,
    │               integration failures
    ▼
  build ─── catches: compilation errors, missing
    │               dependencies, Dockerfile issues
    ▼
 publish ── produces: versioned Docker image pushed
                     to container registry
```

Each stage has a different failure mode and a different cost:

- **Lint failures** are the cheapest. They are caught in seconds and fixed in seconds.
- **Test failures** take longer but catch real bugs before they ship.
- **Build failures** are rare if lint and tests pass, but they catch environment-specific issues.
- **Publish failures** indicate infrastructure problems: registry unreachable, credentials expired.

The ordering matters. You want the cheapest checks first. Running lint after a 5-minute test suite means a trivial formatting error costs five minutes. Running lint first means it costs ten seconds.

This is the same reasoning behind Gradle's task ordering. `compileJava` runs before `test` because compilation is fast and a compile error makes test results meaningless. The CI pipeline applies the same logic across the full delivery process.

---

## Build Reproducibility

The phrase "works on my machine" represents a specific failure: the build depends on something present on your machine but absent on others. Common culprits include:

- A globally installed tool at a different version (`golangci-lint` 1.54 vs. 1.62)
- An environment variable set in your shell profile
- A local Go module replace directive left in `go.mod`
- macOS vs. Linux path or filesystem behavior

Reproducibility means: **given the same inputs (source code, dependencies), the build produces the same outputs on any machine**. The mechanism is containers. If your build runs inside a Docker container with a pinned image, the environment is identical everywhere — your laptop, the CI server, your colleague's machine.

Earthly, which we cover in Section 10.2, enforces this by running every build target inside a container. You cannot accidentally depend on your local Go installation because the build uses the Go version declared in the Earthfile, not the one in your PATH.

GitHub Actions runners provide a similar guarantee at the CI level: every run starts from a clean, known-good VM image. But runners are not available locally. Earthly solves the local half; GitHub Actions solves the cloud half. Together, they eliminate the gap.

---

## The Two-Tool Approach

This chapter uses two tools that serve different roles:

**Earthly** is a build tool. It defines *what* to build and *how*: fetch dependencies, run linters, run tests, build Docker images. An Earthfile is like a Makefile crossed with a Dockerfile: each target runs in a container, caches layers intelligently, and can be invoked with `earthly +target` from your terminal. If you come from Gradle, think of Earthly as Gradle with first-class Docker layer caching and a containerized execution model.

**GitHub Actions** is an orchestration platform. It defines *when* to run things: on push, on pull request, on a schedule, on a tag. It handles secrets (registry credentials, API keys), matrix builds (test on Go 1.22 and 1.23), and cloud integration (deploy to EKS, notify Slack). If you come from Jenkins or TeamCity, GitHub Actions is the same category of tool — a pipeline runner triggered by repository events.

Why use both?

| Concern | Earthly | GitHub Actions |
|---|---|---|
| Build logic | Yes — defined in Earthfile | No — GHA does not define builds |
| Runs locally | Yes — `earthly +test` | No — requires pushing to GitHub |
| Secrets management | No | Yes — encrypted secrets per repo |
| Trigger on push/PR/tag | No | Yes |
| Cloud integration | No | Yes |
| Reproducible environment | Yes — containerized | Partial — runners reset, but not portable locally |

The practical benefit: when a CI build fails, you reproduce it locally with `earthly +test`. No debug commit, no three-minute wait for a runner, no reading logs in a browser. This is the single biggest quality-of-life improvement over a pure GitHub Actions build.

The GitHub Actions workflow in this chapter is deliberately thin. It installs Earthly and calls `earthly +ci`. All the real logic lives in the Earthfile. GitHub Actions handles triggers and secrets, nothing more.

---

## Exercises

1. **Map your feedback loop.** Draw the CI/CD pipeline for your current or most recent project (Jenkins, TeamCity, GitHub Actions, or any other tool). Identify each stage, what it checks, and roughly how long it takes. Where are the bottlenecks? What is the total time from push to a deployable artifact?

2. **Compare to a Jenkins pipeline.** If you have worked with a Jenkinsfile, compare its structure to the two-tool approach described here. What does the Jenkinsfile define that would move into an Earthfile? What stays in the orchestration layer? What does Jenkins give you that GitHub Actions does not, and vice versa?

3. **Think through Continuous Deployment.** Suppose we wanted to add automatic deployment to a staging Kubernetes cluster on every merge to `main`. What additional components would the pipeline need? What new failure modes would you need to handle (partial rollout, failed health check, database migration failures)?

4. **Identify reproducibility risks.** Look at a build you own or have recently worked with. What does it depend on that is not explicitly declared — host OS tools, environment variables, implicit filesystem paths? How would you eliminate each dependency?

---

## References

[^1]: [Martin Fowler — Continuous Integration](https://martinfowler.com/articles/continuousIntegration.html) — The definitive article on CI: what it is, why it matters, and what practices make it effective. Written in 2006, still accurate.
[^2]: Jez Humble and David Farley, *Continuous Delivery: Reliable Software Releases through Build, Test, and Deployment Automation* (Addison-Wesley, 2010) — The book that established CD as a discipline. Covers the deployment pipeline pattern in depth.
[^3]: [GitHub Actions Documentation](https://docs.github.com/en/actions) — Official reference for workflow syntax, triggers, runners, and secrets management.
