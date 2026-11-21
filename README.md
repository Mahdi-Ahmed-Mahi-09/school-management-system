# Classora — School Management System

Classora is a full-stack school management platform for **administrators, teachers, students and guardians**.

| Layer    | Stack |
| -------- | ----- |
| Frontend | React 19, Vite, Tailwind CSS v4, React Router, Axios, React Hook Form, Lucide |
| Backend  | Go, Gin, pgx (PostgreSQL), JWT, bcrypt |
| Database | PostgreSQL 14+ |

## Features

### Feature 01 — Authentication & User Management

- **Sign-in** with email or username, a show/hide password toggle and an optional *Keep me signed in* setting (7 days instead of a browser session).
- **Sessions** are JWTs stored in an `HttpOnly`, `SameSite=Strict` cookie, so JavaScript never sees the token.
  - Logout revokes the token on the server.
  - A password change or admin reset signs the user out on every other device.
  - Deactivating an account ends that user's active sessions immediately.
- **Four roles:** `admin`, `teacher`, `student` and `guardian`.
  - Permissions are defined once, in `backend/internal/models/permissions.go`.
  - The API enforces them with role- and permission-based middleware; the UI only mirrors them.
- **Admin user management:**
  - List users with search, role and status filters, sorting and pagination.
  - See account statistics.
  - Create and edit users, assign roles, activate or deactivate accounts, and reset passwords (optionally forcing a change at next sign-in).
- **Self-service profile:** users can edit their name and phone number and change their password. Email, username, role and status can only be changed by an admin.
- **Safeguards:**
  - Admins can't change their own role or status.
  - The last active admin can't be demoted or deactivated (enforced with row locks).
  - Restricted fields sent to an endpoint are rejected rather than silently ignored.

### Feature 02 — Admin Dashboard

An admin-only overview at `/`, backed by `GET /api/v1/admin/dashboard`.

- **Real data today:**
  - Student, teacher and guardian totals, with active and new-this-month counts.
  - Accounts by role (donut chart).
  - New accounts per month for the last six months (stacked bar chart).
  - A recent activity feed.
- **Not available yet:** classes, sections, academic year, enrollment, class distribution, today's attendance, fee collection, recent admissions and recent notices.
  - Each of these sections comes back from the API with `"status": "unavailable"` and the module that will provide it. The UI says it isn't set up yet rather than showing a fake zero.
  - The response format for each section is already defined in `backend/internal/models/dashboard.go`. When a later feature implements its module, it fills in that section.
- **Activity log:** a new `activity_logs` table records admin actions on users (create, edit, role change, activation and deactivation, password reset).
  - The migration backfilled one "account created" event for each existing account, from its real creation time.
- **Quick actions:**
  - Links to features that exist, for example "Add student account" opens the create form with the Student role already selected.
  - Actions for future modules are shown disabled. No placeholder pages.
- **Interface:** loading skeletons, error state with retry, manual refresh, and a responsive layout from mobile to wide desktop. The charting library (Recharts) is only downloaded by admins.

## Getting started

### 1. Database

Create an empty database:

```sql
CREATE DATABASE classora;
```

### 2. Backend

```bash
cd backend
cp .env.example .env          # then set DATABASE_URL and JWT_SECRET
go run ./cmd/migrate up       # apply migrations (also runs automatically in development)
go run ./cmd/seed             # optional: create demo accounts (refuses to run in production)
go run ./cmd/server           # API on http://localhost:8080
```

### 3. Frontend

```bash
cd frontend
npm install
npm run dev                   # http://localhost:5173 (proxies /api to the backend)
```

### Demo accounts (development only)

These accounts are created by `go run ./cmd/seed`:

| Role     | Email                   | Username   | Password         |
| -------- | ----------------------- | ---------- | ---------------- |
| Admin    | `admin@classora.dev`    | `admin`    | `Admin@12345`    |
| Teacher  | `teacher@classora.dev`  | `teacher`  | `Teacher@12345`  |
| Student  | `student@classora.dev`  | `student`  | `Student@12345`  |
| Guardian | `guardian@classora.dev` | `guardian` | `Guardian@12345` |

The seed command never overwrites existing accounts. Change these passwords, or don't seed at all, on any shared deployment.

## API

Base URL: `/api/v1`. Every response uses one envelope:

```jsonc
{ "success": true,  "data": { /* ... */ }, "meta": { /* pagination, when relevant */ } }
{ "success": false, "error": { "code": "VALIDATION_FAILED", "message": "...", "details": { "email": "..." } } }
```

Every `POST`, `PUT`, `PATCH` and `DELETE` request must include the header `X-Requested-With: XMLHttpRequest`. This is the CSRF guard.

| Method | Endpoint                | Access            | Description |
| ------ | ----------------------- | ----------------- | ----------- |
| GET    | `/health`               | Public            | Health check (includes the database) |
| POST   | `/auth/login`           | Public, rate limited | Sign in with `identifier`, `password` and `remember_me`; sets the session cookie |
| POST   | `/auth/logout`          | Public            | Revokes the session and clears the cookie |
| GET    | `/auth/me`              | Signed in         | Current user, their permissions and session info |
| GET    | `/roles`                | Signed in         | Every role and its permissions |
| GET    | `/profile`              | `profile:read`    | Own profile |
| PATCH  | `/profile`              | `profile:update`  | Update `first_name`, `last_name` and `phone` |
| PUT    | `/profile/password`     | `password:change` | Change own password; rotates the session |
| GET    | `/users`                | Admin             | List users: `search`, `role`, `status`, `sort`, `order`, `page`, `page_size` |
| GET    | `/users/stats`          | Admin             | Account counts |
| GET    | `/users/:id`            | Admin             | User details |
| POST   | `/users`                | Admin             | Create a user |
| PATCH  | `/users/:id`            | Admin             | Update `username`, `email`, names and `phone` |
| PATCH  | `/users/:id/role`       | Admin             | Assign a role |
| PATCH  | `/users/:id/status`     | Admin             | Activate or deactivate an account |
| PUT    | `/users/:id/password`   | Admin             | Reset a password; signs the user out everywhere |
| GET    | `/admin/dashboard`      | Admin (`dashboard:view_admin`) | Dashboard data; each section is marked `available` or `unavailable` |

## Project structure

```
backend/
  cmd/server            API entry point
  cmd/migrate           Migration CLI (up | down | status)
  cmd/seed              Demo account seeder
  internal/config       Environment configuration
  internal/database     pgx pool and embedded SQL migrations
  internal/models       Domain types, roles and permissions
  internal/repository   Data-access interfaces and PostgreSQL implementations
  internal/service      Business logic (auth, users, profile)
  internal/handler      HTTP handlers
  internal/middleware   Authentication, RBAC, CSRF, rate limiting, security headers, CORS
  internal/router       Route wiring
  internal/validation   Input normalisation and validation
frontend/src/
  api/                  Axios client and API services
  context/, hooks/      Authentication state
  routes/               Route guards (auth, permission, guest-only)
  layouts/              App shell with sidebar navigation
  components/ui/        Design-system components
  components/users/     User-management modals and shared actions
  pages/                Login, dashboard, profile, admin users, error pages
```

## Testing

```bash
cd backend
go test ./...                                          # unit tests and HTTP tests (in-memory repositories)
TEST_DATABASE_URL=postgres://... go test ./internal/repository/   # PostgreSQL integration test

cd frontend
npm run lint
npm run build
```

The PostgreSQL integration test only creates and deletes its own uniquely tagged rows. Even so, run it against a separate test database.
