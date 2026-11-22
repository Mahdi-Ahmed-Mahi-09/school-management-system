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
