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
- **Not included yet:** subjects, enrollment and promotion.

### Feature 06 — Class & Section Management

Admin-only classes and sections. A **class** is a reusable level (Play, KG, Class 6 …); its **sections** belong to one academic year, so Class 6 can have sections A–B in 2026–2027 and A–C in 2027–2028.

- **Pages:**
  - Classes (`/classes`): real totals (classes, active, inactive, sections), search (name, code, description), status filter, sorting, pagination, section counts for the current year, and a reorder dialog that saves the whole order atomically.
  - Class details (`/classes/:id`): class information and its sections grouped by academic year.
  - Sections (`/sections`): always scoped to one academic year (defaulting to the current year), with a class filter, search and status filter. It handles "no academic years" and "no current year" explicitly.
  - Section details (`/sections/:id`): section, class and academic year information.
- **Data:**
  - Class: name (unique, case-insensitive), optional code (unique), display order, description, status.
  - Section: class, academic year, name (unique within the class and year, case-insensitive), optional code (unique within the year), room, capacity (1–1000), description, status.
- **Rules (enforced by the API and the database):**
  - Sections need a valid class and academic year. New sections can only be added to an active class in the current or an upcoming year; moving a section follows the same rule. Sections in completed years can still be corrected in place.
  - Deactivating a class keeps its sections, but none can be added or re-activated until the class is active again.
  - Section writes share-lock the parent class and year in the same transaction, so they can't race a class deactivation or a year being completed.
  - Foreign keys use `ON DELETE RESTRICT`: a class with sections (`CLASS_IN_USE`) or an academic year with sections (`ACADEMIC_YEAR_IN_USE`) can't be deleted. Sections referenced by future modules return `SECTION_IN_USE`.
- **Dashboard:** the Total classes and Total sections KPIs are real. Sections are counted for the current academic year. "Students per class" stays unavailable until Student Enrollment exists.
- **Not included yet:** teacher assignment, student enrollment and student counts. Future modules will reference `classes.id` / `sections.id`.

### Feature 07 — Subject Management

Admin-only subject catalogue and class-subject assignments. A **subject** (Bangla, Mathematics, ICT …) is created once; it is **assigned to a class for one academic year**, as compulsory or optional for that class. Class 6 can take Bangla, English and Mathematics in 2026–2027, while Class 9 takes Physics, Chemistry and Biology.

- **Pages:**
  - Subjects (`/subjects`): real totals (subjects, active, inactive, class assignments), search (name, code, description), type and status filters, sorting, pagination. Row actions: view, edit, assign to class, activate/deactivate, and delete (only offered when the subject has no assignments).
  - Subject details (`/subjects/:id`): subject information, marks and the classes it is assigned to, grouped by academic year.
  - Class subjects (`/class-subjects`): always scoped to one academic year (defaulting to the current year), with class, subject, type and search filters. Admins assign several subjects at once, switch an assignment between compulsory and optional, and remove assignments.
  - Class details (`/classes/:id`) also lists the class's subjects by academic year.
- **Data:**
  - Subject: name (unique, case-insensitive), code (required, unique), description, type (compulsory/optional, the default for new assignments), optional full marks (1–1000) and pass marks (0 up to the full marks), status.
  - Class subject (`class_subjects`): class, subject, academic year, compulsory flag. Unique per class, academic year and subject.
- **Rules (enforced by the API and the database):**
  - Assignments need a valid class, subject and academic year. New assignments need an active class, active subjects, and the current or an upcoming year. A request assigning several subjects is all-or-nothing: duplicates (`CLASS_SUBJECT_EXISTS`, naming the subjects) or inactive subjects (`SUBJECT_INACTIVE`) save nothing.
  - Assigning share-locks the class, year and subjects in the same transaction, so it can't race a deactivation, a year being completed or a delete.
  - Changing a subject's type doesn't change existing assignments; each keeps its own compulsory flag.
  - Deactivating a subject keeps its assignments but blocks new ones.
  - Foreign keys use `ON DELETE RESTRICT`. A subject with assignments (`SUBJECT_IN_USE`), a class with assignments (`CLASS_IN_USE`) or an academic year with assignments (`ACADEMIC_YEAR_IN_USE`) can't be deleted. Assignments referenced by future modules return `CLASS_SUBJECT_IN_USE`.
- **Dashboard:** Subject Management is listed as an implemented module, with a "Manage subjects" quick action and subject activity in the feed.
- **Not included yet:** teacher assignment, routines, examinations, marks and the student/teacher portals. They will reference `subjects.id` / `class_subjects.id`.

### Feature 08 — Student Enrollment & Promotion

Admin-only enrollment of existing students (Feature 03) into a class and section (Feature 06) for an academic year (Feature 05), and year-end promotion with a full audit trail.

