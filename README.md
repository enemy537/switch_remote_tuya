# Switch Remote

A small web app for turning Tuya Wi-Fi smart switches on and off. Built for up to 50 users and 1000 devices.

- Users pair their switch in the Smart Life mobile app, then register it here and control it from the browser.
- An admin creates users, resets passwords and monitors every device.

To deploy or operate it, follow the [runbook](docs/RUNBOOK.md).

## How it works

```
Browser ── CloudFront ─┬─ /*      → S3 (private)             Vue SPA     front/
                       └─ /api/*  → Lambda Function URL      Go API      back/
                                        ├─ Neon Postgres     users, devices
                                        └─ Tuya OpenAPI      device list, state, commands
```

1. A user pairs a switch with the Smart Life app. A browser cannot do this step: Tuya pairing and LAN discovery use UDP, which web pages have no access to.
2. The admin links that user's Smart Life account to the Tuya cloud project and copies the account's Tuya UID.
3. The admin creates the user in the app with email, password and Tuya UID.
4. The user logs in, opens "Add device", and registers a switch from their Smart Life account.
5. Toggling a switch sends a command from the Lambda to Tuya's cloud, which relays it to the device.

## Components

### `front/` — web UI

Vue 3 single-page app built with Vite. Plain JavaScript and one hand-written stylesheet; no state library or UI kit.

| File | Role |
|---|---|
| `src/api.js` | Fetch wrapper and one function per API endpoint. Sends the user to the login page on any 401. |
| `src/router.js` | Routes, the current-user ref, and the guard that protects pages and the admin area. |
| `src/views/Login.vue` | Email and password form. |
| `src/views/Devices.vue` | The user's switches: toggle, remove, and the "Add device" panel. |
| `src/views/Admin.vue` | User list, create user, reset password, and the paged list of all devices. |

The router uses hash URLs (`/#/admin`). CloudFront's page-not-found fallback applies to a whole distribution and would replace API 404 responses with the index page, so hash routing avoids needing one.

### `back/` — API

One Go binary running as a single AWS Lambda (`provided.al2023`, arm64). Routing uses the standard library.

| Package | Role |
|---|---|
| `cmd/api` | Wires everything together. Runs as a Lambda in AWS and as a plain HTTP server on port 8080 locally. |
| `internal/auth` | Password hashing (bcrypt), the session token (JWT in an HttpOnly cookie), and the login and admin checks. |
| `internal/store` | Postgres queries for users and devices. |
| `internal/tuya` | Signed client for Tuya OpenAPI: access token, device list, device state, switch command. |
| `internal/httpapi` | HTTP handlers and routes. |
| `migrations/` | The database schema, applied by hand with `psql`. |

On/off and online state are never stored. They are read from Tuya on each request, so there is nothing to keep in sync.

### `infra/` — AWS resources

Terraform with local state. It creates:

- a private S3 bucket for the UI, readable only by CloudFront;
- the Lambda, its IAM role, a log group with 14-day retention, and a public Function URL;
- one CloudFront distribution that serves the UI and forwards `/api/*` to the Lambda.

The UI and API share one origin, so there is no CORS setup. Secrets are passed to the Lambda as environment variables from `terraform.tfvars`. The Neon database and the Tuya project are created by hand and are not managed by Terraform.

## API

All bodies are JSON. Errors return a non-2xx status with `{"error": "message"}`. The session is a cookie named `session`.

| Method and path | Purpose |
|---|---|
| `POST /api/login` | Log in with `{email, password}`. |
| `POST /api/logout` | Log out. |
| `GET /api/me` | The current user. |
| `GET /api/tuya-devices` | Switches in the user's Smart Life account that are not registered yet. |
| `GET /api/devices` | The user's registered switches with live state. |
| `POST /api/devices` | Register a switch with `{id, name}`. It must belong to the user's Smart Life account. |
| `DELETE /api/devices/{id}` | Remove a switch. |
| `POST /api/devices/{id}/switch` | Turn on or off with `{on: true}` or `{on: false}`. |
| `GET /api/admin/users` | List users (admin). |
| `POST /api/admin/users` | Create a user with `{email, password, tuya_uid, is_admin}` (admin). |
| `PATCH /api/admin/users/{id}` | Set a new password with `{password}` (admin). |
| `GET /api/admin/devices?page=1` | All devices across users, 20 per page, with live state (admin). |

## Data

| Table | Columns |
|---|---|
| `users` | `id`, `email`, `password_hash`, `is_admin`, `tuya_uid`, `created_at` |
| `devices` | `id` (the Tuya device ID), `user_id`, `name`, `created_at` |

## Known limits

- **Single-channel switches only.** On/off uses the Tuya code `switch_1`, defined once in `back/internal/tuya`. A switch that reports a different code needs that constant changed.
- **No self-service signup or password reset.** The admin does both.
- **One manual step per user.** Each Smart Life account is linked in the Tuya console by scanning a QR code.
