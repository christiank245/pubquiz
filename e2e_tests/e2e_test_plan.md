# Pub Quiz E2E Test Plan

## Purpose
This plan covers the full browser-visible behavior of the pub quiz website:
- public discovery and registration
- split-team and merge-consent flows
- capacity and auto-close rules
- unregister flow
- quiz admin login, navigation, CRUD, merge, and delete workflows
- error and empty states

The goal is to verify the app as a user would experience it, not just the underlying handlers.

## Test environment
- Run the app with a fresh temporary PocketBase data directory.
- Ensure migrations seed baseline quiz data.
- Create a system admin account for setup.
- Create at least one quiz admin account in `quiz_admins`.
- Seed fixtures for:
  - one open quiz with ample capacity
  - one open quiz close to capacity
  - one quiz within 1 hour of start time
  - one closed quiz
  - at least two locations
  - registrations that can be merged later

## Coverage matrix

| Area | What must be verified |
| --- | --- |
| Public home | quiz tiles, empty state, setup error state, HTMX navigation |
| Quiz detail | location, maps link, seats left, registration panel, back link |
| Registration | required fields, invalid values, capacity checks, success state |
| Split teams | prompt, custom split names, final registration creation |
| Merge consent | prompt for teams under 4, yes/no handling |
| Auto-close | close when full, close within 1 hour of start |
| Unregister | confirmation page, delete action, missing record handling |
| Admin auth | login, invalid credentials, logout, route protection |
| Admin collections | sidebar, row click edit, create modal, delete modal |
| Locations | create, update, delete, readable values |
| Quiz dates | create, update, delete, relation dropdown, cascade delete |
| Registrations | edit, merge preview, merge execution, delete, readable relations |
| Error handling | missing params, invalid selections, invalid forms |

## Test cases

### Public landing page

#### E2E-PUB-01: Load the home page
- Open `/`.
- Verify the page title and public header render.
- Verify the header contains links to Quiz Admin and System Admin.
- Verify upcoming quiz cards are shown when quizzes exist.
- Verify each card shows:
  - quiz date/time
  - location name
  - seats remaining
  - register call to action

#### E2E-PUB-02: Show empty-state messaging
- Use a fixture with no open future quizzes.
- Open `/`.
- Verify the page shows `No upcoming quizzes yet.`
- Verify no quiz tiles render.

#### E2E-PUB-03: Show setup error when collections are missing
- Start the app with missing or uninitialized schema data.
- Open `/`.
- Verify the setup error banner is shown.
- Verify the message instructs the user to import the schema and create locations/quiz dates.

#### E2E-PUB-04: Open a quiz via HTMX
- Open `/`.
- Click a quiz tile.
- Verify the content swaps into `#quiz-content`.
- Verify the browser URL changes to `/quiz?id=...`.
- Verify the registration panel is visible without a full page reload.

### Quiz detail page

#### E2E-QUIZ-01: Load the quiz detail page directly
- Open `/quiz?id=<quizId>`.
- Verify the page shows the quiz registration heading.
- Verify the selected quiz location and Google Maps link are visible.
- Verify seats left are shown.
- Verify the back link to all quiz dates is visible.

#### E2E-QUIZ-02: Reject missing quiz id
- Open `/quiz` without an id.
- Verify the request fails with a bad request style error.

#### E2E-QUIZ-03: Reject closed or unknown quizzes
- Open `/quiz?id=<unknown-or-closed-id>`.
- Verify the app does not show the registration form.
- Verify the user sees an error or closed-quiz response.

### Normal registration

#### E2E-REG-01: Register a valid team
- Open a quiz with enough capacity.
- Fill email, team name, and team size between 1 and 10.
- Submit the form.
- Verify success feedback appears.
- Verify the form resets after success.
- Verify the seats-left counter updates.

#### E2E-REG-02: Require all registration fields
- Submit the form with one or more empty fields.
- Verify the inline error says all fields must be filled.
- Verify the form stays visible and preserves entered values.

