<!-- [STRUCTURAL] Chapter opener. Mission statement, learning objectives, and an ASCII diagram of the pipeline — well scoped for a short landing page. Consider adding a single sentence naming the five services, or a link to where they are introduced, so a reader who jumps in here knows the system under discussion. -->
# Chapter 10: CI/CD with GitHub Actions & Earthly

<!-- [STRUCTURAL] Opening promise is good. "production-grade" is a strong claim — consider softening given this is a learning project without secrets rotation, signed images enforced, or promotion gates. -->
<!-- [LINE EDIT] "Rather than scripting builds directly in YAML, we use Earthly to encapsulate all build logic in portable, cacheable targets — and GitHub Actions purely as the trigger and orchestration layer." → "Rather than scripting builds directly in YAML, we put all build logic in Earthly targets — portable and cacheable — and use GitHub Actions purely as trigger and orchestration." -->
<!-- [COPY EDIT] "GitHub Actions & Earthly" in the H1 uses an ampersand. CMOS 6.14 allows ampersands in titles of works. Consistent with chapter naming in other chapters — check series consistency. -->
<!-- [COPY EDIT] "CI/CD" should be spelled out on first use in the chapter: "continuous integration / continuous delivery (CI/CD)". The fundamentals file does this, but the chapter opener references CI/CD before the definition. -->
In this chapter, we build a production-grade CI/CD pipeline for the library system. Rather than scripting builds directly in YAML, we use Earthly to encapsulate all build logic in portable, cacheable targets — and GitHub Actions purely as the trigger and orchestration layer. The result is a pipeline that runs identically on your laptop and in CI.

## What You'll Learn

<!-- [STRUCTURAL] Good bulleted objectives. They correspond 1:1 to the sibling sections, which is what a chapter opener should do. -->
<!-- [COPY EDIT] "CI/CD fundamentals: the feedback loop, fast failure, and why reproducibility matters" — serial comma present (CMOS 6.19). Good. -->
- CI/CD fundamentals: the feedback loop, fast failure, and why reproducibility matters
<!-- [COPY EDIT] "`COPY`-based layer caching" — compound adjective hyphenated before noun (CMOS 7.81). Good. -->
- Earthly's build model: targets, `COPY`-based layer caching, and how it compares to plain Docker builds
<!-- [COPY EDIT] "Linting Go code with `golangci-lint`" — product name correctly lowercased. -->
- Linting Go code with `golangci-lint` integrated as an Earthly target
- Writing GitHub Actions workflows for PR checks and multi-service matrix builds
<!-- [COPY EDIT] "GitHub Container Registry (GHCR)" — acronym defined on first use. Good. -->
- Publishing Docker images to GitHub Container Registry (GHCR) on merge to `main`
<!-- [COPY EDIT] "The two-tool philosophy" — em dash with no spaces (CMOS 6.85). Good. Note the italicized *what*, *when*, *where*. Consistent use of em dash throughout. -->
- The two-tool philosophy: Earthly owns *what* to build, GitHub Actions owns *when* and *where*

## Pipeline Architecture

<!-- [STRUCTURAL] The ASCII diagram is clear. Good. A later version could show where remote cache and attestations fit, but that may over-complicate for an opener. -->
```
Pull Request
  │
  └─► GitHub Actions: pr.yml
            │
            └─► earthly +ci          (per service)
                    ├── +lint         (golangci-lint)
                    └── +test         (go test ./...)

Merge to main
  │
  └─► GitHub Actions: main.yml
            │
            └─► earthly +ci          (per service, matrix)
                    ├── +lint
                    ├── +test
                    └── +build-and-push ──► ghcr.io/<org>/<service>:sha
```

<!-- [STRUCTURAL] Diagram caveat: `+build-and-push` is not an Earthly target in the rest of the chapter — the build-and-push step is a GitHub Actions job, not an Earthly target. This creates a contradiction with 10.5 ("The `build-and-push` job uses `docker/build-push-action`"). Recommend reworking the `main.yml` branch of the diagram so `+build-and-push` is shown as a GHA job rather than nested under `earthly +ci`. -->
<!-- [LINE EDIT] "Every step that runs in CI is an Earthly target, so you can reproduce any failure locally with a single `earthly +<target>` command — no CI-specific environment required." → "Every CI step is an Earthly target, so you can reproduce any failure locally with a single `earthly +<target>` command — no CI-specific environment required." -->
Every step that runs in CI is an Earthly target, so you can reproduce any failure locally with a single `earthly +<target>` command — no CI-specific environment required.
<!-- [FINAL] Final para ends the chapter opener abruptly. Consider a one-line forward pointer: "Section 10.1 starts with the fundamentals; 10.2 covers Earthly in depth; 10.3–10.5 put them to work in GitHub Actions." -->