- **Pages:**
  - Enrollments (`/enrollments`): real counts for the selected year (total, currently enrolled, new admissions, withdrawn), filters (year, class, section, status, admission type) and search by student name, student ID or roll number. Row actions: view, edit, change class/section, withdraw, mark transferred, re-admit, and delete (only for enrollments made by mistake).
  - Enroll a student (dialog): picks from active students not yet enrolled in the year. The roll number is optional and defaults to the next free number in the section.
  - Bulk enrollment (`/enrollments/bulk`): choose year, class, section, admission type, date and an optional starting roll; select students; the server checks them and assigns roll numbers (dry run); adjust any roll; confirm. A report lists every student enrolled or not, with the reason.
  - Enrollment details (`/enrollments/:id`): placement, status and reason, remarks, record history and the student's enrollments across all years, with a link to the student profile. The student profile also shows this history.
  - Promotions (`/promotions`), promotion wizard (`/promotions/new`) and batch details (`/promotions/:id`): choose source and target years, select students (all, by class or individually), map each class to its destination (suggested by display order, never by name), set per-student exceptions (repeat, complete studies, exclude, section, roll number), choose how roll numbers are assigned, preview, then confirm.
- **Data:**
  - `student_enrollments`: student, academic year, section (the class comes from the section), roll number, admission type (`new`, `continuing`, `readmission`), enrollment date, status (`enrolled`, `promoted`, `repeated`, `completed`, `withdrawn`, `transferred`), status note, remarks.
  - `promotion_batches` and `promotion_batch_items`: every confirmed promotion with its counts, who ran it and when, and each student's previous and new placement, decision and outcome.
- **Rules (enforced by the API and the database):**
  - One enrollment per student per academic year (`UNIQUE (student_id, academic_year_id)`); withdrawn students are re-admitted rather than enrolled twice.
  - A composite foreign key guarantees the section belongs to the enrollment's academic year; the API also checks that it belongs to the chosen class, that class and section are active, and that the year isn't completed.
  - Roll numbers are unique within a section among students who hold a place (a partial unique index); withdrawing or transferring frees the number.
  - The enrollment date must fall within the academic year, or up to 180 days before it starts.
  - Every enrollment write takes a per-academic-year advisory lock and share-locks the section, class and year, so concurrent writes can't double-book a student or a roll number.
  - Promoted, repeated and completed are set only by promotions. Those enrollments, and any enrollment in a completed year, keep their placement; only remarks can change. Enrollments referenced by promotion history can't be deleted.
  - Students with enrollments can't be deleted (`STUDENT_HAS_ENROLLMENTS`), and a section with enrollments can't be moved to another class or year (`SECTION_HAS_ENROLLMENTS`).
- **Promotion safety:**
  - Preview never writes. Execution locks both years and the selected enrollments, re-reads everything and re-validates the whole plan on the server (eligibility, destination class and section in the target year, active class and section, roll numbers), then applies it in one transaction. Any problem means nothing is saved.
  - Only enrollments still `enrolled` are eligible, and students already enrolled in the target year are skipped. The target year must be later and not completed.
  - Each confirmation carries a `request_id`; submitting it again returns the same batch instead of running twice. An enrollment can be processed by at most one promotion (unique index).
  - Roll numbers: `keep` keeps current numbers where free and gives the next free number otherwise (noted per student); `sequential` renumbers each destination section from 1. Manual numbers are validated too.
