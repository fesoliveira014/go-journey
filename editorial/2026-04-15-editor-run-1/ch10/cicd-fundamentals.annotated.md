# 10.1 CI/CD Fundamentals

<!-- [STRUCTURAL] Strong opening. Theory-only framing sets expectations. Progression CI → CD → Deployment → Feedback Loop → Reproducibility → Two-Tool Approach → Exercises is logical and builds from vocabulary to rationale. -->
<!-- [LINE EDIT] "Before we configure a single Earthly target or GitHub Actions workflow, we need a clear picture of what CI/CD is and why the two-tool approach in this chapter works the way it does." (35 words) → "Before configuring a single Earthly target or GitHub Actions workflow, we need a clear picture of what CI/CD is — and why the two-tool approach in this chapter works the way it does." — Active voice; splits the two ideas with a dash. -->
<!-- [COPY EDIT] "This section is theory only." — fine. Consider "This section is theory-only." as a compound adjective before an elided noun, but standalone predicative use is fine without hyphen (CMOS 7.81). -->
<!-- [COPY EDIT] The file uses `--` (two hyphens) throughout instead of en dashes (–) or em dashes (—). CMOS 6.85: em dash has no surrounding spaces. Suggest converting all `--` to `—` for consistency with the rest of the chapter (index.md and image-publishing.md use em dashes). This is a repeated pattern; flagged once here. -->
Before we configure a single Earthly target or GitHub Actions workflow, we need a clear picture of what CI/CD is and why the two-tool approach in this chapter works the way it does. This section is theory only. By the end you will understand the terms, the feedback loop, and why "it builds on my machine" stopped being acceptable.

---

## Continuous Integration

<!-- [COPY EDIT] "**Continuous Integration (CI)** means merging your changes to the shared branch frequently -- multiple times per day if possible -- and running automated checks on every push." — convert `--` to `—`. -->
**Continuous Integration (CI)** means merging your changes to the shared branch frequently -- multiple times per day if possible -- and running automated checks on every push. The goal is to catch integration problems early, when they are still cheap to fix.

<!-- [LINE EDIT] "When two developers change the same codebase independently for a week and then try to merge, the resulting conflicts -- in code, in behavior, in assumptions -- are expensive." (28 words) — keep, good rhythm. -->
<!-- [COPY EDIT] "in code, in behavior, in assumptions" — serial comma not required here (asyndetic repetition works without it); acceptable as written (CMOS 6.19 applies to conjunctions). -->
The key word is *integration*. When two developers change the same codebase independently for a week and then try to merge, the resulting conflicts -- in code, in behavior, in assumptions -- are expensive. CI trades one large painful merge event for many small, low-cost merges. Each merge is automatically validated by the same test suite, so regressions surface within minutes rather than days.

<!-- [STRUCTURAL] JVM analogy (Jenkins/TeamCity) works well for the target reader. Consider moving it just before the operational definition so the definition lands as summary. -->
<!-- [COPY EDIT] "`./gradlew test`" — backticks consistent. Good. -->
If you have used Jenkins or TeamCity on a JVM project, you have used CI. The pipeline triggered on every push, ran `./gradlew test`, and failed the build if tests broke. That is the essential pattern. Everything else -- parallelism, caching, reporting -- is refinement.

<!-- [LINE EDIT] "The operational definition: **every push to the shared branch runs the full automated check suite, and the result is visible to the team within minutes**." — clean; keep. -->
The operational definition: **every push to the shared branch runs the full automated check suite, and the result is visible to the team within minutes**.

---

## Continuous Delivery

<!-- [COPY EDIT] "**Continuous Delivery (CD)**" — CD defined on first use. Good. -->
<!-- [LINE EDIT] "After the checks pass, the pipeline builds and publishes an image tagged with the commit SHA." — "commit SHA" should be "commit SHA" (acronym OK on second use); verify the acronym is not ambiguous with SHA-1/SHA-256. In context it is clearly "Git commit SHA". -->
**Continuous Delivery (CD)** extends CI by ensuring that every green build produces a releasable artifact. In our case, that artifact is a Docker image. After the checks pass, the pipeline builds and publishes an image tagged with the commit SHA. The image could be deployed to production at any time -- manually, with a click, or on a schedule.

The key distinction:

<!-- [COPY EDIT] Three-column table. Column alignments are `|---|---|---|`; rendered tables typically work without per-column spec. Acceptable. -->
| Practice | What it validates | What it produces |
|---|---|---|
| CI | Code integrates correctly | Confidence |
| CD | Code is deployable | A shippable artifact |

