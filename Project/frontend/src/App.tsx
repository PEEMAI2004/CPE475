import { useState, useEffect } from 'react';
import { Leaf, Cpu, Save, Plus, Trash2, Edit2, Server, Activity, Users, LogOut } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { GoogleOAuthProvider, GoogleLogin } from '@react-oauth/google';
import type { Profile, Device, ServiceHealth, User } from './api';
import { 
  getProfiles, getDevices, createProfile, updateProfile, deleteProfile, 
  updateDeviceProfile, getInfrastructureHealth, login, logout, 
  getUsers, inviteUser, deleteUser 
} from './api';

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
      <Dashboard user={user} onLogout={handleLogout} />
    </GoogleOAuthProvider>
  );
}

function Dashboard({ user, onLogout }: { user: { name: string, role: string }, onLogout: () => void }) {
  const [activeTab, setActiveTab] = useState<'devices' | 'profiles' | 'infrastructure' | 'users'>('devices');
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [infra, setInfra] = useState<ServiceHealth[]>([]);
  const [users, setUsers] = useState<User[]>([]);

  useEffect(() => {
    fetchData();
  }, [activeTab]);

  const fetchData = async () => {
    try {
      if (activeTab === 'devices') setDevices(await getDevices());
      if (activeTab === 'profiles') setProfiles(await getProfiles());
      if (activeTab === 'infrastructure') setInfra(await getInfrastructureHealth());
      if (activeTab === 'users' && user.role === 'Super Admin') setUsers(await getUsers());
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
          <div style={{ background: 'var(--accent)', padding: '0.75rem', borderRadius: '12px' }}>
            <Leaf size={28} color="white" />
          </div>
          <div>
            <h1 style={{ fontWeight: 600, fontSize: '1.5rem', margin: 0 }}>PotBuddy</h1>
            <p style={{ color: 'var(--text-sub)', margin: 0, fontSize: '0.9rem' }}>Welcome, {user.name} ({user.role})</p>
          </div>
        </div>
        
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          <div className="glass-panel nav-tabs">
            <div className={`tab ${activeTab === 'devices' ? 'active' : ''}`} onClick={() => setActiveTab('devices')}>
              <Cpu size={16} /> Devices
            </div>
            <div className={`tab ${activeTab === 'profiles' ? 'active' : ''}`} onClick={() => setActiveTab('profiles')}>
              <Leaf size={16} /> Profiles
            </div>
            <div className={`tab ${activeTab === 'infrastructure' ? 'active' : ''}`} onClick={() => setActiveTab('infrastructure')}>
              <Server size={16} /> Infra
            </div>
            {isSuperAdmin && (
              <div className={`tab ${activeTab === 'users' ? 'active' : ''}`} onClick={() => setActiveTab('users')}>
                <Users size={16} /> Users
              </div>
            )}
          </div>
          <button className="btn" style={{ background: 'rgba(255,255,255,0.1)', padding: '0.6rem' }} onClick={onLogout}>
            <LogOut size={18} />
          </button>
        </div>
      </header>

      <main>
        <AnimatePresence mode="wait">
          {activeTab === 'devices' ? (
            <motion.div key="devices" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="glass-panel" style={{ padding: '1.5rem' }}>
                <h2 style={{ marginBottom: '1.5rem', fontWeight: 500 }}>Mapped Hardware Nodes</h2>
                <div style={{ overflowX: 'auto' }}>
                  <table>
                    <thead>
                      <tr>
                        <th>Device ID</th>
                        <th>Status</th>
                        <th>Health</th>
                        <th>Assigned Profile</th>
                      </tr>
                    </thead>
                    <tbody>
                      {devices.map(d => (
                        <tr key={d.device_id}>
                          <td style={{ fontWeight: 500 }}>{d.device_id}</td>
                          <td>{d.online ? <span className="badge">Online</span> : <span className="badge" style={{background: 'rgba(255,255,255,0.1)', color: '#aaa'}}>Offline</span>}</td>
                          <td>
                            {d.health === 'healthy' && <span className="badge" style={{background: '#4ade80', color: 'white'}}>Healthy</span>}
                            {d.health === 'warning' && <span className="badge" style={{background: '#fbbf24', color: '#1f2937'}}>Warning</span>}
                            {d.health === 'critical' && <span className="badge" style={{background: '#ef4444', color: 'white'}}>Critical</span>}
                            {d.health === 'unknown' && <span className="badge" style={{background: 'rgba(255,255,255,0.1)', color: '#aaa'}}>Unknown</span>}
                          </td>
                          <td>
                            <select 
                              className="input-field" 
                              value={d.profile_id}
                              onChange={(e) => handleDeviceUpdate(d.device_id, Number(e.target.value))}
                              disabled={!isAtLeastSiteAdmin}
                            >
                              {profiles.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
                            </select>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </motion.div>
          ) : activeTab === 'infrastructure' ? (
            <motion.div key="infrastructure" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <div className="glass-panel" style={{ padding: '1.5rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                  <h2 style={{ fontWeight: 500 }}>System Infrastructure</h2>
                  <button className="btn" onClick={fetchData}><Activity size={18} /> Refresh</button>
                </div>
                <div className="grid">
                  {infra.map((s, idx) => (
                    <div key={idx} style={{ background: 'rgba(0,0,0,0.2)', padding: '1.5rem', borderRadius: '12px' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
                        <h4 style={{ margin: 0, fontWeight: 600 }}>{s.name}</h4>
                        <span style={{ color: s.status === 'online' ? '#4ade80' : '#f87171', fontSize: '0.85rem', fontWeight: 600 }}>● {s.status}</span>
                      </div>
                      <div style={{ fontSize: '0.8rem', color: 'var(--text-sub)', textTransform: 'uppercase' }}>Type: {s.type}</div>
                      <div style={{ fontFamily: 'monospace', fontSize: '0.8rem', color: 'rgba(255,255,255,0.7)', overflow: 'hidden', textOverflow: 'ellipsis' }}>{s.address}</div>
                    </div>
                  ))}
                </div>
              </div>
            </motion.div>
          ) : activeTab === 'users' ? (
            <motion.div key="users" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <UserManagement users={users} reload={fetchData} />
            </motion.div>
          ) : (
            <motion.div key="profiles" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}>
              <ProfileManager profiles={profiles} reload={fetchData} canEdit={isAtLeastSiteAdmin} isSuperAdmin={isSuperAdmin} />
            </motion.div>
          )}
        </AnimatePresence>
      </main>
    </div>
  );
}

function UserManagement({ users, reload }: { users: User[], reload: () => void }) {
  const [email, setEmail] = useState('');
  const [role, setRole] = useState('Viewer');

  const handleInvite = async () => {
    if (!email) return;
    try {
      await inviteUser(email, role);
      setEmail('');
      reload();
    } catch (e) {
      alert("Failed to invite user.");
    }
  };

  const handleDelete = async (id: number) => {
    if (confirm('Remove this user?')) {
      await deleteUser(id);
      reload();
    }
  };

  return (
    <div className="glass-panel" style={{ padding: '1.5rem' }}>
      <h2 style={{ marginBottom: '1.5rem', fontWeight: 500 }}>Access Control & Invitations</h2>
      <div style={{ display: 'flex', gap: '1rem', marginBottom: '2rem', background: 'rgba(0,0,0,0.2)', padding: '1.5rem', borderRadius: '12px' }}>
        <div style={{ flex: 1 }}>
          <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Email Address</label>
          <input className="input-field" value={email} onChange={e => setEmail(e.target.value)} placeholder="user@company.com" />
        </div>
        <div style={{ width: '200px' }}>
          <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Assigned Role</label>
          <select className="input-field" value={role} onChange={e => setRole(e.target.value)}>
            <option>Super Admin</option>
            <option>Site Admin</option>
            <option>Viewer</option>
          </select>
        </div>
        <div style={{ alignSelf: 'flex-end' }}>
          <button className="btn" onClick={handleInvite}><Plus size={18} /> Invite User</button>
        </div>
      </div>

      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Role</th>
            <th>Joined</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {users.map(u => (
            <tr key={u.id}>
              <td>{u.name || <span style={{opacity: 0.3}}>Pending...</span>}</td>
              <td>{u.email}</td>
              <td><span className="badge">{u.role}</span></td>
              <td style={{ fontSize: '0.85rem' }}>{new Date(u.created_at).toLocaleDateString()}</td>
              <td>
                <button className="btn danger" style={{ padding: '0.4rem' }} onClick={() => handleDelete(u.id)}><Trash2 size={14} /></button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ProfileManager({ profiles, reload, canEdit, isSuperAdmin }: { profiles: Profile[], reload: () => void, canEdit: boolean, isSuperAdmin: boolean }) {
  const [editingId, setEditingId] = useState<number | null>(null);
  const getProfile = (id: number) => profiles.find(p => p.id === id);

  const handleDelete = async (id: number) => {
    if (confirm('Are you sure?')) {
      await deleteProfile(id);
      reload();
    }
  };

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h2 style={{ fontWeight: 500 }}>Health Logic Profiles</h2>
        {!editingId && canEdit && (
          <button className="btn" onClick={() => setEditingId(-1)}><Plus size={18} /> New Profile</button>
        )}
      </div>

      {editingId !== null ? (
        <ProfileEditor 
          profile={getProfile(editingId)} 
          onCancel={() => setEditingId(null)}
          onSave={async (p: Partial<Profile>) => {
            if (editingId === -1) await createProfile(p);
            else await updateProfile(editingId, p);
            setEditingId(null);
            reload();
          }}
        />
      ) : (
        <div className="grid">
          {profiles.map(p => (
            <div key={p.id} className="glass-panel" style={{ padding: '1.5rem', display: 'flex', flexDirection: 'column' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '1rem' }}>
                <h3 style={{ textTransform: 'capitalize', fontWeight: 600 }}>{p.name}</h3>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  {canEdit && (
                    <button className="btn" style={{ padding: '0.4rem', background: 'rgba(255,255,255,0.1)' }} onClick={() => setEditingId(p.id)}><Edit2 size={14} /></button>
                  )}
                  {isSuperAdmin && p.name !== 'default' && (
                    <button className="btn danger" style={{ padding: '0.4rem' }} onClick={() => handleDelete(p.id)}><Trash2 size={14} /></button>
                  )}
                </div>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', flex: 1 }}>
                <BoundSummary title="Soil" inner={[p.soil_inner_low, p.soil_inner_high]} outer={[p.soil_outer_low, p.soil_outer_high]} />
                <BoundSummary title="Temp" inner={[p.temp_inner_low, p.temp_inner_high]} outer={[p.temp_outer_low, p.temp_outer_high]} />
                <BoundSummary title="Hum"  inner={[p.hum_inner_low, p.hum_inner_high]} outer={[p.hum_outer_low, p.hum_outer_high]} />
                <BoundSummary title="Light" inner={[p.light_inner_low, p.light_inner_high]} outer={[p.light_outer_low, p.light_outer_high]} />
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

function BoundSummary({ title, inner, outer }: any) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.875rem' }}>
      <span style={{ color: 'var(--text-sub)' }}>{title}</span>
      <span>{inner[0]}-{inner[1]} <span style={{ opacity: 0.3 }}>|</span> {outer[0]}-{outer[1]}</span>
    </div>
  )
}

function ProfileEditor({ profile, onCancel, onSave }: any) {
  const [form, setForm] = useState<Partial<Profile>>(profile || {
    name: '',
    soil_inner_low: 1500, soil_inner_high: 2500, soil_outer_low: 1000, soil_outer_high: 3000,
    temp_inner_low: 18, temp_inner_high: 30, temp_outer_low: 15, temp_outer_high: 35,
    hum_inner_low: 40, hum_inner_high: 70, hum_outer_low: 30, hum_outer_high: 80,
    light_inner_low: 2000, light_inner_high: 50000, light_outer_low: 500, light_outer_high: 80000,
  });
  const update = (field: keyof Profile, value: any) => setForm(f => ({ ...f, [field]: value }));

  return (
    <motion.div className="glass-panel" style={{ padding: '2rem' }} initial={{ scale: 0.98, opacity: 0 }} animate={{ scale: 1, opacity: 1 }}>
      <h3 style={{ marginBottom: '1.5rem' }}>{profile ? 'Edit Profile' : 'New Profile'}</h3>
      <div style={{ marginBottom: '1.5rem' }}>
        <label style={{ display: 'block', marginBottom: '0.5rem', color: 'var(--text-sub)', fontSize: '0.9rem' }}>Profile Name</label>
        <input className="input-field" value={form.name} onChange={e => update('name', e.target.value)} placeholder="e.g. tropical" disabled={form.name === 'default' && profile} />
      </div>
      <div className="grid">
        {(['soil', 'temp', 'hum', 'light'] as const).map(group => (
          <div key={group} style={{ background: 'rgba(0,0,0,0.2)', padding: '1.5rem', borderRadius: '12px' }}>
            <h4 style={{ textTransform: 'capitalize', marginBottom: '1rem', color: 'var(--accent)' }}>{group} Thresholds</h4>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
              <div><label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Inner Low</label><input type="number" className="input-field" value={(form as any)[`${group}_inner_low`]} onChange={e => update(`${group}_inner_low` as any, Number(e.target.value))} /></div>
              <div><label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Inner High</label><input type="number" className="input-field" value={(form as any)[`${group}_inner_high`]} onChange={e => update(`${group}_inner_high` as any, Number(e.target.value))} /></div>
              <div><label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Outer Low</label><input type="number" className="input-field" value={(form as any)[`${group}_outer_low`]} onChange={e => update(`${group}_outer_low` as any, Number(e.target.value))} /></div>
              <div><label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Outer High</label><input type="number" className="input-field" value={(form as any)[`${group}_outer_high`]} onChange={e => update(`${group}_outer_high` as any, Number(e.target.value))} /></div>
            </div>
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '1rem', marginTop: '2rem' }}>
        <button className="btn" style={{ background: 'transparent', border: '1px solid var(--panel-border)' }} onClick={onCancel}>Cancel</button>
        <button className="btn" onClick={() => onSave(form)}><Save size={18} /> Save Settings</button>
      </div>
    </motion.div>
  )
}

export default App;
