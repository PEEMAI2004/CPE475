import { useState } from 'react';
import { motion } from 'framer-motion';
import { Plus, Edit2, Trash2, Save } from 'lucide-react';
import type { Profile } from '../api';
import { createProfile, updateProfile, deleteProfile } from '../api';

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

export default function ProfileManager({ profiles, reload, canEdit, isSuperAdmin }: { profiles: Profile[], reload: () => void, canEdit: boolean, isSuperAdmin: boolean }) {
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