#### E2E-REG-03: Reject invalid team size values
- Submit a non-numeric team size.
- Submit a team size of 0.
- Submit a negative or blank value if the UI allows it.
- Verify the validation message requires a number greater than 0.

#### E2E-REG-04: Reject teams larger than remaining seats
- Create a quiz with a known seat count.
- Attempt to register a team larger than the remaining seats.
- Verify the app returns the capacity error with the exact seat count.
- Verify no registration is created.

#### E2E-REG-05: Preserve form state on registration errors
- Trigger a validation error.
- Verify the email, team name, and team size fields remain populated.
- Verify the user can correct the values without retyping everything.

### Split-team flow

#### E2E-SPLIT-01: Show split prompt for teams over 10
- Submit a team size above 10.
- Verify the split prompt appears.
- Verify the prompt explains the max-team-size rule.
- Verify the prompt lists the generated split sizes.
- Verify split team name inputs are rendered with defaults.

#### E2E-SPLIT-02: Confirm split and create multiple registrations
- Start with a team size above 10.
- Accept the split prompt.
- Fill each split team name.
- Submit the confirmation.
- Verify success feedback says the group was split.
- Verify the resulting registrations are created as separate rows.
- Verify no split team exceeds 10 people.

#### E2E-SPLIT-03: Reject split confirmation until all split names are provided
- Trigger the split prompt.
- Leave one split team name blank.
- Submit the split form.
- Verify the app shows the missing-name error.
- Verify the split prompt remains visible with the entered values preserved.

#### E2E-SPLIT-04: Verify split sizes are balanced
- Test several oversized team sizes, such as 11, 20, and 21.
- Verify the generated sizes add up to the original team size.
- Verify no generated size exceeds 10.
- Verify the distribution is as even as possible.

#### E2E-SPLIT-05: Verify split success resets the form
- Complete a split registration.
- Verify the form clears email and team name after success.
- Verify the team size resets to 1.
- Verify the seats-left value decreases by the full original team size.

### Small-team merge-consent flow

#### E2E-MERGE-01: Show merge prompt for teams under 4
- Submit a team size of 1, 2, or 3.
- Verify the merge prompt appears.
- Verify the prompt explains that small teams may be merged.
- Verify both answer buttons are visible.

#### E2E-MERGE-02: Accept merge consent
- Choose the merge-consent yes option.
- Verify the registration succeeds.
- Verify the resulting registration is stored with willing_to_merge enabled.

#### E2E-MERGE-03: Decline merge consent
- Choose the no option.
- Verify the registration succeeds.
- Verify the registration is stored with willing_to_merge disabled.

#### E2E-MERGE-04: Reject invalid merge consent input
- Submit an unexpected merge-consent value.
- Verify the app shows the merge-choice validation message.
- Verify the prompt remains on screen.

### Capacity and auto-close behavior

#### E2E-CAP-01: Close a quiz when it becomes full
- Register enough seats to exhaust the quiz capacity.
- Verify the success response appears.
- Refresh the home page.
- Verify the quiz disappears from the open quiz list.
- Verify the quiz is now treated as closed.

#### E2E-CAP-02: Auto-close a quiz within one hour of start
- Seed a quiz scheduled within 1 hour.
- Open the home page.
- Verify the quiz is not listed as open.
- Attempt direct registration.
- Verify the quiz is closed.

#### E2E-CAP-03: Keep a quiz open when more than one hour remains
- Seed a quiz with more than 1 hour before start and available seats.
- Open the home page.
- Verify the quiz remains visible.
- Verify registration is still allowed.

### Unregister flow

#### E2E-UNREG-01: Show unregister confirmation
- Register a team.
- Visit `/unregister?id=<registrationId>`.
- Verify the unregister confirmation page shows:
  - team name
  - team size
  - registration email
- Verify the confirmation button is visible.

#### E2E-UNREG-02: Complete unregister
- From the confirmation page, submit the unregister form.
- Verify the success message is shown.
- Verify the confirmation state disappears.
- Verify the registration no longer exists in the admin table.

