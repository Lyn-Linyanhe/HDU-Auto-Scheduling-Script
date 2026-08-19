# Commands

## `course-exporter`

Compatibility and development entry for running the course exporter on its own. The unified main program reuses `cmd/course-exporter/web/` at `/exporter/`, so the web directory remains a runtime source dependency even though the standalone command is not included as a separate executable in release packages.

Run with:

```powershell
go run ./cmd/course-exporter
```

## `hdu-testlab`

Deterministic loopback-only teaching-system simulator used by acceptance tests. It never contacts HDU endpoints and must continue to reject non-loopback listen addresses.
