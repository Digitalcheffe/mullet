import { useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';

// Public and unauthenticated -- see AdminApp.tsx's AdminShell, and
// ForgotPasswordPage.tsx's comment for why this uses plain fetch.
export default function ResetPasswordPage() {
  const [params] = useSearchParams();
  const token = params.get('token') ?? '';
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    if (!token) {
      setError('This reset link is missing its token. Please request a new one.');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match.');
      return;
    }

    setSubmitting(true);
    try {
      const res = await fetch('/api/admin/reset-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, new_password: newPassword }),
      });
      if (res.ok) {
        setDone(true);
        return;
      }
      const message = await res.text();
      setError(message || 'Could not reset your password. The link may have expired.');
    } catch {
      setError('Could not reach the server. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  if (done) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <img className="auth-logo" src="/logo-wordmark.png" alt="Mullet" />
          <p className="auth-subtitle">Reset password</p>
          <p className="form-note">Your password has been changed.</p>
          <Link className="auth-secondary-link" to="/admin">
            Sign in
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={handleSubmit}>
        <img className="auth-logo" src="/logo-wordmark.png" alt="Mullet" />
        <p className="auth-subtitle">Reset password</p>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <label className="field">
          <span className="kicker">New password</span>
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            required
            minLength={8}
            autoFocus
          />
        </label>
        <label className="field">
          <span className="kicker">Confirm password</span>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            minLength={8}
          />
        </label>
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting ? 'Resetting…' : 'Reset password'}
        </button>
        <Link className="auth-secondary-link" to="/admin">
          Back to sign in
        </Link>
      </form>
    </div>
  );
}
