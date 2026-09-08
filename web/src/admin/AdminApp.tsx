import { useEffect, useState } from 'react';
import { Route, Routes } from 'react-router-dom';
import './adminTheme.css';
import './forms.css';
import { AuthProvider, useAuth } from './auth/AuthContext';
import AdminLayout from './layout/AdminLayout';
import Dashboard from './pages/Dashboard';
import LoginPage from './pages/LoginPage';
import SettingsPage from './pages/SettingsPage';
import SetupWizard from './pages/SetupWizard';

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
  // null while the first-run check is in flight.
  const [status, setStatus] = useState<SetupStatus | null>(null);

  useEffect(() => {
    fetch('/api/admin/setup')
      .then((res) => res.json())
      .then((body: SetupStatus) => setStatus(body))
      .catch(() => setStatus({ required: false }));
  }, []);

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
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}
