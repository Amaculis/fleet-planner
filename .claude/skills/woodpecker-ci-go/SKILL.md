---
name: woodpecker-ci-go
description: >
  Woodpecker CI/CD pipeline conventions for Go microservices at git.zzdats.lv.
  Use when creating, reviewing, or updating .woodpecker/*.yaml pipeline files for Go APIs.
  Covers approved image versions, golangci-lint + reviewdog, trivy security scanning,
  SonarQube integration, DefectPortal reporting, Docker buildx, cosign image signing,
  and Portainer deploy webhooks. Trigger this skill whenever the user mentions Woodpecker CI,
  pipeline YAML files, .woodpecker/ directories, or CI/CD for Go services at zzdats.lv —
  even if they just ask to "review" or "fix" a pipeline without naming the skill explicitly.
---

# Woodpecker CI/CD Pipelines — Go Conventions

Canonical example files live in `references/` next to this skill. Read the relevant file
when you need a full, authoritative template — they come directly from the internal woodpecker
examples repo and reflect the current approved patterns.

| Reference file | When to read it |
|----------------|-----------------|
| `references/build.yaml` | Main pipeline: vendor, build, test, lint, trivy + DefectPortal, sonar, docker push to develop |
| `references/release.yaml` | Tag-triggered release: versioned build, docker auto_tag, cosign signing |

---

## Approved Image Versions

| Purpose | Image |
|---------|-------|
| Go build | `golang:1.26-alpine` |
| golangci-lint | `golangci/golangci-lint:v2.11.4` (live repos) / `v2.11` (examples) |
| reviewdog + lint | `woodpeckerci/plugin-reviewdog-golangci-lint:2.11` |
| Docker build | `woodpeckerci/plugin-docker-buildx:latest` |
| Trivy scanner | `git.zzdats.lv/woodpeckerci/trivy:latest` |
| Trivy vuln DB | `git.zzdats.lv/woodpeckerci/trivy-db:2` |
| Trivy Java DB | `git.zzdats.lv/woodpeckerci/trivy-java-db:1` |
| Trivy checks | `git.zzdats.lv/woodpeckerci/trivy-checks:0` |
| SonarQube scanner | `sonarsource/sonar-scanner-cli:latest` |
| Cosign sign | `git.zzdats.lv/woodpeckerci/cosign-sign:1.0.0` |
| Sonar report (DefectPortal) | `node:24.14` |
| Curl reporter | `curlimages/curl:latest` |
| Portainer curl | `byrnedo/alpine-curl` |
| Scan result check | `alpine:latest` |
| Gitea SQ PR comments | `git.zzdats.lv/internal/gitea-sq-reporter:latest` |

> **Never use**: `aquasec/trivy:latest`, `git.zzdats.lv/zzdats/trivy*` (wrong namespace — must be `woodpeckerci/`), `DRONE_TAG` (dead — use `CI_COMMIT_TAG`), `mtu: 1280` in docker settings, reviewdog `1.61` or `2.8` on go1.26+ (use `2.11`).

---

## Pipeline Structure

The standard pattern is a single `.woodpecker/build.yaml` covering everything, with explicit `when` conditions on every step rather than relying on global trigger inheritance:

```
.woodpecker/
  build.yaml    # all events: vendor, build, test, lint, trivy, sonar, docker, release, cosign
```

Each step declares exactly when it runs (branch + event). This makes the file self-documenting and avoids surprises from global trigger inheritance. See `references/build.yaml` for the full example.

For repos that want a dedicated release file (e.g. to keep the main file shorter), see `references/release.yaml`.

---

## Trigger Conventions

```yaml
# Default branch push + PR (use for tests pipeline)
when:
  - branch: ${CI_REPO_DEFAULT_BRANCH}
    event: push
  - event: pull_request

# Branch push only (use for build/deploy)
when:
  branch: [main, develop]
  event: push

# Tag only
when:
  event: [tag]
```

Use `${CI_REPO_DEFAULT_BRANCH}` instead of hardcoding `[develop]` for the tests pipeline —
it respects repos where `main` is the default.

---

## Secrets Reference

| Secret name | Used for |
|-------------|----------|
| `package_token` | Private Go module auth + Trivy registry auth |
| `docker_username` / `docker_password` | Registry push/pull |
| `reviewdog_token` | PR lint annotations + gitea-sq-reporter |
| `sonar_token` | SonarQube scanner |
| `defect_portal_dojo_token` | DefectPortal API |
| `portainer_webhook_dev` | Portainer develop deploy webhook |
| `portainer_webhook_tv` | Portainer staging deploy webhook |
| `hsm_password` | Cosign HSM signing |
| `cosign_public_key` | Cosign public key |
| `rekor_public_key` | Rekor transparency log key |

---

## Key Patterns

### Trivy scanning (always use air-gapped mirrors)

Always set all four env vars together. Use `failure: ignore` on scan steps so pipeline
continues even if the DB pull is slow. The `sharedenv` pattern lets the final
`vulnerability-scan-result-check` step fail the build if any scan had HIGH/CRITICAL findings,
even if the scan step itself was set to `failure: ignore`.

```yaml
environment:
  TRIVY_DB_REPOSITORY: git.zzdats.lv/woodpeckerci/trivy-db:2
  TRIVY_JAVA_DB_REPOSITORY: git.zzdats.lv/woodpeckerci/trivy-java-db:1
  TRIVY_CHECKS_BUNDLE_REPOSITORY: git.zzdats.lv/woodpeckerci/trivy-checks:0
  TRIVY_USERNAME: token
  TRIVY_PASSWORD:
    from_secret: package_token
```

### SonarQube reporting — two separate steps, different purposes

**On push** (`sonarqube-code-check-send-report`): generates a full HTML report via `sonar-report`
npm package and uploads it to DefectPortal. Uses `node:24.14`.

**On PR** (`sonarqube-code-check-pr-comment-summary`): posts an inline summary comment to the
Gitea PR. Uses `gitea-sq-reporter`. These are different steps with different images — don't
conflate them.

Always pass `sonar.token=$${SONAR_TOKEN}` (double `$$` for Woodpecker escaping) as a `-D` param,
not just as an environment variable.

### golangci-lint output

```yaml
golangci-lint run --timeout 3m --output.checkstyle.path=golangci-lint.out --output.text.path stdout
```

The `--output.checkstyle.path` flag produces the file consumed by SonarQube. The `--output.text.path stdout`
flag replaces the deprecated `--output.text.print-issued-lines` / `--output.text.print-linter-name` flags.

---

## Common Pitfalls

| Problem | Fix |
|---------|-----|
| `DRONE_TAG##v` in build flags | Replace with `CI_COMMIT_TAG##v` |
| `mtu: 1280` in docker step settings | Remove it |
| `aquasec/trivy:latest` | Replace with `git.zzdats.lv/woodpeckerci/trivy:latest` |
| `git.zzdats.lv/zzdats/trivy-*` | Replace namespace `zzdats/` → `woodpeckerci/` |
| reviewdog `:1.61` or `:2.8` on go1.26 | Use `:2.11` |
| Go module path ≠ repo clone path | Fix `go.mod` module directive; causes SonarQube "Failed to find test file" |
| `sonar.go.golangci-lint.reportPaths` missing | Pass `--output.checkstyle.path=golangci-lint.out` to golangci-lint |
| Missing `sonar.token` `-D` param | Always pass `-D "sonar.token=$${SONAR_TOKEN}"` |
| `gitea-sq-reporter` used on push event | That image is for PR comments only; use `node:24.14` + sonar-report for push |
| Missing `vulnerability-scan-result-check` | Required after any trivy scan to actually fail the build |
