# Pub Quiz Signup Website

PocketBase-powered pub quiz registration website using:
- HTML templates (Go templating)
- Tailwind CSS
- HTMX

## Features

- Landing page lists upcoming pub quiz dates.
- Auto-seeded demo data (locations + upcoming quiz dates) via migrations for quick testing.
- Quizzes auto-close when full or when they are within 1 hour of the start time.
- Quiz detail page includes:
  - date/time
  - location
  - Google Maps link
  - registration form (email, team name, team size)
- Team size rules:
  - max 10 people per team
  - for larger groups, users are asked to confirm an automatic split into evenly distributed teams (all teams <= 10)
  - users can set custom team names for each split team before confirming
- Hidden unregister flow via direct endpoint:
  - `GET /unregister?id=<registrationId>` (confirmation page)
  - `POST /unregister` with `registration_id` (removes registration)
- Capacity-aware registration (prevents overbooking).
- Admin data management via PocketBase Admin UI (`/_/`):
  - `locations` (name, maps_url, capacity)
  - `quiz_dates` (scheduled_at, location, is_open)
  - `registrations` (quiz, email, team_name, team_size)

## Quick start

1. Install dependencies:
   ```bash
   make deps
   ```
2. Build CSS:
   ```bash
   make css
   ```
3. Run PocketBase app:
   ```bash
   make run
   ```
4. Open admin UI:
   - `http://127.0.0.1:8090/_/`
   - Create your first admin user when prompted.
5. The app auto-bootstraps required collections via PocketBase migrations on startup.
6. Add locations and quiz dates in admin UI.
7. Open landing page:
   - `http://127.0.0.1:8090/`

## Tests

```bash
make test
```

Test stack: Ginkgo + Gomega.

## Build single binary (embedded templates + static assets)

```bash
make build
```

Binary output:
- `bin/pubquiz`

Run the binary:
```bash
./bin/pubquiz serve
```
