# Schedule Data Integrity Repair

## Goal

Fix the scheduler's handling of alternating-week classes and prevent a parseable but creditless `course.json` from producing misleading credit constraints and totals.

## Decisions

- Parse both the legacy `单周` / `双周` wording and the HDU export form `周(单)` / `周(双)`.
- A course payload is credit-capable when at least one item has a positive, parseable value in `xf`, `credits`, or `credit`.
- If `course.json` is parseable but creditless and a course Excel file exists in the same working directory, back up the incomplete JSON and regenerate it from Excel.
- If no Excel is available, retain the readable JSON so scheduling remains available, but disable credit bounds and render credits as unavailable instead of `0.00`.

## Data Flow

1. `EnsureCourseFile` reads `course.json`.
2. A credit-capable payload is used unchanged.
3. A creditless payload triggers a same-directory Excel lookup. When found, the old JSON is timestamp-backed up and the Excel conversion replaces it.
4. The browser independently detects whether normalized course data has credits, so imported or externally served data cannot make a misleading credit UI.

## Verification

- Unit tests cover `(单)` and `(双)` parsing, non-overlap of equivalent odd/even meetings, and overlap with all-week meetings.
- Tests cover Excel recovery from a creditless JSON, backup creation, and no overwrite when an existing JSON already has credits.
- Scheduler smoke tests assert creditless data is represented as unavailable and that normal decimal credits remain precise.
