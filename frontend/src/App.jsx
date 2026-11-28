import { lazy } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { ToastProvider } from './components/ui/Toast'
import { AuthProvider } from './context/AuthContext'
import AppLayout from './layouts/AppLayout'
import { PERMISSIONS } from './lib/roles'
import DashboardPage from './pages/DashboardPage'
import LoginPage from './pages/LoginPage'
import ProfilePage from './pages/ProfilePage'
import { ForbiddenPage, NotFoundPage } from './pages/StatusPages'

// Admin modules are loaded on demand so other roles never download them.
const AcademicYearDetailPage = lazy(() => import('./pages/academic-years/AcademicYearDetailPage'))
const AcademicYearsPage = lazy(() => import('./pages/academic-years/AcademicYearsPage'))
const ClassDetailPage = lazy(() => import('./pages/classes/ClassDetailPage'))
const ClassesPage = lazy(() => import('./pages/classes/ClassesPage'))
const SectionDetailPage = lazy(() => import('./pages/classes/SectionDetailPage'))
const SectionsPage = lazy(() => import('./pages/classes/SectionsPage'))
const BulkEnrollPage = lazy(() => import('./pages/enrollments/BulkEnrollPage'))
const EnrollmentDetailPage = lazy(() => import('./pages/enrollments/EnrollmentDetailPage'))
const EnrollmentsPage = lazy(() => import('./pages/enrollments/EnrollmentsPage'))
const PromotionDetailPage = lazy(() => import('./pages/promotions/PromotionDetailPage'))
const PromotionsPage = lazy(() => import('./pages/promotions/PromotionsPage'))
const PromotionWizardPage = lazy(() => import('./pages/promotions/PromotionWizardPage'))
const BulkAssignPage = lazy(() => import('./pages/teacher-assignments/BulkAssignPage'))
const TeacherAssignmentDetailPage = lazy(() => import('./pages/teacher-assignments/TeacherAssignmentDetailPage'))
const TeacherAssignmentsPage = lazy(() => import('./pages/teacher-assignments/TeacherAssignmentsPage'))
const AttendanceDashboardPage = lazy(() => import('./pages/attendance/AttendanceDashboardPage'))
const AttendanceRecordPage = lazy(() => import('./pages/attendance/AttendanceRecordPage'))
const AttendanceRecordsPage = lazy(() => import('./pages/attendance/AttendanceRecordsPage'))
const AttendanceReportsPage = lazy(() => import('./pages/attendance/AttendanceReportsPage'))
const AttendanceSessionPage = lazy(() => import('./pages/attendance/AttendanceSessionPage'))
const MarkAttendancePage = lazy(() => import('./pages/attendance/MarkAttendancePage'))
const ExamFormPage = lazy(() => import('./pages/examinations/ExamFormPage'))
const ExaminationDetailPage = lazy(() => import('./pages/examinations/ExaminationDetailPage'))
const ExaminationsPage = lazy(() => import('./pages/examinations/ExaminationsPage'))
const ExamTimetablePage = lazy(() => import('./pages/examinations/ExamTimetablePage'))
const ExamTimetablesPage = lazy(() => import('./pages/examinations/ExamTimetablesPage'))
const ScheduleBuilderPage = lazy(() => import('./pages/examinations/ScheduleBuilderPage'))
const ExamResultsPage = lazy(() => import('./pages/results/ExamResultsPage'))
const MarkEntryPage = lazy(() => import('./pages/results/MarkEntryPage'))
const MarkSchemesPage = lazy(() => import('./pages/results/MarkSchemesPage'))
const MarksHomePage = lazy(() => import('./pages/results/MarksHomePage'))
const MarksReviewPage = lazy(() => import('./pages/results/MarksReviewPage'))
const ReportCardPage = lazy(() => import('./pages/results/ReportCardPage'))
const ResultsDashboardPage = lazy(() => import('./pages/results/ResultsDashboardPage'))
const ClassSubjectsPage = lazy(() => import('./pages/subjects/ClassSubjectsPage'))
const SubjectDetailPage = lazy(() => import('./pages/subjects/SubjectDetailPage'))
const SubjectsPage = lazy(() => import('./pages/subjects/SubjectsPage'))
const UserDetailPage = lazy(() => import('./pages/admin/UserDetailPage'))
const UsersPage = lazy(() => import('./pages/admin/UsersPage'))
const StudentFormPage = lazy(() => import('./pages/students/StudentFormPage'))
const StudentProfilePage = lazy(() => import('./pages/students/StudentProfilePage'))
const StudentsPage = lazy(() => import('./pages/students/StudentsPage'))
const TeacherFormPage = lazy(() => import('./pages/teachers/TeacherFormPage'))
const TeacherProfilePage = lazy(() => import('./pages/teachers/TeacherProfilePage'))
const TeachersPage = lazy(() => import('./pages/teachers/TeachersPage'))
import { GuestOnly, RequireAccess, RequireAuth } from './routes/guards'

