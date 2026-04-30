import { useState, useEffect } from 'react';
import { Leaf, Cpu, Server, Users, LogOut, ShieldCheck } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { GoogleOAuthProvider, GoogleLogin } from '@react-oauth/google';
import { BrowserRouter, Routes, Route, Navigate, NavLink, useNavigate, useLocation } from 'react-router-dom';
import type { Profile, Device, ServiceHealth, User } from './api';
import { 
  getProfiles, getDevices, updateDeviceProfile, getInfrastructureHealth, login, logout, getUsers
} from './api';

import DeviceList from './components/DeviceList';
import InfrastructureList from './components/InfrastructureList';
import UserManagement from './components/UserManagement';
import EnrollmentManagement from './components/EnrollmentManagement';
import ProfileManager from './components/ProfileManager';

const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID;

if (!GOOGLE_CLIENT_ID) {
  console.error("VITE_GOOGLE_CLIENT_ID is not defined in the environment!");
}

function App() {
  const [user, setUser] = useState<{ name: string, role: string } | null>(() => {
    const saved = localStorage.getItem('user');
    return saved ? JSON.parse(saved) : null;
  });

  const handleLoginSuccess = async (response: any) => {
    try {
      const data = await login(response.credential);
      setUser({ name: data.name, role: data.role });
    } catch (e) {
      alert("Login failed: User not invited or invalid token.");
    }
  };

  const handleLogout = () => {
    logout();
    setUser(null);
  };

  if (!user) {
    return (
      <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
        <div className="container" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '80vh' }}>
          <div className="glass-panel" style={{ padding: '3rem', textAlign: 'center', maxWidth: '400px' }}>
            <div style={{ background: 'var(--accent)', padding: '1rem', borderRadius: '16px', display: 'inline-block', marginBottom: '1.5rem' }}>
              <Leaf size={48} color="white" />
            </div>
            <h1 style={{ marginBottom: '0.5rem' }}>PotBuddy</h1>
            <p style={{ color: 'var(--text-sub)', marginBottom: '2rem' }}>Enterprise IoT Fleet Management. Please sign in to continue.</p>
            <div style={{ display: 'flex', justifyContent: 'center' }}>
              <GoogleLogin
                onSuccess={handleLoginSuccess}
                onError={() => console.log('Login Failed')}
                useOneTap
              />
            </div>
          </div>
        </div>
      </GoogleOAuthProvider>
    );
  }

  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
      <BrowserRouter>
        <Dashboard user={user} onLogout={handleLogout} />
      </BrowserRouter>
    </GoogleOAuthProvider>
  );
}

function Dashboard({ user, onLogout }: { user: { name: string, role: string }, onLogout: () => void }) {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [infra, setInfra] = useState<ServiceHealth[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    fetchData();
  }, [location.pathname]);

  const fetchData = async () => {
    try {
      if (location.pathname === '/devices' || location.pathname === '/') setDevices(await getDevices());
      if (location.pathname === '/profiles') setProfiles(await getProfiles());
      if (location.pathname === '/infrastructure') setInfra(await getInfrastructureHealth());
      if (location.pathname === '/users' && user.role === 'Super Admin') setUsers(await getUsers());
    } catch (e) {
      console.error("Failed to fetch data:", e);
    }
  };

  const handleDeviceUpdate = async (deviceId: string, profileId: number) => {
    try {
      await updateDeviceProfile(deviceId, profileId);
      setDevices(await getDevices());
    } catch (e) {
      console.error(e);
    }
  };

  const isSuperAdmin = user.role === 'Super Admin';
  const isAtLeastSiteAdmin = isSuperAdmin || user.role === 'Site Admin';

  return (
    <div className="container">
      <header className="header">
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <div style={{ background: 'var(--accent)', padding: '0.75rem', borderRadius: '12px', cursor: 'pointer' }} onClick={() => navigate('/')}>
            <Leaf size={28} color="white" />
          </div>
          <div>
            <h1 style={{ fontWeight: 600, fontSize: '1.5rem', margin: 0, cursor: 'pointer' }} onClick={() => navigate('/')}>PotBuddy</h1>
            <p style={{ color: 'var(--text-sub)', margin: 0, fontSize: '0.9rem' }}>Welcome, {user.name} ({user.role})</p>
          </div>
        </div>
        
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          <nav className="glass-panel nav-tabs">
            <NavLink to="/devices" className={({ isActive }) => `tab ${isActive || location.pathname === '/' ? 'active' : ''}`}>
              <Cpu size={16} /> Devices
            </NavLink>
            <NavLink to="/profiles" className={({ isActive }) => `tab ${isActive ? 'active' : ''}`}>
              <Leaf size={16} /> Profiles
            </NavLink>
            <NavLink to="/infrastructure" className={({ isActive }) => `tab ${isActive ? 'active' : ''}`}>
              <Server size={16} /> Infra
            </NavLink>
            {isAtLeastSiteAdmin && (
              <NavLink to="/enrollment" className={({ isActive }) => `tab ${isActive ? 'active' : ''}`}>
                <ShieldCheck size={16} /> Enrollment
              </NavLink>
            )}
            {isSuperAdmin && (
              <NavLink to="/users" className={({ isActive }) => `tab ${isActive ? 'active' : ''}`}>
                <Users size={16} /> Users
              </NavLink>
            )}
          </nav>
          <button className="btn" style={{ background: 'rgba(255,255,255,0.1)', padding: '0.6rem' }} onClick={onLogout}>
            <LogOut size={18} />
          </button>
        </div>
      </header>

      <main>
        <AnimatePresence mode="wait">
          <Routes location={location} key={location.pathname}>
            <Route path="/" element={<Navigate to="/devices" replace />} />
            <Route path="/devices" element={
              <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                <DeviceList devices={devices} profiles={profiles} isAtLeastSiteAdmin={isAtLeastSiteAdmin} onUpdate={handleDeviceUpdate} />
              </motion.div>
            } />
            <Route path="/profiles" element={
              <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                <ProfileManager profiles={profiles} reload={fetchData} canEdit={isAtLeastSiteAdmin} isSuperAdmin={isSuperAdmin} />
              </motion.div>
            } />
            <Route path="/infrastructure" element={
              <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                <InfrastructureList infra={infra} reload={fetchData} />
              </motion.div>
            } />
            {isAtLeastSiteAdmin && (
              <Route path="/enrollment" element={
                <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                  <EnrollmentManagement isSuperAdmin={isSuperAdmin} />
                </motion.div>
              } />
            )}
            {isSuperAdmin && (
              <Route path="/users" element={
                <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
                  <UserManagement users={users} reload={fetchData} />
                </motion.div>
              } />
            )}
            <Route path="*" element={<Navigate to="/devices" replace />} />
          </Routes>
        </AnimatePresence>
      </main>
    </div>
  );
}

export default App;
