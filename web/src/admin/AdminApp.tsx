import { useEffect, useState } from 'react';
import { AuthProvider, useAuth } from './auth/AuthContext';
import LoginPage from './pages/LoginPage';
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
    <div>
      <h1>Mullet Admin</h1>
      <p>
        Signed in as {username}. <button onClick={logout}>Sign out</button>
      </p>
      <p>Dashboard and navigation are added in a later issue.</p>
    </div>
  );
}