export default function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <AuthProvider>
          <Routes>
            <Route element={<GuestOnly />}>
              <Route path="/login" element={<LoginPage />} />
            </Route>

            <Route element={<RequireAuth />}>
              <Route element={<AppLayout />}>
                <Route index element={<DashboardPage />} />
                <Route element={<RequireAccess permission={PERMISSIONS.PROFILE_READ} />}>
                  <Route path="profile" element={<ProfilePage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.USERS_READ} roles={['admin']} />}>
                  <Route path="users" element={<UsersPage />} />
                  <Route path="users/:id" element={<UserDetailPage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.ACADEMIC_YEARS_READ} roles={['admin']} />}>
                  <Route path="academic-years" element={<AcademicYearsPage />} />
                  <Route path="academic-years/:id" element={<AcademicYearDetailPage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.CLASSES_READ} roles={['admin']} />}>
                  <Route path="classes" element={<ClassesPage />} />
                  <Route path="classes/:id" element={<ClassDetailPage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.SECTIONS_READ} roles={['admin']} />}>
                  <Route path="sections" element={<SectionsPage />} />
                  <Route path="sections/:id" element={<SectionDetailPage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.SUBJECTS_READ} roles={['admin']} />}>
                  <Route path="subjects" element={<SubjectsPage />} />
                  <Route path="subjects/:id" element={<SubjectDetailPage />} />
                  <Route path="class-subjects" element={<ClassSubjectsPage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.TEACHER_ASSIGNMENTS_READ} roles={['admin']} />}>
                  <Route path="teacher-assignments" element={<TeacherAssignmentsPage />} />
                  <Route path="teacher-assignments/:id" element={<TeacherAssignmentDetailPage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.TEACHER_ASSIGNMENTS_CREATE} />}>
                    <Route path="teacher-assignments/bulk" element={<BulkAssignPage />} />
                  </Route>
                </Route>
                {/* Marks: admins and the teachers of each subject and section (the API enforces it). */}
                <Route element={<RequireAccess permission={PERMISSIONS.MARKS_ENTER} roles={['admin', 'teacher']} />}>
                  <Route path="marks" element={<MarksHomePage />} />
                  <Route path="marks/:scheduleId" element={<MarkEntryPage />} />
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.RESULTS_READ} roles={['admin']} />}>
                  <Route path="results" element={<ResultsDashboardPage />} />
                  <Route path="results/:examId/results" element={<ExamResultsPage />} />
                  <Route path="results/:examId/students/:studentId" element={<ReportCardPage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.MARKS_CONFIGURE} />}>
                    <Route path="results/:examId/configuration" element={<MarkSchemesPage />} />
                  </Route>
                  <Route element={<RequireAccess permission={PERMISSIONS.MARKS_VERIFY} />}>
                    <Route path="results/:examId/review" element={<MarksReviewPage />} />
                  </Route>
                </Route>
                {/* Examinations: managed by admins; timetables are read-only for teachers of the classes. */}
                <Route element={<RequireAccess permission={PERMISSIONS.EXAMS_READ} roles={['admin']} />}>
                  <Route path="examinations" element={<ExaminationsPage />} />
                  <Route path="examinations/:id" element={<ExaminationDetailPage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.EXAMS_CREATE} />}>
                    <Route path="examinations/new" element={<ExamFormPage />} />
                  </Route>
                  <Route element={<RequireAccess permission={PERMISSIONS.EXAMS_UPDATE} />}>
                    <Route path="examinations/:id/edit" element={<ExamFormPage />} />
                    <Route path="examinations/:id/schedule" element={<ScheduleBuilderPage />} />
                  </Route>
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.EXAM_TIMETABLES_READ} roles={['admin', 'teacher']} />}>
                  <Route path="exam-timetables" element={<ExamTimetablesPage />} />
                  <Route path="examinations/:id/timetable" element={<ExamTimetablePage />} />
                </Route>
                {/* Attendance: admins and teachers (the API limits teachers to their sections). */}
                <Route element={<RequireAccess permission={PERMISSIONS.ATTENDANCE_READ} roles={['admin', 'teacher']} />}>
                  <Route path="attendance" element={<AttendanceDashboardPage />} />
                  <Route path="attendance/session" element={<AttendanceSessionPage />} />
                  <Route path="attendance/records" element={<AttendanceRecordsPage />} />
                  <Route path="attendance/records/:id" element={<AttendanceRecordPage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.ATTENDANCE_MARK} />}>
                    <Route path="attendance/mark" element={<MarkAttendancePage />} />
                  </Route>
                  <Route element={<RequireAccess permission={PERMISSIONS.ATTENDANCE_REPORTS} roles={['admin']} />}>
                    <Route path="attendance/reports" element={<AttendanceReportsPage />} />
                  </Route>
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.ENROLLMENTS_READ} roles={['admin']} />}>
                  <Route path="enrollments" element={<EnrollmentsPage />} />
                  <Route path="enrollments/:id" element={<EnrollmentDetailPage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.ENROLLMENTS_CREATE} />}>
                    <Route path="enrollments/bulk" element={<BulkEnrollPage />} />
                  </Route>
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.PROMOTIONS_READ} roles={['admin']} />}>
                  <Route path="promotions" element={<PromotionsPage />} />
                  <Route path="promotions/:id" element={<PromotionDetailPage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.PROMOTIONS_EXECUTE} />}>
                    <Route path="promotions/new" element={<PromotionWizardPage />} />
                  </Route>
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.STUDENTS_READ} roles={['admin']} />}>
                  <Route path="students" element={<StudentsPage />} />
                  <Route path="students/:id" element={<StudentProfilePage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.STUDENTS_CREATE} />}>
                    <Route path="students/new" element={<StudentFormPage />} />
                  </Route>
                  <Route element={<RequireAccess permission={PERMISSIONS.STUDENTS_UPDATE} />}>
                    <Route path="students/:id/edit" element={<StudentFormPage />} />
                  </Route>
                </Route>
                <Route element={<RequireAccess permission={PERMISSIONS.TEACHERS_READ} roles={['admin']} />}>
                  <Route path="teachers" element={<TeachersPage />} />
                  <Route path="teachers/:id" element={<TeacherProfilePage />} />
                  <Route element={<RequireAccess permission={PERMISSIONS.TEACHERS_CREATE} />}>
                    <Route path="teachers/new" element={<TeacherFormPage />} />
                  </Route>
                  <Route element={<RequireAccess permission={PERMISSIONS.TEACHERS_UPDATE} />}>
                    <Route path="teachers/:id/edit" element={<TeacherFormPage />} />
                  </Route>
                </Route>
                <Route path="forbidden" element={<ForbiddenPage />} />
                <Route path="*" element={<NotFoundPage />} />
              </Route>
            </Route>
          </Routes>
        </AuthProvider>
      </ToastProvider>
    </BrowserRouter>
  )
}
