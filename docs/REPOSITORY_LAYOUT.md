# Repository Layout and Publishing Boundary

## Track In Git

The repository contains the Go source, frontend source, tests, deterministic `testdata/`, documentation, build scripts, `VERSION`, and module files.

`HDU-Smart-Course-Agent/` is an intentional part of this monorepo. It keeps its own Go module, but it is not a second repository.

## Keep Local

The following files are local runtime data or generated output and must not be committed:

- `course.json`, personal/current/target timetable snapshots, diagnostics, and backups;
- login configuration, cookies, execution plans, execution logs, and Excel exports;
- browser profiles and their database files;
- `dist/`, `release/`, `.gocache/`, temporary test folders, executables, ZIP files, and private notes.

The real `HDU-KillCourse-main` directory is an external dependency. It is configured locally and is not copied into this repository.

## Release Flow

1. Run `powershell -ExecutionPolicy Bypass -File scripts/build-release.ps1`.
2. Inspect `release/HDU-Auto-Scheduling-Script-v<version>/manifest.json`.
3. Run the release main smoke test and the Smart Agent checks from the generated package. If the default port is busy, pass `-Port <free-port>` to `release-main-smoke.ps1` and set `HDU_SMART_AGENT_PORT` for the Smart Agent checks.
4. Upload only the generated ZIP as a GitHub Release asset for the matching tag.

The tag, `VERSION`, package directory, ZIP name, and manifest version must match. A release asset is separate from the Git file tree, so a successful `git push` does not upload files under `release/`.

## Current Publishing State

The repository `master` is at version `0.5.3`. The local `release/` directory may contain the matching package, while GitHub Releases can lag behind the source branch. Before publishing, rebuild the package from the exact release commit and verify its manifest and checksums.
