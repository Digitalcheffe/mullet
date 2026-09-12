import { useState, type FormEvent } from 'react';
import { useAuth } from '../auth/AuthContext';

export default function LoginPage() {
  const { login } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(username, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={handleSubmit}>
        <img className="auth-logo" src="/logo-wordmark.png" alt="Mullet" />
        <p className="auth-subtitle">Admin</p>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <label className="field">
          <span className="kicker">Username</span>
          <input value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus />
        </label>
        <label className="field">
          <span className="kicker">Password</span>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
    </div>
  );
}