#### E2E-UNREG-03: Handle missing or deleted registrations
- Visit `/unregister?id=<unknown-id>`.
- Verify the app shows the not-found or already-removed message.
- Verify the page offers a safe return link to the start page.

### Global layout and navigation

#### E2E-LAYOUT-01: Verify the public header
- Open the home page and quiz page.
- Verify the public header shows the app name and summary text.
- Verify the Quiz Admin and System Admin links are visible.

#### E2E-LAYOUT-02: Verify the admin header
- After login, open any admin page.
- Verify the admin header shows Quiz Admin.
- Verify the Public website link is visible.
- Verify the Logout button is visible.

### Admin login and authorization

#### E2E-ADMIN-01: Show login page for protected routes
- Open `/quiz-admin` without a quiz-admin cookie.
- Verify the login page renders.

#### E2E-ADMIN-02: Reject invalid login
- Submit an incorrect email or password.
- Verify the login error message is shown.
- Verify the user stays on the login page.

#### E2E-ADMIN-03: Accept valid login and redirect
- Log in with a quiz admin account.
- Verify the app redirects to the admin collection view.
- Verify the session cookie is set.

#### E2E-ADMIN-04: Logout clears the session
- Log in successfully.
- Click Logout.
- Verify the app returns to the login page.
- Verify protected routes are no longer accessible without login.

#### E2E-ADMIN-05: Reject unauthorized direct collection access
- Open a collection URL without logging in.
- Verify the app redirects to login.

### Admin collection navigation

#### E2E-COL-01: Show only managed collections
- Open the admin home after login.
- Verify the sidebar lists only non-system, non-auth collections.
- Verify `quiz_admins`, `users`, and other protected system collections are hidden.

#### E2E-COL-02: Navigate between collections
- Click different sidebar entries.
- Verify the content swaps in place.
- Verify the browser URL updates.
- Verify the active collection is highlighted.

#### E2E-COL-03: Verify human-readable relation labels
- Open collections with relation fields.
- Verify relation values are rendered as labels instead of raw ids where possible.
- Verify `quiz_dates` rows show a readable date/time label.
- Verify `registrations` rows show a readable quiz label.

#### E2E-COL-04: Verify row click opens edit modal
- Click a row value or record id link.
- Verify the edit modal opens.
- Verify the record id is shown as read-only context, not editable.

#### E2E-COL-05: Verify create modal opens and closes
- Click Add entry.
- Verify the create modal opens.
- Close it using the Close control.
- Verify the modal disappears and the collection view remains usable.

### Locations CRUD

#### E2E-LOC-01: Create a location
- Open the locations collection.
- Use Add entry to create a location.
- Fill name, maps URL, and capacity.
- Submit.
- Verify the success banner appears.
- Verify the new location appears in the table.

#### E2E-LOC-02: Edit a location
- Open an existing location row.
- Change capacity or other editable values.
- Save changes.
- Verify the success banner appears.
- Verify the table reflects the updated values.

#### E2E-LOC-03: Reject invalid location input
- Submit a blank required field or invalid capacity.
- Verify the form shows the validation error.
- Verify the modal stays open with the entered values preserved.

#### E2E-LOC-04: Delete a location
- Select one or more locations.
- Open the delete confirmation modal.
- Confirm deletion.
- Verify the deleted row disappears.
- Verify the success banner states how many entries were deleted.

### Quiz date CRUD

#### E2E-QUIZADMIN-01: Create a quiz date
- Open the quiz_dates collection.
- Click Add entry.
- Fill scheduled_at and choose a location from the dropdown.
- Submit.
- Verify the new row appears with a readable German-style date label.

#### E2E-QUIZADMIN-02: Edit a quiz date
- Open an existing quiz date row.
- Update the scheduled time or location.
- Save changes.
- Verify the table updates with the new human-readable relation label.

#### E2E-QUIZADMIN-03: Reject invalid date-time input
- Enter an invalid scheduled_at value.
- Verify the app shows a date-time validation error.
- Verify the modal remains open.

