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

### Feature 03 — Student Management

Admin-only student records. These are separate from login accounts, because a student may never sign in.

- **Pages:**
  - Student list (`/students`) with real totals, search (name, student ID, phone, guardian), filters (status, gender, admission date range), sorting and pagination. The URL keeps the search, filters, sort order and page.
  - Profile page (`/students/:id`) with sections for personal, contact, parent, guardian, emergency contact and admission details.
  - A dedicated create/edit page with field-level validation.
- **Fields:**
  - Student ID: unique, case-insensitive, stored in upper case.
  - Name, date of birth, gender, optional blood group, optional phone and email, address.
  - Optional father's and mother's names and phones.
  - Guardian (required), emergency contact (required), admission date, status.
- **Photos:**
  - In the browser, the photo is cropped to a square and resized before upload. This also strips location data.
  - The server identifies the file type from its contents and accepts only JPEG, PNG or WebP up to 1 MB.
  - Photos are stored in PostgreSQL (`student_photos`) and served only to authorised users.
- **Deleting:** permanently removes the record and its photo, after the admin types the student ID to confirm.
- **Activity and dashboard:** every change is written to the activity log. The dashboard's student KPI, enrollment and recent admissions now come from student records.
- **Not included yet:** class and section placement, attendance and fees, and guardian portal links. Those future modules will reference `students.id`.

### Feature 04 — Teacher Management

Admin-only teacher records: profile, contact, qualifications, employment terms and an optional confidential salary.

- **Pages:**
  - Teacher list (`/teachers`) with real totals (total, active, full-time/part-time, joined this month), search (name, Teacher ID, subject, designation, phone, email), filters (status, employment type, designation, joining-date range), sorting and pagination.
  - Profile page (`/teachers/:id`) with sections for personal, contact, qualification and specialization, employment, emergency contact, login account and notes.
  - Create/edit pages.
- **Fields:**
  - Required: Teacher ID (unique, case-insensitive), name, gender, phone, designation, employment type (full-time or part-time), joining date, status.
  - Optional: date of birth, email, address, qualification, specialization, salary, emergency contact (name and phone must be given together), notes, photo.
- **Salary protection:**
  - The salary field appears in a teacher's details only for holders of the `teachers:view_salary` permission.
  - It never appears in lists, the dashboard or activity logs.
  - On the profile it's masked until revealed.
  - Saving without a salary field keeps the stored value, and only permitted users can change it.
- **Login account link:** a record can be linked to one existing login account with the Teacher role, reusing Feature 01's accounts rather than a second authentication system. Each account can be linked to only one record. This is checked inside a transaction that locks the user row and backed by a partial unique index. Deleting a teacher record keeps the login account.
- **Shared code:**
  - Photo validation and serving.
  - The avatar, photo picker, type-to-confirm delete dialog, form/detail sections and list URL state.
  - The admin pages are lazy-loaded per route.
- **Dashboard:** the Teachers KPI now counts teacher records.
- **Not included yet:** subject and class assignment, routines, attendance and payroll. Those future modules will reference `teachers.id`.

### Feature 05 — Academic Year Management

Admin-only academic years: plan sessions, choose the current year and keep completed years for records.

- **Pages:**
  - Academic year list (`/academic-years`) with real totals (total, current year with progress, upcoming, completed), search (name, description), status filter, sorting and pagination. Create and edit use a form dialog that suggests a name such as `2026–2027` from the dates.
  - Details page (`/academic-years/:id`) with overview, timeline progress, record history (created/updated by whom, when it was made current or completed) and a management-actions panel that shows why an action isn't available.