<!-- [LINE EDIT] "CD does not mean you deploy to production on every commit. It means you *could*." — crisp. Keep. -->
CD does not mean you deploy to production on every commit. It means you *could*. The artifact exists, it is tagged, and the deployment step is a deliberate human decision -- or an automated one, which brings us to the next term.

<!-- [COPY EDIT] "`./gradlew test publish`" — two tasks space-separated on a Gradle command line. Technically correct; consider showing as two commands for clarity: `./gradlew test && ./gradlew publish` or `./gradlew test publish` (which does work as a single Gradle invocation). -->
In the Gradle world, this maps to the distinction between `./gradlew test` (CI: did it compile and pass tests?) and `./gradlew test publish` (CD: did it compile, pass tests, *and* push a versioned artifact to Nexus or Artifactory?).

---

## Continuous Deployment

<!-- [STRUCTURAL] Good clear call-out that this chapter does NOT implement Continuous Deployment. Honest framing. Sets reader expectations. -->
**Continuous Deployment** takes CD one step further: every green build is automatically deployed to production with no human approval. The deployment step is part of the pipeline, not a separate decision.

<!-- [LINE EDIT] "Deploying to Kubernetes requires a running cluster, rolling update strategies, health checks, and rollback mechanisms -- topics covered in the Kubernetes chapter." — keep. -->
This chapter does not implement Continuous Deployment. Deploying to Kubernetes requires a running cluster, rolling update strategies, health checks, and rollback mechanisms -- topics covered in the Kubernetes chapter. What we build here lays the groundwork: every merge produces a tagged Docker image that a deployment pipeline could pick up automatically.

---

## The Feedback Loop

<!-- [STRUCTURAL] Excellent section. The diagram + failure-cost analysis is the load-bearing insight of the chapter. -->
Every CI/CD pipeline is fundamentally a feedback loop. Push code, get a signal. The faster and more specific the signal, the cheaper the correction.

Our pipeline has five stages:

<!-- [STRUCTURAL] The diagram lists five stages but the labeled flow arrow chain shows only four (lint → test → build → publish). The "push" is the input, so the pipeline is 4 stages plus a trigger. The sentence "five stages" (pre-diagram) then misleads. Recommend: "Our pipeline has four stages, triggered by a push:" — OR count push as stage 0 in the diagram. -->
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

<!-- [COPY EDIT] "**Lint failures** are the cheapest." — serial comma present in later bullet. Good. -->
- **Lint failures** are the cheapest. They are caught in seconds and fixed in seconds.
- **Test failures** take longer but catch real bugs before they ship.
- **Build failures** are rare if lint and tests pass, but they catch environment-specific issues.
<!-- [COPY EDIT] "**Publish failures** indicate infrastructure problems: registry unreachable, credentials expired." — sentence fragments are deliberate and fine. -->
- **Publish failures** indicate infrastructure problems: registry unreachable, credentials expired.

<!-- [COPY EDIT] "Running lint after a 5-minute test suite means a trivial formatting error costs 5 minutes." — CMOS 9.2 spells out zero–ninety-nine in prose, but "5-minute" is a unit-adjective (hyphenated) with a numeral per CMOS 9.13 (use numerals with units of measurement). Acceptable. For "costs 5 minutes", CMOS 9.7 prefers "five minutes" in prose. Suggest: "Running lint after a 5-minute test suite means a trivial formatting error costs five minutes." -->
The ordering matters. You want the cheapest checks first. Running lint after a 5-minute test suite means a trivial formatting error costs 5 minutes. Running lint first means it costs 10 seconds.

<!-- [COPY EDIT] "it costs 10 seconds" — CMOS 9.7 prefers "ten seconds" in prose. -->
<!-- [STRUCTURAL] Gradle analogy at the end of this section is a fine bridging touch. -->
This is the same reasoning behind Gradle's task ordering. `compileJava` runs before `test` because compilation is fast and a compile error makes test results meaningless. The CI pipeline applies the same logic across the full delivery process.

---

## Build Reproducibility

<!-- [STRUCTURAL] Good standalone section. The "works on my machine" hook pulls the reader in. -->
The phrase "works on my machine" represents a specific failure: the build depends on something present on your machine but absent on others. Common culprits include:

