import { useEffect, useState } from 'react';
import { Route, Routes, useLocation } from 'react-router-dom';
import './adminTheme.css';
import './forms.css';
import { AuthProvider, useAuth } from './auth/AuthContext';
import AdminLayout from './layout/AdminLayout';
import ClientsPage from './pages/ClientsPage';
import Dashboard from './pages/Dashboard';
import DesignerPage from './pages/DesignerPage';
import DisplaysPage from './pages/DisplaysPage';
import ForgotPasswordPage from './pages/ForgotPasswordPage';
import LoginPage from './pages/LoginPage';
import PluginsPage from './pages/PluginsPage';
import ResetPasswordPage from './pages/ResetPasswordPage';
import SettingsPage from './pages/SettingsPage';
import SetupWizard from './pages/SetupWizard';
import ThemeEditorPage from './pages/ThemeEditorPage';
import ThemesPage from './pages/ThemesPage';

export default function AdminApp() {
  return (
    <div className="admin-app-root">
      <AuthProvider>
        <AdminShell />
      </AuthProvider>
    </div>
  );
}

interface SetupStatus {
  required: boolean;
  auth_disabled?: boolean;
}

function AdminShell() {
  const { isAuthenticated, username, logout } = useAuth();
  const location = useLocation();
  // null while the first-run check is in flight.
  const [status, setStatus] = useState<SetupStatus | null>(null);

  useEffect(() => {
    fetch('/api/admin/setup')
      .then((res) => res.json())
      .then((body: SetupStatus) => setStatus(body))
      .catch(() => setStatus({ required: false }));
  }, []);

  // Reachable regardless of session or first-run state -- a locked-out
  // admin has no session to check, by definition.
  if (location.pathname === '/admin/forgot-password') {
    return <ForgotPasswordPage />;
  }
  if (location.pathname === '/admin/reset-password') {
    return <ResetPasswordPage />;
  }

  if (status === null) {
    return <p>Loading…</p>;
  }

  if (status.required) {
    return <SetupWizard onComplete={() => setStatus({ ...status, required: false })} />;
  }

  if (!isAuthenticated && !status.auth_disabled) {
    return <LoginPage />;
  }

  return (
    <Routes>
      <Route element={<AdminLayout username={status.auth_disabled ? 'dev' : username!} onSignOut={logout} />}>
        <Route index element={<Dashboard />} />
        <Route path="plugins" element={<PluginsPage />} />
        <Route path="displays" element={<DisplaysPage />} />
        <Route path="displays/:displayId/screens/:screenId/design" element={<DesignerPage />} />
        <Route path="clients" element={<ClientsPage />} />
        <Route path="themes" element={<ThemesPage />} />
        <Route path="themes/:themeId" element={<ThemeEditorPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}