#### E2E-QUIZADMIN-04: Delete a quiz date and its registrations
- Create a quiz date with one or more registrations.
- Delete the quiz date from admin.
- Verify the quiz date is removed.
- Verify linked registrations are also removed.

#### E2E-QUIZADMIN-05: Verify quiz-date fields use relation dropdowns
- Open the quiz date edit/create modal.
- Verify location is a dropdown, not a raw id input.
- Verify the options are readable location labels.

### Registrations CRUD and merge

#### E2E-REGADMIN-01: View registrations with readable quiz labels
- Open the registrations collection.
- Verify each row shows the related quiz label, team name, team size, email, and willing-to-merge status.

#### E2E-REGADMIN-02: Edit a registration
- Open a registration row.
- Update team name, email, team size, or merge preference as allowed.
- Save changes.
- Verify the row updates.

#### E2E-REGADMIN-03: Reject invalid registration edits
- Enter an invalid team size or required field.
- Verify the admin form surfaces the validation error.
- Verify the modal remains open.

#### E2E-REGADMIN-04: Enable and disable merge selection button
- Select zero rows.
- Verify Merge entries is disabled.
- Select one eligible row.
- Verify Merge entries stays disabled.
- Select at least two eligible rows from the same quiz.
- Verify Merge entries becomes enabled.

#### E2E-REGADMIN-05: Show merge preview modal
- Select two eligible registrations.
- Open the merge modal.
- Verify the modal summarizes the selected entries.
- Verify team names, sizes, emails, and registration ids are listed.

#### E2E-REGADMIN-06: Reject merge selection from different quizzes
- Select registrations from different quiz dates.
- Open the merge modal.
- Verify the modal shows a same-quiz validation error.

#### E2E-REGADMIN-07: Reject merge selection when a row is not mergeable
- Select at least one registration with willing_to_merge disabled.
- Open the merge modal.
- Verify the modal shows the merge-consent validation error.

#### E2E-REGADMIN-08: Merge registrations with custom values
- Select 2+ eligible registrations from the same quiz.
- Open the merge modal.
- Provide a custom team name and email.
- Submit the merge.
- Verify the merged record appears in the table.
- Verify the old rows are removed.
- Verify the merged row has the combined team size.

#### E2E-REGADMIN-09: Merge registrations with default values
- Open the merge modal.
- Leave team name and email blank.
- Submit the merge.
- Verify the app falls back to a sensible merged team name and email source.

#### E2E-REGADMIN-10: Delete selected registrations
- Select one or more registration rows.
- Open the delete confirmation modal.
- Confirm deletion.
- Verify the selected rows disappear.
- Verify the success message reports the deleted count.

#### E2E-REGADMIN-11: Close the merge modal without merging
- Open the merge modal.
- Click Close.
- Verify the modal disappears.
- Verify no records are changed.

### Error and edge-state handling

#### E2E-ERR-01: Handle no selection for delete
- Open any admin collection.
- Trigger delete without selecting rows.
- Verify the app shows the select-at-least-one error.

#### E2E-ERR-02: Handle malformed admin form input
- Submit bad data through create or update forms.
- Verify validation errors are surfaced in-page, not swallowed.

#### E2E-ERR-03: Keep collection navigation stable after HTMX updates
- Navigate via sidebar and row links.
- Verify the correct content area updates.
- Verify the modal and sidebar state remain consistent after replacement.

## Expected assertions per test
For each case, assert the following where relevant:
- visible text matches the app copy
- the right HTMX target is updated
- URL changes when push-url is expected
- success and error banners appear in the correct place
- records are created, updated, or deleted in PocketBase as expected
- no forbidden collections appear in the admin UI
- all human-readable labels remain readable, not raw ids

## Suggested priority order
1. public home and quiz registration
2. split and merge flows
3. unregister flow
4. admin login and navigation
5. collection CRUD
6. registration merge and delete actions
7. error-state and edge-case coverage

## Notes
- Keep test data isolated per run.
- Prefer stable selectors already present in the UI such as ids, form field names, and button text.
- Treat the public and admin flows as separate smoke paths, but share fixture setup where possible.
