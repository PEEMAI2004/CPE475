import type { Device, Profile } from '../api';

export default function DeviceList({ devices, profiles, isAtLeastSiteAdmin, onUpdate }: { devices: Device[], profiles: Profile[], isAtLeastSiteAdmin: boolean, onUpdate: (deviceId: string, profileId: number) => void }) {
  return (
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
                    onChange={(e) => onUpdate(d.device_id, Number(e.target.value))}
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
  );
}