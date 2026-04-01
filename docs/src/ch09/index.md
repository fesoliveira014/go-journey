# Chapter 9: CI/CD with GitHub Actions & Earthly

In this chapter, we build a production-grade CI/CD pipeline for the library system. Rather than scripting builds directly in YAML, we use Earthly to encapsulate all build logic in portable, cacheable targets — and GitHub Actions purely as the trigger and orchestration layer. The result is a pipeline that runs identically on your laptop and in CI.

## What You'll Learn

- CI/CD fundamentals: the feedback loop, fast failure, and why reproducibility matters
- Earthly's build model: targets, `COPY`-based layer caching, and how it compares to plain Docker builds
- Linting Go code with `golangci-lint` integrated as an Earthly target
- Writing GitHub Actions workflows for PR checks and multi-service matrix builds
- Publishing Docker images to GitHub Container Registry (GHCR) on merge to `main`
- The two-tool philosophy: Earthly owns *what* to build, GitHub Actions owns *when* and *where*

## Pipeline Architecture

```
Pull Request
  │
  └─► GitHub Actions: pr-check.yml
            │
            └─► earthly +ci          (per service)
                    ├── +lint         (golangci-lint)
                    └── +test         (go test ./...)

Merge to main
  │
  └─► GitHub Actions: publish.yml
            │
            └─► earthly +ci          (per service, matrix)
                    ├── +lint
                    ├── +test
                    └── +build-and-push ──► ghcr.io/<org>/<service>:sha
```

Every step that runs in CI is an Earthly target, so you can reproduce any failure locally with a single `earthly +<target>` command — no CI-specific environment required.
