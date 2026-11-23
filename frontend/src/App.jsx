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
