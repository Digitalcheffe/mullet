import { useEffect, useState, type ComponentType, type SVGProps } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { ClientsIcon, DashboardIcon, DisplaysIcon, MenuIcon, PinIcon, PluginsIcon, SettingsIcon, ThemesIcon } from './icons';
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

// Whether a pinned sidebar survives a reload (issue #159) -- unpinned
// always starts collapsed, matching a fresh visit's default.
const PIN_STORAGE_KEY = 'mullet-admin-sidebar-pinned';

function readPinned(): boolean {
  try {
    return localStorage.getItem(PIN_STORAGE_KEY) === 'true';
  } catch {
    // A private window or blocked storage just means the pin doesn't
    // survive a reload, not that pinning itself should fail.
    return false;
  }
}

export default function AdminLayout({ username, onSignOut }: AdminLayoutProps) {
  const [pinned, setPinned] = useState(readPinned);
  const [expanded, setExpanded] = useState(pinned);

  useEffect(() => {
    try {
      localStorage.setItem(PIN_STORAGE_KEY, String(pinned));
    } catch {
      // Best-effort, see readPinned above.
    }
  }, [pinned]);

  // Expanded-but-not-pinned floats over the page (a scrim behind it
  // closes it on an outside click); pinned expanded instead pushes
  // .admin-content over, same as the sidebar's old permanently-232px
  // behavior.
  const overlay = expanded && !pinned;

  function togglePin() {
    setPinned((p) => {
      const next = !p;
      setExpanded(next);
      return next;
    });
  }

  // Only meaningful for the overlay case -- a pinned sidebar has nowhere
  // to "close back" to short of unpinning, so a nav click leaves it open.
  function collapseIfOverlay() {
    if (overlay) setExpanded(false);
  }

  return (
    <div className="admin-layout">
      {overlay && <div className="admin-sidebar-scrim" onClick={() => setExpanded(false)} />}
      <nav className={`admin-sidebar${expanded ? ' expanded' : ''}${overlay ? ' overlay' : ''}`}>
        <div className="admin-sidebar-top">
          <button
            className="admin-sidebar-toggle"
            onClick={() => setExpanded((e) => !e)}
            title={expanded ? 'Collapse sidebar' : 'Expand sidebar'}
            aria-label={expanded ? 'Collapse sidebar' : 'Expand sidebar'}
          >
            <MenuIcon />
          </button>
          <div className="admin-logo">
            <img className="admin-logo-mark" src="/logo-mark.png" alt="" />
            <span className="admin-logo-word">mullet</span>
          </div>
          <button
            className={`admin-sidebar-pin${pinned ? ' pinned' : ''}`}
            onClick={togglePin}
            title={pinned ? 'Unpin sidebar' : 'Pin sidebar open'}
            aria-label={pinned ? 'Unpin sidebar' : 'Pin sidebar open'}
          >
            <PinIcon filled={pinned} />
          </button>
        </div>

        <NavLink to="/admin" end className={navLinkClassName} title="Dashboard" onClick={collapseIfOverlay}>
          <DashboardIcon />
          <span className="admin-nav-label">Dashboard</span>
        </NavLink>

        <NavLink to="/admin/plugins" className={navLinkClassName} title="Data Plugins" onClick={collapseIfOverlay}>
          <PluginsIcon />
          <span className="admin-nav-label">Data Plugins</span>
        </NavLink>

        <NavLink to="/admin/displays" className={navLinkClassName} title="Displays" onClick={collapseIfOverlay}>
          <DisplaysIcon />
          <span className="admin-nav-label">Displays</span>
        </NavLink>

        <NavLink to="/admin/themes" className={navLinkClassName} title="Themes" onClick={collapseIfOverlay}>
          <ThemesIcon />
          <span className="admin-nav-label">Themes</span>
        </NavLink>

        <NavLink to="/admin/clients" className={navLinkClassName} title="Clients" onClick={collapseIfOverlay}>
          <ClientsIcon />
          <span className="admin-nav-label">Clients</span>
        </NavLink>

        {soonItems.map(({ label, icon: Icon }) => (
          <div className="admin-nav-link soon" key={label} title={label}>
            <Icon />
            <span className="admin-nav-label">{label}</span>
            <span className="soon-badge admin-nav-label">Soon</span>
          </div>
        ))}

        <NavLink
          to="/admin/settings"
          className={navLinkClassName}
          style={{ marginTop: 'auto' }}
          title="Settings"
          onClick={collapseIfOverlay}
        >
          <SettingsIcon />
          <span className="admin-nav-label">Settings</span>
        </NavLink>

        <div className="admin-user-chip">
          <span className="admin-user-avatar" />
          <span className="admin-nav-label">{username}</span>
          <button className="admin-signout admin-nav-label" onClick={onSignOut}>
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
