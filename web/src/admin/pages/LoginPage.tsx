import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

export default function LoginPage() {
  const { login, verifyMFA } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Set once login() reports mfa_required -- switches the form to the
  // code-entry step for the same username, without a second password
  // prompt.
  const [pendingToken, setPendingToken] = useState<string | null>(null);
  const [code, setCode] = useState('');

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const result = await login(username, password);
      if (result.mfaRequired && result.pendingToken) {
        setPendingToken(result.pendingToken);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleVerifyMFA(e: FormEvent) {
    e.preventDefault();
    if (!pendingToken) return;
    setError(null);
    setSubmitting(true);
    try {
      await verifyMFA(username, pendingToken, code);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (pendingToken) {
    return (
      <div className="auth-page">
        <form className="auth-card" onSubmit={handleVerifyMFA}>
          <img className="auth-logo" src="/logo-wordmark.png" alt="Mullet" />
          <p className="auth-subtitle">Two-factor authentication</p>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <p>Enter the 6-digit code from your authenticator app, or one of your backup codes.</p>
          <label className="field">
            <span className="kicker">Code</span>
            <input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              required
              autoFocus
              autoComplete="one-time-code"
            />
          </label>
          <button type="submit" className="btn-primary" disabled={submitting}>
            {submitting ? 'Verifying…' : 'Verify'}
          </button>
          <button
            type="button"
            className="auth-secondary-link"
            onClick={() => {
              setPendingToken(null);
              setCode('');
              setError(null);
            }}
          >
            Back to sign in
          </button>
        </form>
      </div>
    );
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
        <Link className="auth-secondary-link" to="/admin/forgot-password">
          Forgot password?
        </Link>
      </form>
    </div>
  );
}
