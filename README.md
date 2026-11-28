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

### Feature 09 — Teacher Assignment

Admin-only assignment of existing teachers (Feature 04) to sections (Feature 06) of an academic year (Feature 05): as the **subject teacher** of a subject the class takes that year (Feature 07), or as the section's **class teacher**.

- **Pages:**
  - Teacher assignments (`/teacher-assignments`): real counts for the selected year (active assignments, assigned teachers, subject assignments, class teachers, subjects without a teacher, sections without a class teacher), filters (year, class, section, subject, teacher, type, status) and search by teacher name or ID or subject. Row actions: view, change teacher, deactivate/reactivate, remove (only assignments made by mistake).
  - Assign subject teacher / Assign class teacher (dialogs): year → class → section (only that class's sections in that year) → subject (only subjects the class takes that year) → active teacher. Changing the year or class resets the dependent choices. The slot's current teacher is shown; replacing them needs an explicit confirmation.
  - Bulk assignment (`/teacher-assignments/bulk`): one teacher, one subject, several sections of a class. The preview shows each section's outcome; sections with another teacher are left alone unless the admin ticks "Replace" for them. A report lists every section.
  - Assignment details (`/teacher-assignments/:id`): the assignment and the full history of its slot (who held it, from when to when, and why it ended).
  - Teacher profile: teaching assignments for the current year (class teacher of, subjects taught, counts) and previous assignments.
  - Section details: the class teacher and each subject's teacher, with "Not assigned" where nobody teaches it. Class details: each section's class teacher and subject coverage.
- **Data:** `teacher_assignments` with teacher, academic year, class, section, subject (subject teachers only), type (`subject_teacher` / `class_teacher`), status (`active` / `inactive`), assigned and ended dates, end note, the assignment it replaced, and remarks.
- **Rules (enforced by the API and the database):**
  - A composite foreign key guarantees the section belongs to the assignment's class and year; another guarantees a subject teacher's subject is one the class takes that year (`class_subjects`). Class-teacher rows have no subject (check constraint).
  - Partial unique indexes allow one active teacher per subject in a section and one active class teacher per section. A teacher may teach several subjects and be class teacher of several sections.
  - New assignments need an active teacher, class, section and subject, and the current or an upcoming year.
  - Changing the teacher ends the current assignment (with a note) and creates a new one linked to it, so the history is kept. Deactivating keeps the row; reactivating needs the slot to be free. Only assignments made by mistake (active, open year, not part of a change) can be removed.
  - Writes take a per-section advisory lock and share-lock the teacher, class, year, section and subject rows, so concurrent requests can't double-assign a slot.
  - Teachers with assignments can't be deleted (`TEACHER_HAS_ASSIGNMENTS`), a section with assignments can't move to another class or year (`SECTION_HAS_TEACHER_ASSIGNMENTS`), and a class subject with teachers can't be removed.
  - Assignment responses carry only the teacher's name, ID, designation, status and photo — never contact or salary details.
- **Not included yet:** routines, examinations, marks and the teacher portal will reference `teacher_assignments.id`. Attendance (Feature 10) uses assignments to decide which sections a teacher can take attendance for.

### Feature 10 — Student Attendance

Daily attendance of enrolled students (Feature 08), taken per section (Feature 06) and academic year (Feature 05) by administrators and by the teachers assigned to the section (Feature 09).

- **Statuses:** `present`, `absent`, `late`, `leave` (shown as Present / Absent / Late / Leave), each with an optional remark. A student with no record for a day is **unmarked** — never counted as absent.
- **Pages:**
  - Attendance (`/attendance`): for a date, cards for enrolled students, present, absent, late, on leave and the attendance percentage (an empty state instead of zeros when nothing has been taken), a class-wise summary, and every section's state (complete, partly marked, not taken) with shortcuts.
  - Take attendance (`/attendance/mark`): year → class → section → date, the roster from enrollments, a Present/Absent/Late/Leave control per student, optional remarks, "Mark all present" (an explicit action — nothing is pre-filled), "Mark selected absent", live counts, a warning before leaving with unsaved changes, and a review dialog before saving. A session can't be saved while a student is unmarked.
  - Session details (`/attendance/session`): one section on one date, read-only, with who saved each record and CSV export.
  - Attendance records (`/attendance/records`): filters (year, class, section, status, date, date range, roll number, student), search by student name or ID, pagination, and row actions (details, edit, student history).
  - Record details (`/attendance/records/:id`): the record and its change history (old → new status and remarks, reason, who and when).
  - Reports (`/attendance/reports`, admin): daily, monthly (per student), class-wise, date range (with a day-by-day breakdown) and per student; each exports CSV.
  - Student profile: an Attendance card with recorded days, counts, percentage, recent days and a year selector. The admin dashboard's "Today's attendance" panel and the "Take attendance" quick action now use real data; teachers get "Take attendance" shortcuts on their dashboard.
- **Percentage:** (present + late) ÷ (present + absent + late) × 100. Leave and unmarked days are excluded; with a zero denominator the result is N/A. No weekends or holidays are assumed — only recorded days count.
- **Data:** `student_attendance` — enrollment, student, academic year, and the class, section and roll number at the time (a snapshot, so later section changes never rewrite history), date, status, remarks, version, marked by / updated by and timestamps. `student_attendance_changes` keeps every change after the first save (previous and new status and remarks, reason, who, when). Records are corrected in place and never deleted.
- **Rules (enforced by the API and the database):**
  - One record per student per day (unique on student + date and on enrollment + date). Composite foreign keys tie each record to its enrollment's student and year, and its section to its class and year; a trigger keeps the date inside the academic year; a check limits the status.
  - New records only for students holding a place in the section (not withdrawn or transferred) who were enrolled on or before the date; never for future dates or outside the year; not in a completed year or an inactive class/section (saved records there can still be corrected).
  - Teachers see and take attendance only for sections of their active assignments (class or subject teacher) through the teacher record linked to their login, and only for today. Administrators can take and correct any day; changing a saved record of an earlier day needs a reason.
  - Saves lock the section and date and check each record's version, so a concurrent change is reported (`ATTENDANCE_CHANGED`) instead of being overwritten; resubmitting the same data changes nothing.
  - Enrollments with attendance can't be deleted, an enrollment date can't move past recorded days, a section with attendance can't move to another class or year, and a year's dates can't shrink past recorded days.
  - Student photos are only included for administrators; CSV cells that look like formulas are neutralised.

### Feature 11 — Examination Management

Examinations of an academic year (Feature 05) for one or more classes (Feature 06), with a subject-wise timetable per class built from the subjects each class takes that year (Feature 07). Marks, grades and results are Feature 12 and are not part of this feature.

- **Examination types:** a predefined list (Class Test, Monthly Test, First Term, Half-Yearly, Second Term, Annual, Model Test, Test, Pre-Test, Selection Examination) plus custom types. Predefined types can be deactivated but not renamed or deleted; unused custom types can be deleted.
- **Pages:**
  - Examinations (`/examinations`): counts for the year (total, published, upcoming, completed, drafts), filters (academic year or all years for history, type, status, class, date range), search, pagination and row actions (details, timetable, edit, manage schedule, publish, complete, cancel, delete draft). Examination types are managed from here.
  - Create / edit (`/examinations/new`, `/examinations/:id/edit`): type, academic year, name (suggested from type and year), dates within the year, description, instructions and the classes (active classes only; selected classes shown as removable chips).
  - Details (`/examinations/:id`): information, record (created, updated, published, completed, cancelled), each class with its sections, enrolled students and timetable, adding/removing classes while a draft, and the full history.
  - Schedule builder (`/examinations/:id/schedule`): one tab per class and one row per subject the class takes that year; tick the subjects included and set date, start and end time, room and instructions inline. "Apply to included" sets one time for all papers. Overlaps are shown before saving; server conflicts (another class in the same room, the same class in another examination) are shown on the paper. Unsaved changes are protected.
  - Timetable (`/examinations/:id/timetable`): per-class tables or a day-by-day view, filters by class and date, CSV download and a print layout without navigation or buttons, headed with the examination, type, year and dates.
  - Exam timetables (`/exam-timetables`): published and past timetables. Teachers see only the classes they are assigned to (Feature 09), read-only, with the papers of their own subjects highlighted.
  - Admin dashboard: an Examinations panel (published this year, upcoming and recently created) and a working "Create examination" quick action.
- **Data:** `exam_types`; `examinations` (name, type, academic year, dates, description, instructions, status, published/completed/cancelled stamps and the cancellation reason); `examination_classes` (an examination's classes); `examination_schedules` (one paper: class, subject, date, start/end time, room, instructions, version). Schedules are class-level — every section of the class sits the same timetable — and are what Feature 12 will attach marks to.
- **Rules (enforced by the API and the database):**
  - Names are unique within an academic year; dates must lie within the year (trigger) and start on or before the end; completed years can't get examinations.
  - Composite foreign keys tie each class to the examination's year and each paper to a subject the class takes that year (`class_subjects`); one paper per subject per class; start before end; a paper's date within the examination (trigger).
  - An exclusion constraint (btree_gist) stops two papers of one class overlapping. The service also rejects overlaps with the same class in another (non-cancelled) examination and with a room already used at that time.
  - Status flow: draft → published → completed, and draft/published → cancelled (with a reason). Publishing runs a check — at least one class, every class scheduled, active classes and subjects, no conflicts — and shows issues and warnings (e.g. compulsory subjects without a paper). Completing needs the last day to have come.
  - Drafts can be freely edited and deleted. Published timetables can change only with a reason (recorded with before/after in the history); papers and classes can't be removed and the examination can't be deleted. Completed and cancelled examinations are read-only history.
  - Writes lock the examination and the academic year's examination schedule; papers carry a version, so a concurrent change is reported (`EXAM_CHANGED`) instead of overwritten.
  - Classes, class subjects and subjects used by examinations can't be removed, and an academic year's dates can't shrink past its examinations. Enrollment and attendance data are never changed.

### Feature 12 — Marks, Grades & Results

Marks for the papers of published examinations (Feature 11), entered per section by administrators and by each subject's teacher (Feature 09) for enrolled students (Feature 08), graded with the Bangladesh grading system and processed into published results.

- **Marking schemes** (`/results/:examId/configuration`): every paper starts from its subject's full and pass marks (Feature 07; 100/33% when the subject has none) and can be overridden for the examination without changing the subject. Optional components (Theory, MCQ, Practical, Assessment, …) are configured per paper; their maximums must add up to the full marks (checked by the API and a deferred database trigger). Once marks exist, changes need a reason, every sheet must be back in draft, components can only be renamed or resized, and maximums can't drop below marks already given.
- **Marks entry** (`/marks`, `/marks/:scheduleId?section=`): pick an examination, then a paper and section; teachers only see the subjects and sections they teach. The table has one column per component (or a single marks column), an exam attendance switch (Present/Absent — separate from daily attendance), live totals, inline validation, progress, unmarked students highlighted (never given zero automatically) and a warning before leaving with unsaved changes. Saving is all-or-nothing: every record is validated and each failure is reported by row.
- **Workflow:** draft → submitted (teacher or admin, every student marked) → verified → finalized (admins). Submitted marks are read-only for teachers; admins can return or reopen marks with a reason, which discards unpublished processed results. Every change to a saved mark is kept with its before/after values, reason and author.
- **Grading:** percentage = obtained ÷ full × 100, compared exactly with the boundaries (marks are stored in hundredths, so 79.99% is an A, never rounded up to A+): 80 A+ 5.00, 70 A 4.00, 60 A- 3.50, 50 B 3.00, 40 C 2.00, 33 D 1.00, below 33 F 0.00. A subject fails when the student was absent (0 marks, absence recorded separately), scored below the paper's pass marks (even when the percentage would be a D) or earned F; a failed subject is always F/0.00.
- **GPA:** sum of grade points ÷ number of subjects, rounded half up to two decimals (4.375 → 4.38). **If any subject is failed the GPA is 0.00, the grade F and the result FAIL.** The overall grade comes from the GPA (5.00 A+, ≥4.00 A, ≥3.50 A-, ≥3.00 B, ≥2.00 C, ≥1.00 D). No fourth-subject bonus. The database refuses a failed result with a GPA above 0.
- **Processing** (`/results`): subject results (every paper of every student holding a place in an examination class), then overall results (totals, percentage, GPA, grade, class and section positions), verification and publication — each step from the results dashboard with a confirmation. Processing needs every sheet finalized and every student marked, runs in one transaction under the examination's lock, and can be repeated safely until publication. Results are snapshots (subject name, marks, grade) so later configuration changes don't alter them.
- **Merit lists:** passed students only, by total marks then GPA; equal totals and GPAs share a position with standard competition ranking (1, 2, 2, 4); failed students get no position and are listed last. Class, section and whole-examination lists, with CSV and print.
- **Publication:** publishing requires verified results and re-checks completeness and the fail rule; it records who published and when and stores a JSON snapshot of the published results. Published results can't be edited, reopened or reprocessed; an administrator starts a correction with a reason, reopens the affected marks, reprocesses and republishes (each publication keeps its own snapshot and reason).
- **Dashboard and reports:** students, marks entered and pending, sheets finalized, results processed, passed, failed and published, pass percentage, average and highest GPA, highest marks (filterable by class and section); results with search and pass/fail filter; printable report cards (`/results/:examId/students/:studentId`) with subject grades, totals, GPA, grade, result, position and publication date (unpublished results are labelled and shown to administrators only); the student profile lists each examination result.
- **Access:** teachers (`marks:enter`) can view and enter marks for their own papers and sections and submit them; configuring, verifying, processing, publishing and every result report are administrator-only. Enrollments with marks can't be deleted (`ENROLLMENT_HAS_MARKS`).

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
| GET    | `/teacher-assignments`  | Admin (`teacher_assignments:read`) | List: `academic_year_id`, `class_id`, `section_id`, `subject_id`, `teacher_id`, `type`, `status`, `search`, `page`, `page_size` |
| GET    | `/teacher-assignments/stats` | Admin (`teacher_assignments:read`) | Counts for one year (`academic_year_id`, default: the current year) |
| GET    | `/teacher-assignments/:id` | Admin (`teacher_assignments:read`) | Assignment details with the slot's `history` |
| POST   | `/teacher-assignments`  | Admin (`teacher_assignments:create`) | Assign (`assignment_type`, `teacher_id`, `academic_year_id`, `class_id`, `section_id`, `subject_id`, `replace`, `remarks`) |
| POST   | `/teacher-assignments/bulk` | Admin (`teacher_assignments:create`) | One teacher, one subject, several sections (`section_ids`, `replace_section_ids`, `dry_run`); per-section report |
| PUT    | `/teacher-assignments/:id` | Admin (`teacher_assignments:update`) | Change the teacher (history kept) and/or remarks |
| PATCH  | `/teacher-assignments/:id/status` | Admin (`teacher_assignments:update`) | Deactivate (`reason`) or reactivate |
| DELETE | `/teacher-assignments/:id` | Admin (`teacher_assignments:delete`) | Remove an assignment made by mistake |
| GET / POST | `/class-teacher-assignments` | Admin (`teacher_assignments:read` / `:create`) | Class-teacher assignments only (same fields, no subject) |
| PUT    | `/class-teacher-assignments/:id` | Admin (`teacher_assignments:update`) | Change a class teacher (history kept) |
| GET    | `/teachers/:id/assignments` | Admin (`teacher_assignments:read`) | A teacher's assignments, newest year first |
| GET    | `/sections/:id/teacher-assignments` | Admin (`teacher_assignments:read`) | The section's class teacher and each subject's teacher (or null) |
| GET    | `/classes/:id/teacher-assignments` | Admin (`teacher_assignments:read`) | The same for each section of the class (`academic_year_id`, default: current) |
| GET    | `/attendance/options`   | Admin, Teacher (`attendance:read`) | Years and the sections the caller can use (`academic_year_id`, default: current), `today`, scope |
| GET    | `/attendance`           | Admin, Teacher (`attendance:read`) | Records: `academic_year_id`, `class_id`, `section_id`, `student_id`, `date`, `from`, `to`, `status`, `roll_number`, `search`, `page`, `page_size` (teachers: their sections) |
| GET    | `/attendance/:id`       | Admin, Teacher (`attendance:read`) | A record with its change `history` and whether the caller may edit it |
| GET    | `/attendance/session`   | Admin, Teacher (`attendance:read`) | A section's roster and attendance (`academic_year_id`, `section_id`, `date`) |
| GET    | `/attendance/session/summary` | Admin, Teacher (`attendance:read`) | A date across sections: totals, per class, per section |
| POST   | `/attendance`           | Admin, Teacher (`attendance:mark`) | One student, one day (`enrollment_id`, `date`, `status`, `remarks`) |
| POST   | `/attendance/bulk`      | Admin, Teacher (`attendance:mark`) | Save a section for a date (`records[]` with `version`; `reason` for earlier days) in one transaction |
| PUT    | `/attendance/:id`       | Admin, Teacher (`attendance:mark`) | Change status and remarks (`version`, `reason` for earlier days) |
| PATCH  | `/attendance/:id/status` | Admin, Teacher (`attendance:mark`) | Change the status only |
| GET    | `/attendance/reports/daily` | Admin (`attendance:reports`) | A date across sections; with `section_id`, every student |
| GET    | `/attendance/reports/monthly` | Admin (`attendance:reports`) | Per-student counts for `month` (YYYY-MM) and `class_id` (and optional `section_id`) |
| GET    | `/attendance/reports/class-wise` | Admin (`attendance:reports`) | Classes (or a class's sections) over `from`–`to`, with a daily breakdown |
| GET    | `/attendance/reports/student-wise` | Admin (`attendance:reports`) | A student's year summary, months and records (`student_id`, `academic_year_id`) |
| GET    | `/attendance/export`    | Admin (`attendance:export`) | CSV of `report` = daily, monthly, class-wise, student or records, with the same filters |
| GET    | `/students/:id/attendance` | Admin (`attendance:read`) | A student's records |
| GET    | `/students/:id/attendance/summary` | Admin (`attendance:read`) | A student's counts, months and recent days for a year |
| GET    | `/exam-types`           | Admin (`examinations:read`) | Predefined and custom examination types with usage counts |
| POST / PUT / DELETE | `/exam-types[/:id]` | Admin (`examinations:create` / `:update` / `:delete`) | Add a custom type, rename or (de)activate, delete an unused custom type |
| GET    | `/examinations`         | Admin (`examinations:read`) | List: `academic_year_id`, `exam_type_id`, `status`, `class_id`, `from`, `to`, `search`, `page`, `page_size` |
| GET    | `/examinations/statistics` | Admin (`examinations:read`) | Counts for one year (default: current) |
| GET    | `/examinations/:id`     | Admin (`examinations:read`) | Details with `classes` (sections, students, subjects, schedule) |
| GET    | `/examinations/:id/classes`, `/examinations/:id/schedules` | Admin (`examinations:read`) | Classes with subjects and papers (`class_id` to narrow) |
| GET    | `/examinations/:id/history` | Admin (`examinations:read`) | Audit trail |
| POST   | `/examinations`         | Admin (`examinations:create`) | Create (`name`, `exam_type_id`, `academic_year_id`, `start_date`, `end_date`, `description`, `instructions`, `class_ids`) |
| PUT    | `/examinations/:id`     | Admin (`examinations:update`) | Edit details (`reason` required once published) |
| DELETE | `/examinations/:id`     | Admin (`examinations:delete`) | Delete a draft |
| POST   | `/examinations/:id/classes` | Admin (`examinations:update`) | Add classes (drafts) |
| DELETE | `/examinations/:id/classes/:classId` | Admin (`examinations:update`) | Remove a class and its papers (drafts) |
| POST   | `/examinations/:id/schedules` | Admin (`examinations:update`) | Add papers for a class (`class_id`, `items[]`) |
| PUT    | `/examinations/:id/classes/:classId/schedules` | Admin (`examinations:update`) | Save a class's whole schedule (`items[]` with `id`/`version` for existing papers; unlisted papers are removed in drafts) |
| PUT / DELETE | `/examination-schedules/:id` | Admin (`examinations:update`) | Change one paper (`version`, `reason` once published) / remove a draft paper |
| GET    | `/examinations/:id/publish-check` | Admin (`examinations:publish`) | `{ ready, issues, warnings, classes, papers }` |
| PATCH  | `/examinations/:id/publish`, `/complete`, `/cancel` | Admin (`examinations:publish`) | Status transitions (`reason` to cancel) |
| GET    | `/exam-timetables`      | Admin, Teacher (`exam_timetables:read`) | Non-draft examinations the caller can see (teachers: their classes) |
| GET    | `/examinations/:id/timetable`, `/examinations/:id/classes/:classId/timetable` | Admin, Teacher (`exam_timetables:read`) | Timetable (`class_id`, `date`); teachers: published/past, their classes, own papers marked |
| GET    | `/mark-entry/examinations` | Admin, Teacher (`marks:enter`) | Published/completed examinations with marks to enter (teachers: with papers they teach) |
| GET    | `/examinations/:id/mark-sheets` | Admin, Teacher (`marks:enter`) | Paper × section sheets with progress and status (teachers: their own) |
| GET    | `/examination-schedules/:id/mark-configuration` | Admin, Teacher (`marks:enter`) | A paper's marking scheme (saved, or the subject's defaults) |
| POST / PUT | `/examination-schedules/:id/mark-configuration` | Admin (`marks:configure`) | Save full/pass marks, instructions and components (`reason` once marks exist) |
| GET    | `/examinations/:id/mark-configurations` | Admin (`marks:configure`) | Every paper with its scheme |
| GET    | `/examination-schedules/:id/marks` | Admin, Teacher (`marks:enter`) | A section's students and marks (`section_id`) with what the caller may do |
| POST   | `/examination-schedules/:id/marks` | Admin, Teacher (`marks:enter`) | One student's marks |
| POST   | `/examination-schedules/:id/marks/bulk` | Admin, Teacher (`marks:enter`) | A section's marks in one transaction (`records[]` with `attendance`, `components` or `total`, `version`) |
| PUT    | `/examination-marks/:id` | Admin, Teacher (`marks:enter`) | Change one saved mark (`version`, optional `reason`) |
| GET    | `/examination-marks/:id/history` | Admin, Teacher (`marks:enter`) | Audited changes of a mark |
| POST   | `/examination-schedules/:id/marks/submit` | Admin, Teacher (`marks:enter`) | Submit a section's marks for verification |
| PATCH  | `/examination-schedules/:id/marks/verify`, `/finalize`, `/reopen` | Admin (`marks:verify`) | Workflow steps (`section_id`; `reason` to reopen) |
| GET    | `/examinations/:id/marks-review` | Admin (`marks:verify`) | Every sheet of the examination |
| POST   | `/examinations/:id/process-subject-results`, `/process-overall-results` | Admin (`results:process`) | Grade papers / compute GPAs and positions |
| PATCH  | `/examinations/:id/verify-results` | Admin (`results:process`) | Mark processed results as reviewed |
| POST   | `/examinations/:id/publish-results` | Admin (`results:publish`) | Publish verified results (snapshot) |
| POST   | `/examinations/:id/correct-published-results` | Admin (`results:publish`) | Start an audited correction (`reason`) |
| GET    | `/examinations/:id/result-status`, `/result-statistics` | Admin (`results:read`) | Stage, readiness issues, statistics (`class_id`, `section_id`), publications |
| GET    | `/examinations/:id/results`, `/examinations/:id/classes/:classId/results` | Admin (`results:read`) | Overall results (`section_id`, `status`, `search`, paging) |
| GET    | `/examinations/:id/merit-list`, `/examinations/:id/classes/:classId/merit-list` | Admin (`results:read`) | Ranked results with positions (`class_id`, `section_id`) |
| GET    | `/examinations/:id/students/:studentId/report-card` | Admin (`results:read`) | One student's report card |
| GET    | `/examination-results/:id` | Admin (`results:read`) | An overall result with its subject results |
| GET    | `/students/:id/examination-results` | Admin (`results:read`) | A student's result history |

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
