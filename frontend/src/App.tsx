import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import Layout from './components/layout/Layout';
import Home from './pages/Home';
import Dashboard from './pages/Dashboard';
import Sessions from './pages/Sessions';
import SessionDetail from './pages/SessionDetail';
import Profile from './pages/Profile';
import Admin from './pages/Admin';
import AdminSessions from './pages/AdminSessions';
import PendingApproval from './pages/PendingApproval';
import Loading from './components/ui/Loading';

function ProtectedRoute({ children, requireApproved = true, requireAdmin = false }: {
  children: React.ReactNode;
  requireApproved?: boolean;
  requireAdmin?: boolean;
}) {
  const { isAuthenticated, isLoading, isApproved, isAdmin } = useAuth();

  if (isLoading) {
    return <Loading />;
  }

  if (!isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  if (requireApproved && !isApproved) {
    return <Navigate to="/pending" replace />;
  }

  if (requireAdmin && !isAdmin) {
    return <Navigate to="/dashboard" replace />;
  }

  return <>{children}</>;
}

function AppRoutes() {
  const { isAuthenticated, isLoading, isApproved } = useAuth();

  if (isLoading) {
    return <Loading />;
  }

  return (
    <Routes>
      <Route path="/" element={isAuthenticated ? <Navigate to={isApproved ? "/dashboard" : "/pending"} replace /> : <Home />} />

      <Route path="/pending" element={
        <ProtectedRoute requireApproved={false}>
          <PendingApproval />
        </ProtectedRoute>
      } />

      <Route element={<Layout />}>
        <Route path="/dashboard" element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        } />

        <Route path="/sessions" element={
          <ProtectedRoute>
            <Sessions />
          </ProtectedRoute>
        } />

        <Route path="/sessions/:id" element={
          <ProtectedRoute>
            <SessionDetail />
          </ProtectedRoute>
        } />

        <Route path="/profile" element={
          <ProtectedRoute requireApproved={false}>
            <Profile />
          </ProtectedRoute>
        } />

        <Route path="/admin" element={
          <ProtectedRoute requireAdmin>
            <Admin />
          </ProtectedRoute>
        } />

        <Route path="/admin/sessions" element={
          <ProtectedRoute requireAdmin>
            <AdminSessions />
          </ProtectedRoute>
        } />
      </Route>
    </Routes>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  );
}
