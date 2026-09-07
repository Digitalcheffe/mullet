import { useEffect, useState } from 'react';
import { Route, Routes } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth/AuthContext';
import AdminLayout from './layout/AdminLayout';
import Dashboard from './pages/Dashboard';
import LoginPage from './pages/LoginPage';
import SettingsPage from './pages/SettingsPage';
import SetupWizard from './pages/SetupWizard';

export default function AdminApp() {
  return (
    <AuthProvider>
      <AdminShell />
    </AuthProvider>
  );
}

function AdminShell() {
  const { isAuthenticated, username, logout } = useAuth();
  // null while the first-run check is in flight.
  const [setupRequired, setSetupRequired] = useState<boolean | null>(null);

  useEffect(() => {
    fetch('/api/admin/setup')
      .then((res) => res.json())
      .then((body: { required: boolean }) => setSetupRequired(body.required))
      .catch(() => setSetupRequired(false));
  }, []);

  if (setupRequired === null) {
    return <p>Loading…</p>;
  }

  if (setupRequired) {
    return <SetupWizard onComplete={() => setSetupRequired(false)} />;
  }

  if (!isAuthenticated) {
    return <LoginPage />;
  }

  return (
    <Routes>
      <Route element={<AdminLayout username={username!} onSignOut={logout} />}>
        <Route index element={<Dashboard />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}
