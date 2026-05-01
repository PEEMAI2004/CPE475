import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Plus, Save, Trash2, Edit2, Shield, Copy, RefreshCw, Key, BookOpen } from 'lucide-react';
import type { InfrastructureNode, EnrolledDevice } from '../api';
import { getEnrolledNodes, getEnrolledDevices, enrollNode, updateEnrolledNode, deleteEnrolledNode, enrollDevice, deleteEnrolledDevice, regenNodeToken, regenDeviceAuthToken } from '../api';

export default function EnrollmentManagement({ isSuperAdmin }: { isSuperAdmin: boolean }) {
  const [nodes, setNodes] = useState<InfrastructureNode[]>([]);
  const [devices, setDevices] = useState<EnrolledDevice[]>([]);
  
  const [newNode, setNewNode] = useState({ name: '', site_id: 0, address: '', mqtt_address: '' });
  const [editingNodeId, setEditingNodeId] = useState<number | null>(null);
  const [newDevice, setNewDevice] = useState({ device_id: '' });

  const [showInstructions, setShowInstructions] = useState(false);

  useEffect(() => {
    fetchEnrollments();
  }, []);

  const fetchEnrollments = async () => {
    try {
      if (isSuperAdmin) setNodes(await getEnrolledNodes());
      setDevices(await getEnrolledDevices());
    } catch (e) {
      console.error(e);
    }
  };

  const handleEnrollNode = async () => {
    if (!newNode.name || !newNode.address) return;
    try {
      if (editingNodeId !== null) {
        await updateEnrolledNode(editingNodeId, newNode);
        setEditingNodeId(null);
      } else {
        await enrollNode(newNode.name, 'Local Node', newNode.site_id, newNode.address, newNode.mqtt_address);
      }
      setNewNode({ name: '', site_id: 0, address: '', mqtt_address: '' });
      fetchEnrollments();
    } catch (e) {
      alert("Failed to save site.");
    }
  };

  const handleEditNode = (node: InfrastructureNode) => {
    setEditingNodeId(node.id);
    setNewNode({
      name: node.name,
      site_id: node.site_id,
      address: node.address,
      mqtt_address: node.mqtt_address || ''
    });
  };

  const cancelEdit = () => {
    setEditingNodeId(null);
    setNewNode({ name: '', site_id: 0, address: '', mqtt_address: '' });
  };

  const handleEnrollDevice = async () => {
    if (!newDevice.device_id) return;
    try {
      const bundle = await enrollDevice(newDevice.device_id);
      
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', `device-${newDevice.device_id}-bundle.json`);
      document.body.appendChild(link);
      link.click();
      link.remove();

      setNewDevice({ device_id: '' });
      fetchEnrollments();
      alert("Device enrolled! Certificate bundle downloaded.");
    } catch (e) {
      alert("Failed to enroll device.");
    }
  };

  const handleDeleteNode = async (id: number) => {
    if (confirm('De-enroll this site?')) {
      await deleteEnrolledNode(id);
      fetchEnrollments();
    }
  };

  const handleDeleteDevice = async (id: string) => {
    if (confirm('De-enroll this device?')) {
      await deleteEnrolledDevice(id);
      fetchEnrollments();
    }
  };

  const handleRegenNodeToken = async (id: number) => {
    if (confirm('Regenerate token for this site? Existing gateway config will need update.')) {
      try {
        await regenNodeToken(id);
        fetchEnrollments();
      } catch (e) {
        alert("Failed to regenerate token.");
      }
    }
  };

  const handleRegenDeviceAuthToken = async (id: string) => {
    if (confirm('Regenerate AuthToken for this device? It will need to be re-provisioned if reset.')) {
      try {
        await regenDeviceAuthToken(id);
        fetchEnrollments();
      } catch (e) {
        alert("Failed to regenerate token.");
      }
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    alert("Copied to clipboard!");
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn" style={{ background: 'rgba(74, 222, 128, 0.2)', color: '#4ade80' }} onClick={() => setShowInstructions(!showInstructions)}>
          <BookOpen size={18} /> Enrollment Guide
        </button>
      </div>

      <AnimatePresence>
        {showInstructions && (
          <motion.div 
            initial={{ height: 0, opacity: 0 }} 
            animate={{ height: 'auto', opacity: 1 }} 
            exit={{ height: 0, opacity: 0 }}
            className="glass-panel" 
            style={{ padding: '1.5rem', overflow: 'hidden', border: '1px solid var(--accent)' }}
          >
            <h3 style={{ marginBottom: '1rem', color: 'var(--accent)' }}>Edge Site Enrollment Guide</h3>
            <ol style={{ paddingLeft: '1.5rem', lineHeight: '1.6', color: 'var(--text-sub)' }}>
              <li><strong>Register Site:</strong> Fill in the "Infrastructure Enrollment" form below.</li>
              <li><strong>Zero-Trust Provision (Recommended):</strong> 
                <p style={{ marginTop: '0.4rem', fontSize: '0.9rem' }}>Run the enrollment tool on your edge server using the unique site token:</p>
                <code style={{ display: 'block', background: 'rgba(0,0,0,0.3)', padding: '0.8rem', borderRadius: '4px', margin: '0.5rem 0', fontSize: '0.8rem', color: '#4ade80', border: '1px solid rgba(74, 222, 128, 0.2)' }}>
                  ./enroll -token YOUR_SITE_TOKEN
                </code>
                This automatically generates keys locally and fetches both certificates and <code>config.yaml</code>.
              </li>
              <li><strong>Manual Setup:</strong> If automated enrollment is not possible, contact an administrator for a manual configuration bundle.</li>
            </ol>
            <h3 style={{ marginTop: '1.5rem', marginBottom: '1rem', color: 'var(--accent)' }}>ESP32 Device Enrollment (mTLS)</h3>
            <ol style={{ paddingLeft: '1.5rem', lineHeight: '1.6', color: 'var(--text-sub)' }}>
              <li><strong>Register Device:</strong> Enter a unique hardware ID below and click <b>Generate Auth Token</b>.</li>
              <li><strong>Provision:</strong> Connect to the ESP32's setup WiFi and enter the one-time <b>AuthToken</b>.</li>
              <li><strong>Secure Handshake:</strong> The device will automatically fetch its mTLS certificates from this API and begin publishing to its local broker.</li>
            </ol>
          </motion.div>
        )}
      </AnimatePresence>

      {isSuperAdmin && (
        <div className="glass-panel" style={{ padding: '1.5rem' }}>
          <h2 style={{ marginBottom: '1.5rem', fontWeight: 500 }}>Infrastructure Enrollment</h2>
          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginBottom: '2rem', background: 'rgba(0,0,0,0.2)', padding: '1.5rem', borderRadius: '12px' }}>
            <div style={{ flex: '1 1 200px' }}>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Site Name</label>
              <input className="input-field" value={newNode.name} onChange={e => setNewNode({...newNode, name: e.target.value})} placeholder="e.g. Bangkok Warehouse" />
            </div>
            <div style={{ width: '100px' }}>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Site ID</label>
              <input type="number" className="input-field" value={newNode.site_id} onChange={e => setNewNode({...newNode, site_id: Number(e.target.value)})} />
            </div>
            <div style={{ flex: '1 1 200px' }}>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Node Address (IP/Domain)</label>
              <input className="input-field" value={newNode.address} onChange={e => setNewNode({...newNode, address: e.target.value})} placeholder="e.g. debian-0.iot.kaminjitt.com" />
            </div>
            <div style={{ flex: '1 1 200px' }}>
              <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>MQTT Address (Optional)</label>
              <input className="input-field" value={newNode.mqtt_address} onChange={e => setNewNode({...newNode, mqtt_address: e.target.value})} placeholder="Fallback to Node Address" />
            </div>
            <div style={{ alignSelf: 'flex-end', display: 'flex', gap: '0.5rem' }}>
              {editingNodeId && <button className="btn" style={{ background: 'rgba(255,255,255,0.1)' }} onClick={cancelEdit}>Cancel</button>}
              <button className="btn" onClick={handleEnrollNode}>
                {editingNodeId ? <Save size={18} /> : <Plus size={18} />} 
                {editingNodeId ? ' Save Changes' : ' Enroll Site'}
              </button>
            </div>
          </div>

          <table>
            <thead>
              <tr>
                <th>Site Name</th>
                <th>Site ID</th>
                <th>Node Addr</th>
                <th>MQTT Addr</th>
                <th>Token</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map(n => (
                <tr key={n.id}>
                  <td style={{fontWeight: 600}}>{n.name}</td>
                  <td>Site {n.site_id}</td>
                  <td style={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{n.address}</td>
                  <td style={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{n.mqtt_address || <span style={{opacity: 0.3}}>(node address)</span>}</td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <span style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                        {'••••••••••••••••'}
                      </span>
                      <button className="btn-icon" title="Copy Token" onClick={() => copyToClipboard(n.token)} style={{ background: 'none', border: 'none', color: 'var(--text-sub)', cursor: 'pointer', padding: 0 }}>
                        <Copy size={14} />
                      </button>
                      <button className="btn-icon" title="Regenerate Token" onClick={() => handleRegenNodeToken(n.id)} style={{ background: 'none', border: 'none', color: 'var(--text-sub)', cursor: 'pointer', padding: 0 }}>
                        <RefreshCw size={14} />
                      </button>
                    </div>
                  </td>
                  <td>
                    <div style={{ display: 'flex', gap: '0.4rem' }}>
                      <button className="btn" style={{ padding: '0.4rem', background: 'rgba(74, 222, 128, 0.2)', color: '#4ade80' }} title="Copy Node Enrollment Command" onClick={() => copyToClipboard(`./enroll -token ${n.token}`)}><Key size={14} /></button>
                      <button className="btn" style={{ padding: '0.4rem', background: 'rgba(59, 130, 246, 0.2)', color: '#60a5fa' }} title="Copy MQTT Enrollment Command" onClick={() => copyToClipboard(`./enroll -type mqtt -cn ${n.mqtt_address || n.address} -token ${n.token}`)}><Shield size={14} /></button>
                      <button className="btn" style={{ padding: '0.4rem', background: 'rgba(255,255,255,0.1)' }} onClick={() => handleEditNode(n)}><Edit2 size={14} /></button>
                      <button className="btn danger" style={{ padding: '0.4rem' }} onClick={() => handleDeleteNode(n.id)}><Trash2 size={14} /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="glass-panel" style={{ padding: '1.5rem' }}>
        <h2 style={{ marginBottom: '1.5rem', fontWeight: 500 }}>IoT Device Enrollment (ESP32)</h2>
        <div style={{ display: 'flex', gap: '1rem', marginBottom: '2rem', background: 'rgba(0,0,0,0.2)', padding: '1.5rem', borderRadius: '12px' }}>
          <div style={{ flex: 1 }}>
            <label style={{ fontSize: '0.8rem', color: 'var(--text-sub)' }}>Device ID (Unique)</label>
            <input className="input-field" value={newDevice.device_id} onChange={e => setNewDevice({...newDevice, device_id: e.target.value})} placeholder="e.g. living-room-fern" />
          </div>
          <div style={{ alignSelf: 'flex-end' }}>
            <button className="btn" onClick={handleEnrollDevice}><Key size={18} /> Generate Auth Token</button>
          </div>
        </div>

        <table>
          <thead>
            <tr>
              <th>Device ID</th>
              <th>Auth Token</th>
              <th>Security</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {devices.map(d => (
              <tr key={d.device_id}>
                <td style={{ fontWeight: 600 }}>{d.device_id}</td>
                <td>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <span style={{ fontFamily: 'monospace' }}>
                      {'••••••••••••••••'}
                    </span>
                    <button className="btn-icon" title="Copy Auth Token" onClick={() => copyToClipboard(d.auth_token)} style={{ background: 'none', border: 'none', color: 'var(--text-sub)', cursor: 'pointer', padding: 0 }}>
                      <Copy size={14} />
                    </button>
                    <button className="btn-icon" title="Regenerate AuthToken" onClick={() => handleRegenDeviceAuthToken(d.device_id)} style={{ background: 'none', border: 'none', color: 'var(--text-sub)', cursor: 'pointer', padding: 0 }}>
                      <RefreshCw size={14} />
                    </button>
                  </div>
                </td>
                <td><span className="badge" style={{background: 'rgba(74, 222, 128, 0.1)', color: '#4ade80', border: '1px solid rgba(74, 222, 128, 0.2)'}}>mTLS</span></td>
                <td style={{ fontSize: '0.85rem' }}>{new Date(d.created_at).toLocaleString()}</td>
                <td>
                  <div style={{ display: 'flex', gap: '0.4rem' }}>
                    <button className="btn danger" style={{ padding: '0.4rem' }} onClick={() => handleDeleteDevice(d.device_id)}><Trash2 size={14} /></button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <p style={{ marginTop: '1rem', fontSize: '0.85rem', color: 'var(--text-sub)' }}>
          * Copy the <b>AuthToken</b> and use it in the ESP32 Captive Portal during device setup.
        </p>
      </div>
    </div>
  );
}