<!-- [COPY EDIT] "A globally installed tool at a different version (`golangci-lint` 1.54 vs. 1.62)" — "vs." is acceptable per CMOS 10.42. Note these version numbers should be verified against current golangci-lint releases. -->
<!-- [COPY EDIT] Please verify: golangci-lint versions 1.54 and 1.62 are both valid published releases. -->
- A globally installed tool at a different version (`golangci-lint` 1.54 vs. 1.62)
- An environment variable set in your shell profile
- A local Go module replace directive left in `go.mod`
<!-- [COPY EDIT] "macOS vs. Linux path or filesystem behavior" — "macOS" correctly capitalized; "filesystem" is one word (consistent with Apple/Linux idiom). -->
- macOS vs. Linux path or filesystem behavior

<!-- [LINE EDIT] "Reproducibility means: **given the same inputs (source code, dependencies), the build produces the same outputs on any machine**." — good. -->
Reproducibility means: **given the same inputs (source code, dependencies), the build produces the same outputs on any machine**. The mechanism is containers. If your build runs inside a Docker container with a pinned image, the environment is identical everywhere -- your laptop, the CI server, your colleague's machine.

<!-- [COPY EDIT] "Earthly, which we cover in section 10.2" — "section" lowercased when followed by number is per CMOS 8.179 (acceptable both cases); consistency matters. Check whether other files use "Section 10.2" (capital S). image-publishing.md line 111 uses "Section 10.4" (capital). Recommend capitalizing: "Section 10.2". -->
Earthly, which we cover in section 10.2, enforces this by running every build target inside a container. You cannot accidentally depend on your local Go installation because the build uses the Go version declared in the Earthfile, not the one in your PATH.

<!-- [LINE EDIT] "GitHub Actions runners provide a similar guarantee at the CI level: every run starts from a clean, known-good VM image." (23 words) — good. -->
<!-- [COPY EDIT] "known-good" — hyphenated compound adjective (CMOS 7.81). Good. -->
GitHub Actions runners provide a similar guarantee at the CI level: every run starts from a clean, known-good VM image. But runners are not available locally. Earthly solves the local half; GitHub Actions solves the cloud half. Together, they eliminate the gap.

---

## The Two-Tool Approach

<!-- [STRUCTURAL] Core thesis of the chapter. Strong payoff section. -->
This chapter uses two tools that serve different roles:

<!-- [LINE EDIT] "An Earthfile is like a Makefile crossed with a Dockerfile: each target runs in a container, caches layers intelligently, and can be invoked with `earthly +target` from your terminal." (31 words) — clean. -->
<!-- [COPY EDIT] "Makefile crossed with a Dockerfile" — the capitalization "Makefile" and "Dockerfile" is correct per vendor convention (Docker, Inc.). -->
**Earthly** is a build tool. It defines *what* to build and *how*: fetch dependencies, run linters, run tests, build Docker images. An Earthfile is like a Makefile crossed with a Dockerfile: each target runs in a container, caches layers intelligently, and can be invoked with `earthly +target` from your terminal. If you come from Gradle, think of Earthly as Gradle with first-class Docker layer caching and a containerized execution model.

<!-- [LINE EDIT] "It handles secrets (registry credentials, API keys), matrix builds (test on Go 1.22 and 1.23), and cloud integration (deploy to EKS, notify Slack)." — good parallel structure. -->
<!-- [COPY EDIT] Please verify: "Go 1.22 and 1.23" — since this book references Go 1.26 in Earthfiles (earthly.md line 49), using 1.22/1.23 as matrix examples is mildly inconsistent. Consider updating to "Go 1.25 and 1.26" for internal consistency (current Go stable at 2026-04-15: please verify latest release series). -->
**GitHub Actions** is an orchestration platform. It defines *when* to run things: on push, on pull request, on a schedule, on a tag. It handles secrets (registry credentials, API keys), matrix builds (test on Go 1.22 and 1.23), and cloud integration (deploy to EKS, notify Slack). If you come from Jenkins or TeamCity, GitHub Actions is the same category of tool -- a pipeline runner triggered by repository events.

Why use both?

<!-- [COPY EDIT] Table cell "Yes -- defined in Earthfile" uses `--` (two hyphens). Convert to em dash per CMOS 6.85. Repeated across the table. -->
| Concern | Earthly | GitHub Actions |
|---|---|---|
| Build logic | Yes -- defined in Earthfile | No -- GHA does not define builds |
| Runs locally | Yes -- `earthly +test` | No -- requires pushing to GitHub |
| Secrets management | No | Yes -- encrypted secrets per repo |
| Trigger on push/PR/tag | No | Yes |
| Cloud integration | No | Yes |
| Reproducible environment | Yes -- containerized | Partial -- runners reset, but not portable locally |

