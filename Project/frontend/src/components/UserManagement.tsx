import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import type { User } from '../api';
import { inviteUser, deleteUser } from '../api';

export default function UserManagement({ users, reload }: { users: User[], reload: () => void }) {
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
