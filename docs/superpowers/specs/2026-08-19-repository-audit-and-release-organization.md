# Repository Audit and Release Organization Specification

## Goal

Keep the monorepo easy to publish and audit: source code, documentation, tests, and build scripts belong in Git; personal data and generated binaries stay local; distributable ZIP files are published as GitHub Release assets.

## Current Facts

- The local and remote \`master\` branches point to commit \`6d00445\`.
- \`VERSION\` is \`0.5.3\`.
- The latest GitHub Release is still \`v0.5.0\`.
- The local \`release/\` directory contains \`v0.5.3\` output and older/local build copies, but \`/release/\` is intentionally ignored by Git.
- \`HDU-KillCourse-main\` is an external dependency and must not be copied into this repository.

## Public Repository Boundary

Keep these in Git:

- Go source, frontend source, tests, \`testdata/\`, \`docs/\`, and \`scripts/\`.
- \`README.md\`, \`VERSION\`, \`go.mod\` files, and \`.gitignore\`.
- Redacted, deterministic test fixtures only.

Keep these out of Git:

- \`course.json\`, personal/current/target timetable snapshots, diagnostics, logs, execution plans, login configuration, cookies, Excel exports, browser profiles, and private planning notes.
- \`dist/\`, \`release/\`, \`.gocache/\`, temporary browser/test directories, executables, ZIP files, and backup files.

Publish these separately as GitHub Release assets:

- A freshly built \`HDU-Auto-Scheduling-Script-v0.5.3.zip\` generated from the release commit.

## Security Finding To Preserve For Follow-up

The Smart Agent \`/api/plan\` response currently includes the full generated KillCourse configuration. That configuration can contain account passwords, session cookies, and SMTP passwords. This audit records the finding; a separate API-contract change must remove those secrets from routine responses without breaking explicit config export or execution preparation.

## Acceptance Criteria

1. Repository documentation clearly distinguishes tracked source, local runtime data, and Release assets.
2. New root-level ZIPs, backup variants, and browser database files are ignored without changing the tracking state of existing source files.
3. A fresh \`v0.5.3\` package can be built from the current repository and its manifest contains only the documented distributable files.
4. Go tests, release smoke checks, and Git status pass after the organization work.
5. No personal timetable, credential, cookie, browser profile, executable, or ZIP is added to Git.

