import { lazy, StrictMode, Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import './index.css';

// Route-level code splitting (issue #31 Pi performance tuning): the
// admin app pulls in react-grid-layout (the Designer's drag-and-drop
// grid, a sizeable library) that a kiosk visiting only /display or
// /register has no use for and shouldn't have to download, parse, and
// evaluate before it can render. Lazy-loading all three keeps that
// weight out of whichever bundle chunk loads first.
const AdminApp = lazy(() => import('./admin/AdminApp'));
const DisplayApp = lazy(() => import('./display/DisplayApp'));
const RegisterPage = lazy(() => import('./register/RegisterPage'));

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <Suspense fallback={null}>
        <Routes>
          <Route path="/admin/*" element={<AdminApp />} />
          <Route path="/display/:slug" element={<DisplayApp />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Routes>
      </Suspense>
    </BrowserRouter>
  </StrictMode>,
);