- **Dashboard:** "Students per class" is real (students currently enrolled in each class of the current year), recent admissions show their current class and section, and Student Enrollment & Promotion is listed as implemented.
- **Not included yet:** attendance, fees, examinations and the portals will reference `student_enrollments.id`.

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
| DELETE | `/students/:id`         | Admin (`students:delete`) | Permanently delete a student and their photo (refused once they have enrollments) |
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
| GET    | `/classes`              | Admin (`classes:read`)    | List classes with section counts: `search`, `status`, `sort` (`display_order`, `name`, `created_at`), `order`, `page`, `page_size` |
| GET    | `/classes/stats`        | Admin (`classes:read`)    | Class counts, total sections and current-year sections |
| GET    | `/classes/:id`          | Admin (`classes:read`)    | Class details |
| GET    | `/classes/:id/sections` | Admin (`sections:read`)   | The class's sections, newest academic year first (`academic_year_id` to filter) |
| POST   | `/classes`              | Admin (`classes:create`)  | Create a class (`name`, `code`, `display_order`, `description`, `status`) |
| PUT    | `/classes/:id`          | Admin (`classes:update`)  | Replace name, code, description (and display order if given) |
| PUT    | `/classes/order`        | Admin (`classes:update`)  | Reorder: `class_ids` lists every class in the new order |
| PATCH  | `/classes/:id/status`   | Admin (`classes:update`)  | Activate or deactivate (`status`) |
| DELETE | `/classes/:id`          | Admin (`classes:delete`)  | Delete a class with no sections or subject assignments |
| GET    | `/sections`             | Admin (`sections:read`)   | List sections: `academic_year_id`, `class_id`, `search`, `status`, `sort`, `order`, `page`, `page_size` |
| GET    | `/sections/:id`         | Admin (`sections:read`)   | Section details with its class and academic year |
| POST   | `/sections`             | Admin (`sections:create`) | Create a section (`academic_year_id`, `class_id`, `name`, `code`, `room_number`, `capacity`, `description`, `status`) |
| PUT    | `/sections/:id`         | Admin (`sections:update`) | Replace a section's details (including its class/year, under the same rules) |
| PATCH  | `/sections/:id/status`  | Admin (`sections:update`) | Activate or deactivate (`status`) |
| DELETE | `/sections/:id`         | Admin (`sections:delete`) | Delete a section no other record references |
| GET    | `/subjects`             | Admin (`subjects:read`)   | List subjects with assignment counts: `search`, `status`, `type`, `sort` (`name`, `code`, `created_at`), `order`, `page`, `page_size` |
| GET    | `/subjects/stats`       | Admin (`subjects:read`)   | Subject counts by status and type, total and current-year assignments |
| GET    | `/subjects/:id`         | Admin (`subjects:read`)   | Subject details |
| GET    | `/subjects/:id/classes` | Admin (`subjects:read`)   | The subject's class assignments, newest academic year first (`academic_year_id` to filter) |
| POST   | `/subjects`             | Admin (`subjects:create`) | Create a subject (`name`, `code`, `description`, `subject_type`, `full_marks`, `pass_marks`, `status`) |
| PUT    | `/subjects/:id`         | Admin (`subjects:update`) | Replace a subject's details (blank `subject_type`/`status` keep the current value) |
| PATCH  | `/subjects/:id/status`  | Admin (`subjects:update`) | Activate or deactivate (`status`) |
| DELETE | `/subjects/:id`         | Admin (`subjects:delete`) | Delete a subject with no class assignments |
| GET    | `/classes/:id/subjects` | Admin (`subjects:read`)   | The class's subject assignments, newest academic year first (`academic_year_id` to filter) |
| GET    | `/class-subjects`       | Admin (`subjects:read`)   | List assignments: `academic_year_id`, `class_id`, `subject_id`, `type` (`compulsory`/`optional`), `search`, `page`, `page_size` |
| GET    | `/class-subjects/:id`   | Admin (`subjects:read`)   | Assignment details with its subject, class and academic year |
| POST   | `/class-subjects`       | Admin (`subjects:assign`) | Assign subjects, all or nothing: `class_id`, `academic_year_id`, `subjects: [{subject_id, is_compulsory}]` (`is_compulsory` defaults to the subject type) |
| PUT    | `/class-subjects/:id`   | Admin (`subjects:assign`) | Change `is_compulsory` for that class and year |
| DELETE | `/class-subjects/:id`   | Admin (`subjects:assign`) | Remove an assignment no other record references |
| GET    | `/enrollments`          | Admin (`enrollments:read`) | List enrollments: `academic_year_id`, `class_id`, `section_id`, `status`, `admission_type`, `search` (name, student ID, roll), `sort` (`roll`, `name`, `student_id`, `enrollment_date`, `created_at`), `order`, `page`, `page_size` |
| GET    | `/enrollments/stats`    | Admin (`enrollments:read`) | Counts for one year (`academic_year_id`, default: the current year) |
| GET    | `/enrollments/unenrolled-students` | Admin (`enrollments:create`) | Active students not enrolled in `academic_year_id` (`search`, `page`, `page_size`) |
| GET    | `/enrollments/:id`      | Admin (`enrollments:read`) | Enrollment details with student, year, class and section |
| POST   | `/enrollments`          | Admin (`enrollments:create`) | Enroll one student (`student_id`, `academic_year_id`, `class_id`, `section_id`, `roll_number`, `admission_type`, `enrollment_date`, `remarks`) |
| POST   | `/enrollments/bulk`     | Admin (`enrollments:create`) | Enroll many students into one section; returns a per-student report. `dry_run: true` checks and assigns rolls without saving |
| PUT    | `/enrollments/:id`      | Admin (`enrollments:update`) | Edit class/section (same year), roll number, admission type, date and remarks |
| PATCH  | `/enrollments/:id/status` | Admin (`enrollments:update`) | Withdraw, mark transferred, or re-admit (`status`, `reason`) |
| DELETE | `/enrollments/:id`      | Admin (`enrollments:delete`) | Delete an enrollment made by mistake (enrolled, open year, not in a promotion) |
| GET    | `/students/:id/enrollment-history` | Admin (`enrollments:read`) | The student's enrollments, newest academic year first |
| POST   | `/promotions/preview`   | Admin (`promotions:execute`) | Compute a promotion without saving: summary, destinations, per-student outcome, blocking issues |
| POST   | `/promotions/execute`   | Admin (`promotions:execute`) | Re-validate and apply in one transaction (`request_id` required; a repeat returns the same batch with `meta.replayed`) |
| GET    | `/promotions`           | Admin (`promotions:read`) | Promotion batches, newest first (`academic_year_id`, `page`, `page_size`) |
| GET    | `/promotions/:id`       | Admin (`promotions:read`) | A batch with every student's decision, outcome and placements |

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
