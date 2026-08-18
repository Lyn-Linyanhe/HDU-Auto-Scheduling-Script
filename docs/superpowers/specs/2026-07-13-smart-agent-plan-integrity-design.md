# Smart Agent Plan Integrity Gate

## Goal

Keep Smart Agent's plan preview useful while preventing a structurally invalid plan from becoming a `HDU-KillCourse/config.json` or progressing to dry-run and authorization.

## Severity Model

- `error`: the target cannot safely become an execution plan. It blocks generated configuration, writing configuration, dry-run, authorization, and execution package creation.
- `warning`: information is incomplete or needs manual attention. The plan remains usable but the issue is visible in the preview.
- `info`: non-blocking context.

## Error Checks

- The target contains duplicate teaching classes or more than one teaching class for the same base course.
- A target teaching class cannot be found in the current `course.json` when the course library is available.
- Target courses contain mixed terms or do not match the plan term.
- Two different target courses occupy overlapping day, period, and week slots.
- A locked current teaching class is absent from the target schedule.
- A plan with drops was not generated from the current `personal-schedule-live.json` snapshot, or that snapshot changed.

The time parser must recognize both `单周` / `双周` and the HDU export form `(单)` / `(双)`.

## Flow

1. Build and return the full action-plan preview with validation issues.
2. If it has errors, do not build a generated KillCourse config and do not write either the executable config or execution artifacts.
3. The UI keeps the plan visible, marks the configuration as blocked, and disables downstream controls.
4. Existing warnings remain non-blocking.

## Verification

- Unit tests cover same-time conflicts, odd/even non-conflicts, missing library teaching classes, mixed terms, and locked-target mismatches.
- API tests verify a blocked plan returns a preview but no generated/written config.
- Existing select-only and live-snapshot drop flows continue to pass.
