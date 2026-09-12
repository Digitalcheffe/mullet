import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';

// Public and unauthenticated -- see AdminApp.tsx's AdminShell, which
// renders this regardless of session state, the same way LoginPage
// itself is reachable without one. Uses plain fetch rather than
// useApiFetch for the same reason AuthContext's own login() does: there
// is no session yet for it to attach a token from.
export default function ForgotPasswordPage() {
  const [username, setUsername] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState<{ kind: 'ok' | 'error'; message: string } | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setResult(null);
    try {
      const res = await fetch('/api/admin/forgot-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username }),
      });
      if (res.ok) {
        setResult({ kind: 'ok', message: 'If that account has an email on file, a reset link is on its way.' });
        return;
      }
      const message = await res.text();
      setResult({ kind: 'error', message: message || 'Something went wrong. Please try again.' });
    } catch {
      setResult({ kind: 'error', message: 'Could not reach the server. Please try again.' });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={handleSubmit}>
        <img className="auth-logo" src="/logo-wordmark.png" alt="Mullet" />
        <p className="auth-subtitle">Reset password</p>

        {result ? (
          <p className={result.kind === 'ok' ? 'form-note' : 'form-error'} role={result.kind === 'error' ? 'alert' : undefined}>
            {result.message}
          </p>
        ) : (
          <p>Enter your username and, if your account has an email on file, we'll send a reset link.</p>
        )}

        {!result?.kind || result.kind === 'error' ? (
          <>
            <label className="field">
              <span className="kicker">Username</span>
              <input value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus />
            </label>
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? 'Sending…' : 'Send reset link'}
            </button>
          </>
        ) : null}

        <Link className="auth-secondary-link" to="/admin">
          Back to sign in
        </Link>
      </form>
    </div>
  );
}