- **Data:** name (unique, case-insensitive), start and end date (both inclusive), status (`upcoming`, `active`, `completed`), optional description. `is_current` is a generated column (`status = 'active'`), so the flag and the status can never disagree.
- **Rules (enforced by the API and the database):**
  - Start date must be before the end date; a year can be at most 2 years long.
  - Date ranges may not overlap (PostgreSQL exclusion constraint); the API names the conflicting year.
  - Only one year can be current (partial unique index). Activation runs in one transaction behind an advisory lock: the previous current year becomes `completed` if it lies before the new year, or goes back to `upcoming` if it lies after it.
  - Years are never activated automatically. A new year starts as `upcoming`, or `completed` if its dates have already passed. A year whose end date has passed can't be made current.
  - The current year can be completed at any time; an upcoming year only after its dates have passed. Edits can't move an upcoming or current year's end into the past.
  - The current year can't be deleted. Future modules reference `academic_years.id` with `ON DELETE RESTRICT`; deleting a referenced year returns `409 ACADEMIC_YEAR_IN_USE`.
  - "Today" is evaluated in the school's timezone (`APP_TIMEZONE`).
- **Dashboard:** the academic year card shows the current year and its progress from the database, or a setup prompt when no year is current. A "Manage academic years" quick action and activity entries for academic year changes were added.
- **Not included yet:** classes, sections, subjects, enrollment and promotion.

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
| GET    | `/students`             | Admin (`students:read`)   | List students: `search`, `status`, `gender`, `admitted_from`, `admitted_to`, `sort`, `order`, `page`, `page_size` |
| GET    | `/students/stats`       | Admin (`students:read`)   | Total, active, inactive and admitted-this-month counts |
| GET    | `/students/:id`         | Admin (`students:read`)   | Student details |
| POST   | `/students`             | Admin (`students:create`) | Create a student |
| PUT    | `/students/:id`         | Admin (`students:update`) | Replace a student's details |
| DELETE | `/students/:id`         | Admin (`students:delete`) | Permanently delete a student and their photo |
| GET    | `/students/:id/photo`   | Admin (`students:read`)   | The stored photo (cached privately when requested with `?v=`) |
| PUT    | `/students/:id/photo`   | Admin (`students:update`) | Upload a photo (multipart form field `photo`) |
| DELETE | `/students/:id/photo`   | Admin (`students:update`) | Remove the photo |
| GET    | `/teachers`             | Admin (`teachers:read`)   | List teachers (never includes salary): `search`, `status`, `employment_type`, `designation`, `joined_from`, `joined_to`, `sort`, `order`, `page`, `page_size` |
| GET    | `/teachers/stats`       | Admin (`teachers:read`)   | Counts and distinct designations |
| GET    | `/teachers/linkable-accounts` | Admin (`teachers:update`) | Teacher-role accounts free to link (`search`, `teacher_id`) |
| GET    | `/teachers/:id`         | Admin (`teachers:read`)   | Teacher details; salary only with `teachers:view_salary` |
| POST   | `/teachers`             | Admin (`teachers:create`) | Create a teacher (optional `user_id` link and `salary`) |
| PUT    | `/teachers/:id`         | Admin (`teachers:update`) | Replace details; an omitted `salary` keeps the stored value |
| DELETE | `/teachers/:id`         | Admin (`teachers:delete`) | Delete the record and photo (the login account is kept) |
| GET/PUT/DELETE | `/teachers/:id/photo` | Admin | View, upload (multipart `photo`) or remove the photo |
| GET    | `/academic-years`       | Admin (`academic_years:read`) | List academic years: `search`, `status`, `sort` (`start_date`, `name`, `created_at`), `order`, `page`, `page_size` |
| GET    | `/academic-years/stats` | Admin (`academic_years:read`) | Counts per status and the current year |
| GET    | `/academic-years/:id`   | Admin (`academic_years:read`) | Academic year details, including which actions are currently allowed |
| POST   | `/academic-years`       | Admin (`academic_years:create`) | Create a year (`name`, `start_date`, `end_date`, `description`) |
| PUT    | `/academic-years/:id`   | Admin (`academic_years:update`) | Replace name, dates and description (status is not editable here) |
| PATCH  | `/academic-years/:id/activate` | Admin (`academic_years:manage_status`) | Make the year current; returns the demoted `previous` year |
| PATCH  | `/academic-years/:id/complete` | Admin (`academic_years:manage_status`) | Mark the year completed |
| DELETE | `/academic-years/:id`   | Admin (`academic_years:delete`) | Delete a year that is not current and not referenced |

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
