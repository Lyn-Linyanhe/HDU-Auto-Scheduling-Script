# Smart Agent Live Schedule Sync Design

## Summary

HDU-Smart-Course-Agent must treat imported `personal-schedule.json` as a planning baseline only. Before a plan containing drop actions can be authorized for real execution, the agent must have a fresh current schedule snapshot from the academic system.

The first implementation uses a file bridge instead of embedding school login logic in Smart Agent. The exporter or the user provides a live schedule JSON, Smart Agent saves it as `personal-schedule-live.json`, compares it with `personal-schedule.json`, and writes `live-schedule-sync.json`.

## Safety Rule

- Plans without drop actions may proceed without live sync, but the UI and readiness checks show a warning.
- Plans with drop actions cannot pass dry-run or authorization unless:
  - `personal-schedule-live.json` exists and is parseable.
  - the current plan was generated from `personal-schedule-live.json`.
  - the plan current-schedule hash still matches the live snapshot on disk.
- If live sync happens after a plan was generated and the live schedule differs, the user must generate the plan again.

## Data Flow

1. User imports target schedule JSON.
2. Smart Agent builds a preliminary plan from `personal-schedule-live.json` if available, otherwise `personal-schedule.json`.
3. User imports or refreshes a live current schedule JSON.
4. Smart Agent writes:
   - `personal-schedule-live.json`
   - `live-schedule-sync.json`
5. User regenerates the plan.
6. Dry-run and authorization verify that drop plans are based on the live snapshot.

## Public Files

`personal-schedule-live.json` has the same course payload shape as `personal-schedule.json`.

`live-schedule-sync.json` contains:

- sync time
- local and live course counts
- local and live current schedule hashes
- courses added in live schedule
- courses removed from live schedule
- unchanged courses
- whether drift exists

## Current Boundary

This milestone does not execute real course selection or drop actions and does not store school credentials. Network login and academic-system scraping remain owned by the exporter/KillCourse side. Smart Agent only consumes the resulting live schedule snapshot.
