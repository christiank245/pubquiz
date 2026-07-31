# E2E Test Backlog

This backlog captures the scenarios from `e2e_test_plan.md` that are not fully covered by the current Playwright e2e suite.

## High priority

- [x] Public home empty state: show `No upcoming quizzes yet.` when no open future quizzes exist.
- [x] Public setup error state: render the schema/import guidance when the app starts with missing or uninitialized data.
- [x] Informational pages: verify `/about`, `/contact`, and `/privacy` render their headings/body copy and keep the public header menu.
- [x] Direct quiz detail page: verify `/quiz?id=<quizId>` shows the quiz heading, location, maps link, seats left, and back link.
- [x] Missing quiz id: reject `/quiz` without an id.
- [x] Closed or unknown quiz: reject `/quiz?id=<unknown-or-closed-id>` without showing the registration form.
- [x] Required registration fields: show the inline error when email, team name, or team size is missing.
- [x] Invalid team size: reject non-numeric, zero, negative, or blank team size values.
- [x] Capacity overflow: reject teams larger than the remaining seats with the exact seat count error.
- [x] Registration error recovery: preserve entered values after validation errors.
- [x] Unregister flow: show confirmation, complete unregister, and handle missing/deleted registrations safely.
- [x] Admin logout: log out through the admin menu and confirm protected routes become inaccessible again.
- [x] Admin route protection: verify direct collection URLs redirect to login when unauthenticated.

## Medium priority

- [x] Split prompt details: verify the split prompt explains the max-team-size rule and shows generated split sizes.
- [x] Split confirmation validation: require every split team name before submission.
- [x] Split balance rules: verify generated split sizes add up correctly and stay as even as possible for sizes like 11, 20, and 21.
- [x] Split success reset: confirm email/team name clear, team size resets to 1, and seats-left updates by the full original size.
- [ ] Merge consent decline: confirm the no option stores `willing_to_merge=false`.
- [ ] Merge consent validation: reject unexpected merge-consent input.
- [ ] Auto-close by capacity: verify a quiz closes and disappears from the open list when it becomes full.
- [ ] Auto-close by time: verify a quiz within 1 hour of start is treated as closed.
- [ ] Keep-open rule: verify a quiz with more than 1 hour remaining stays open.
- [ ] Admin sidebar filtering: ensure system/auth collections are hidden and only managed collections are shown.
- [ ] Human-readable relation labels: verify admin tables show readable labels instead of raw ids where possible.
- [ ] Row click edit modal: verify clicking a row opens the edit modal with read-only record context.
- [ ] Create modal lifecycle: verify the create modal opens and closes cleanly.
- [ ] Invalid admin form input: surface validation errors in-page for malformed location/quiz date/registration edits.
- [ ] Quiz date relation dropdown: verify location uses a dropdown with readable labels.
- [ ] Quiz date cascade delete: verify registrations are removed when a quiz date is deleted.
- [ ] Registration merge selection state: disable merge unless at least two eligible rows from the same quiz are selected.
- [ ] Merge preview modal: show selected team details before merging.
- [ ] Merge validation: reject selections from different quizzes or rows that are not mergeable.
- [ ] Merge default values: verify fallback merged team name/email behavior when fields are left blank.
- [ ] Delete-without-selection: show the select-at-least-one error.
- [ ] HTMX navigation stability: confirm sidebar/row updates keep the correct content, modal state, and URL after replacements.

## Low priority

- [ ] Admin collection CRUD edge states: verify delete confirmation modals and success counts for all managed collections.
- [ ] Readable admin record labels: confirm registrations and quiz dates remain readable after update/delete operations.
