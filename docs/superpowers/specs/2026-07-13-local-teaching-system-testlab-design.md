# Local Teaching-System Test Lab Design

## Goal

Provide a deterministic, offline acceptance environment for the HDU course
exporter, scheduler, and Smart Course Agent while the real teaching system is
unavailable. The test lab must never use a real account, password, cookie, or
school endpoint.

## Scope

The first implementation covers the exporter protocol and data bridge:

- simulated NewJW login and public-key endpoints;
- simulated task-course and personal-schedule queries;
- successful export, authentication failure, authorization failure, malformed
  course response, empty course response, and timeout scenarios;
- generated `course.json` and `personal-schedule.json` consumed by the
  scheduler and Smart Agent's existing end-to-end tests.

It does not emulate the full school UI, QR login, or real course selection.
The existing Smart Agent dry-run and execution-log fixtures remain the
simulation boundary for KillCourse behavior in this iteration.

## Architecture

`school.ExporterEndpoints` holds the six endpoint URLs used by an exporter.
Production constructors use the existing HDU URLs unchanged. Test constructors
accept endpoint overrides only after validating every URL is HTTP and resolves
to the loopback host (`127.0.0.1`, `localhost`, or `::1`). This is a hard
safety boundary: a test setting cannot redirect credentials or cookies to an
arbitrary host.

`cmd/hdu-testlab` starts a loopback-only HTTP server. A scenario is selected
with a command-line flag. The server returns a minimal but realistic sequence:

1. login page with CSRF token;
2. public RSA key payload;
3. successful login cookie or a controlled failure;
4. course-query warm-up and POST response;
5. personal-schedule response.

The server receives only a fictional test username and password. It does not
record request bodies or expose an endpoint outside the local machine.

`school.RunExportWithEndpoints` is an internal/test entry point that follows
the same `RunExport` status flow but takes validated loopback endpoints. The
normal GUI and EXE continue to call `RunExport` and cannot opt into test mode.

## Scenario Data

The test lab reuses `testdata/course.sample.json` and
`testdata/personal-schedule.sample.json` as the canonical successful data
set. The server returns their `items` in the shapes emitted by NewJW. This
keeps the data tested at the HTTP boundary consistent with scheduler and Smart
Agent tests.

Named scenarios:

| Scenario | Login | Course query | Personal schedule | Expected outcome |
| --- | --- | --- | --- | --- |
| `success` | succeeds | valid non-empty JSON | valid JSON | two local JSON files written |
| `bad-password` | login error page | not reached | not reached | login error, no output |
| `forbidden` | succeeds | permission response | not reached | query permission error |
| `malformed-course` | succeeds | invalid JSON | not reached | diagnosis file written |
| `empty-course` | succeeds | empty collection | not reached | diagnosis file written |
| `timeout` | succeeds | delayed beyond client timeout | not reached | timeout error |
| `personal-failure` | succeeds | valid JSON | permission response | course succeeds, personal error retained |

## Acceptance Workflow

`scripts/testlab-acceptance.ps1` will:

1. build or start `cmd/hdu-testlab` on an available loopback port;
2. run each exporter scenario in an isolated temporary directory;
3. validate status, output files, diagnostics, and that the server observed no
   non-fictitious credentials;
4. use the successful exported files as inputs to the scheduler worker smoke
   test and Smart Agent end-to-end test;
5. stop the mock server even on test failure.

The script reports each scenario independently so a protocol regression can be
identified without relying on a live school system.

## Error Handling and Safety

- Endpoint injection rejects HTTPS, non-loopback hosts, credentials in URLs,
  and missing endpoint URLs before any request is made.
- The mock server binds only to `127.0.0.1` and suppresses form/body logging.
- Each scenario uses a short controlled timeout; tests do not wait for the
  production 90-second client timeout.
- Temporary output is removed after assertions. User `course.json` and
  `personal-schedule.json` are never overwritten by the acceptance script.

## Verification

- Unit tests exercise endpoint validation and exporter flows against
  `httptest.Server`.
- The acceptance script exercises the compiled test-lab command plus the
  exporter and downstream project checks.
- Existing `go test`, `go vet`, scheduler worker smoke, Smart Agent E2E, and
  release smoke checks remain required before packaging.
