# E2E Test Backlog

This backlog captures the scenarios from `e2e_test_plan.md` that are still not fully covered by the current Playwright e2e suite.

## High priority

- [x] Public home page: verify the title, header, quiz tiles, and hamburger menu contents on `/`.
- [x] Public empty state: show `No upcoming quizzes yet.` and hide quiz tiles when no future quizzes exist.
- [x] Public setup error state: render the schema/import guidance when collections are missing or uninitialized.
- [x] Public HTMX navigation: click a quiz tile and verify `#quiz-content` updates without a full page reload.
- [x] Public navigation menu: verify Home, About Us, Contact Us, Privacy Policy, and Quiz Admin links.
- [x] Successful registration: submit a valid team, verify success feedback, reset state, and seats-left decrement.
- [x] Split-team success: verify multiple registrations are created, split sizes are balanced, and the form resets after success.
- [x] Merge consent yes path: verify small-team registrations can accept merge consent and store `willing_to_merge=true`.
- [x] Auto-close by capacity and time: verify full quizzes and quizzes within 1 hour of start are treated as closed.
- [x] Unregister confirmation and missing-record handling: show the confirmation page and safe not-found state.
- [x] Successful admin login: verify redirect and session behavior after logging in with a quiz admin account.
- [x] Admin header/menu contents: verify Dashboard, Public website, and Logout appear in the admin hamburger menu.

## Medium priority

- [x] Split prompt reset and validation: preserve values when split names are missing and reset fields after success.
- [x] Quiz date CRUD: create, update, delete, and invalid scheduled_at validation.
- [x] Quiz date admin dropdowns: verify the location field is a readable dropdown in create/edit modals.
- [x] Location invalid input: surface validation errors and keep the modal open for bad location forms.
- [x] Registration admin readable labels: show readable quiz labels and relation values instead of raw ids.
- [x] Registration admin merge preview: list selected records, same-quiz validation, and not-mergeable validation.
- [x] Registration admin merge defaults: verify fallback merged name/email behavior when fields are left blank.
- [x] Registration admin merge execution: verify the merged row appears and the old rows disappear.
- [x] Registration admin delete selected: verify selected rows are deleted and the count banner is shown.
- [x] Admin collection navigation stability: keep sidebar, modal, and URL state consistent after HTMX updates.
