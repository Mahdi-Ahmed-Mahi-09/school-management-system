import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { ToastProvider } from './components/ui/Toast'
import { AuthProvider } from './context/AuthContext'
import AppLayout from './layouts/AppLayout'
import { PERMISSIONS } from './lib/roles'
import DashboardPage from './pages/DashboardPage'
import LoginPage from './pages/LoginPage'
import ProfilePage from './pages/ProfilePage'
import { ForbiddenPage, NotFoundPage } from './pages/StatusPages'
import UserDetailPage from './pages/admin/UserDetailPage'
import UsersPage from './pages/admin/UsersPage'
import StudentFormPage from './pages/students/StudentFormPage'
import StudentProfilePage from './pages/students/StudentProfilePage'
import StudentsPage from './pages/students/StudentsPage'
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
