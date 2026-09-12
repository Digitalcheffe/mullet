import type { ComponentType, SVGProps } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { ClientsIcon, DashboardIcon, DisplaysIcon, PluginsIcon, SettingsIcon, ThemesIcon } from './icons';
import './AdminLayout.css';

interface AdminLayoutProps {
  username: string;
  onSignOut: () => void;
}

const navLinkClassName = ({ isActive }: { isActive: boolean }) =>
  isActive ? 'admin-nav-link active' : 'admin-nav-link';

interface SoonItem {
  label: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
}

// Pages the sidebar shows for a sense of the whole app, but that don't
// exist yet -- rendered disabled with a "Soon" badge rather than either
// a dead link or being hidden entirely.
const soonItems: SoonItem[] = [];

export default function AdminLayout({ username, onSignOut }: AdminLayoutProps) {
  return (
    <div className="admin-layout">
      <nav className="admin-sidebar">
        <div className="admin-logo">
          <img className="admin-logo-mark" src="/logo-mark.png" alt="" />
          <span className="admin-logo-word">mullet</span>
        </div>

        <NavLink to="/admin" end className={navLinkClassName}>
          <DashboardIcon />
          Dashboard
        </NavLink>

        <NavLink to="/admin/plugins" className={navLinkClassName}>
          <PluginsIcon />
          Data Plugins
        </NavLink>

        <NavLink to="/admin/displays" className={navLinkClassName}>
          <DisplaysIcon />
          Displays
        </NavLink>

        <NavLink to="/admin/themes" className={navLinkClassName}>
          <ThemesIcon />
          Themes
        </NavLink>

        <NavLink to="/admin/clients" className={navLinkClassName}>
          <ClientsIcon />
          Clients
        </NavLink>

        {soonItems.map(({ label, icon: Icon }) => (
          <div className="admin-nav-link soon" key={label}>
            <Icon />
            {label}
            <span className="soon-badge">Soon</span>
          </div>
        ))}

        <NavLink to="/admin/settings" className={navLinkClassName} style={{ marginTop: 'auto' }}>
          <SettingsIcon />
          Settings
        </NavLink>

        <div className="admin-user-chip">
          <span className="admin-user-avatar" />
          <span>{username}</span>
          <button className="admin-signout" onClick={onSignOut}>
            Sign out
          </button>
        </div>
      </nav>

      <main className="admin-content">
        <Outlet />
      </main>
    </div>
  );
}