<!-- [COPY EDIT] "wait 3 minutes" — CMOS 9.7: prefer "three minutes" in prose. -->
<!-- [LINE EDIT] "The practical benefit: when a CI build fails, you can reproduce it locally with `earthly +test`. You do not need to push a debug commit, wait 3 minutes for a runner to spin up, and read logs in a browser." (42 words) — could split: "The practical benefit: when a CI build fails, you reproduce it locally with `earthly +test`. No debug commit, no three-minute wait for a runner, no reading logs in a browser." -->
The practical benefit: when a CI build fails, you can reproduce it locally with `earthly +test`. You do not need to push a debug commit, wait 3 minutes for a runner to spin up, and read logs in a browser. This is the single biggest quality-of-life improvement over a pure GitHub Actions build.

<!-- [COPY EDIT] "quality-of-life" — hyphenated compound adjective before noun (CMOS 7.81). Good. -->
<!-- [LINE EDIT] "The GitHub Actions workflow in this chapter is deliberately thin. It installs Earthly, calls `earthly +lint`, `earthly +test`, and `earthly +publish`." — wait: `+publish` is not defined anywhere else in the chapter. The workflow actually calls `earthly +ci` (which runs lint+test) and then uses `docker/build-push-action`. Recommend: "It installs Earthly and calls `earthly +ci`." -->
<!-- [FINAL] The reference to `earthly +publish` here contradicts 10.3 and 10.5 where no such target exists. This is a factual error in the text. -->
The GitHub Actions workflow in this chapter is deliberately thin. It installs Earthly, calls `earthly +lint`, `earthly +test`, and `earthly +publish`. All the real logic lives in the Earthfile. GitHub Actions handles triggers and secrets, nothing more.

---

## Exercises

<!-- [STRUCTURAL] Four exercises, well varied (draw your own pipeline, compare to Jenkins, design-think Continuous Deployment, audit reproducibility). Good depth for an experienced engineer. -->
1. **Map your feedback loop.** Draw the CI/CD pipeline for your current or most recent project (Jenkins, TeamCity, GitHub Actions, or any other tool). Identify each stage, what it checks, and roughly how long it takes. Where are the bottlenecks? What is the total time from push to a deployable artifact?

<!-- [COPY EDIT] "(Jenkins, TeamCity, GitHub Actions, or any other tool)" — serial comma before "or" (CMOS 6.19). Good. -->
2. **Compare to a Jenkins pipeline.** If you have worked with a Jenkinsfile, compare its structure to the two-tool approach described here. What does the Jenkinsfile define that would move into an Earthfile? What stays in the orchestration layer? What does Jenkins give you that GitHub Actions does not, and vice versa?

<!-- [COPY EDIT] "i.e." / "e.g." — Exercise 3: "(partial rollout, failed health check, database migration failures)" — series with no introductory "i.e./e.g.". Good. -->
3. **Think through Continuous Deployment.** Suppose we wanted to add automatic deployment to a staging Kubernetes cluster on every merge to `main`. What additional components would the pipeline need? What new failure modes would you need to handle (partial rollout, failed health check, database migration failures)?

4. **Identify reproducibility risks.** Look at a build you own or have recently worked with. What does it depend on that is not explicitly declared -- host OS tools, environment variables, implicit file system paths? How would you eliminate each dependency?

<!-- [COPY EDIT] "file system" vs. "filesystem" — pick one. Earlier text (line 88) uses "filesystem" as one word. CMOS/Merriam-Webster accept both; be consistent within the manuscript. -->

---

## References

<!-- [COPY EDIT] Please verify: URL https://martinfowler.com/articles/continuousIntegration.html resolves. Article date 2006 claimed. -->
[^1]: [Martin Fowler -- Continuous Integration](https://martinfowler.com/articles/continuousIntegration.html) -- The definitive article on CI: what it is, why it matters, and what practices make it effective. Written in 2006, still accurate.
<!-- [COPY EDIT] Book citation. CMOS 14.100: author surname order, italicize book title, publisher, year. Current form: "Jez Humble and David Farley, *Continuous Delivery: Reliable Software Releases through Build, Test, and Deployment Automation* (Addison-Wesley, 2010)" — CMOS-compliant. Good. Consider adding "reading" vs "reference" context: this is a foundational text. -->
[^2]: Jez Humble and David Farley, *Continuous Delivery: Reliable Software Releases through Build, Test, and Deployment Automation* (Addison-Wesley, 2010) -- The book that established CD as a discipline. Covers the deployment pipeline pattern in depth.
<!-- [COPY EDIT] Please verify: https://docs.github.com/en/actions resolves. -->
[^3]: [GitHub Actions Documentation](https://docs.github.com/en/actions) -- Official reference for workflow syntax, triggers, runners, and secrets management.
