import { NavLink, Outlet } from 'react-router-dom';
import './AdminLayout.css';

interface AdminLayoutProps {
  username: string;
  onSignOut: () => void;
}

const navLinkClassName = ({ isActive }: { isActive: boolean }) =>
  isActive ? 'admin-nav-link active' : 'admin-nav-link';

export default function AdminLayout({ username, onSignOut }: AdminLayoutProps) {
  return (
    <div className="admin-layout">
      <header className="admin-topbar">
        <span className="admin-topbar-title">Mullet Admin</span>
        <span className="admin-topbar-user">
          {username}
          <button onClick={onSignOut}>Sign out</button>
        </span>
      </header>
      <div className="admin-body">
        <nav className="admin-sidebar">
          <NavLink to="/admin" end className={navLinkClassName}>
            Dashboard
          </NavLink>
          <NavLink to="/admin/settings" className={navLinkClassName}>
            Settings
          </NavLink>
        </nav>
        <main className="admin-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
