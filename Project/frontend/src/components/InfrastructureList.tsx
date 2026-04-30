import { Activity } from 'lucide-react';
import type { ServiceHealth } from '../api';

export default function InfrastructureList({ infra, reload }: { infra: ServiceHealth[], reload: () => void }) {
  return (
    <div className="glass-panel" style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
        <h2 style={{ fontWeight: 500 }}>System Infrastructure</h2>
        <button className="btn" onClick={reload}><Activity size={18} /> Refresh</button>
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
  );
}