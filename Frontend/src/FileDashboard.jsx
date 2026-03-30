import { useState, useEffect, useRef } from 'react';
import { UploadCloud, Trash2, Download, Edit2, FileText, Check, X, ServerCrash, LayoutDashboard } from 'lucide-react';

export default function FileDashboard({ nodes }) {
  const [files, setFiles] = useState([]);
  const [logs, setLogs] = useState([]);
  const [leaderNode, setLeaderNode] = useState(null);
  const [dragging, setDragging] = useState(false);
  const [editingFile, setEditingFile] = useState(null);
  const [newName, setNewName] = useState('');
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef();

  const clientId = sessionStorage.getItem('client_id');
  const [viewMode, setViewMode] = useState('grid');
  const [confirmModal, setConfirmModal] = useState({ show: false, fileName: '' });

  const getUserPersona = (id) => {
    const names = ["Voyager", "Explorer", "Sentinel", "Pioneer", "Navigator", "Titan"];
    const colors = ["--persona-1", "--persona-2", "--persona-3", "--persona-4", "--persona-5", "--persona-6"];

    let hash = 0;
    for (let i = 0; i < id.length; i++) {
      hash = id.charCodeAt(i) + ((hash << 5) - hash);
    }
    const name = names[Math.abs(hash) % names.length];
    const color = colors[Math.abs(hash) % colors.length];
    
    return { name: `${name} ${id.slice(-4)}`, color: `var(${color})` };
  };

  const findLeader = async () => {
    for (const node of nodes) {
      try {
        const res = await fetch(`http://${node}/api/consensus`, { signal: AbortSignal.timeout(1000) });
        if (res.ok) {
          const data = await res.json();
          if (data.state === 'Leader') {
            setLeaderNode(node);
            return node;
          }
        }
      } catch (e) { /* ignore offline nodes */ }
    }
    setLeaderNode(null);
    return null;
  };

  const fetchFiles = async () => {
    let currentLeader = leaderNode;
    if (!currentLeader) currentLeader = await findLeader();
    if (!currentLeader) return;

    try {
      const res = await fetch(`http://${currentLeader}/api/files`, { signal: AbortSignal.timeout(2000) });
      if (res.ok) {
        setFiles(await res.json());
      }
    } catch (e) {}
  };

  const fetchLogs = async () => {
    let allLogs = [];
    await Promise.allSettled(nodes.map(async (node) => {
      try {
        const res = await fetch(`http://${node}/api/logs`, { signal: AbortSignal.timeout(2000) });
        if (res.ok) {
          const data = await res.json();
          if (data && Array.isArray(data)) {
            allLogs.push(...data);
          }
        }
      } catch (e) {}
    }));

    const uniqueMap = new Map();
    allLogs.forEach(log => {
      const key = `${log.time}-${log.user}-${log.action}-${log.file}`;
      if (!uniqueMap.has(key)) uniqueMap.set(key, log);
    });

    const uniqueLogs = Array.from(uniqueMap.values());
    uniqueLogs.sort((a, b) => b.time.localeCompare(a.time));
    setLogs(uniqueLogs.slice(0, 100));
  };


  useEffect(() => {
    findLeader().then(() => {
      fetchFiles();
      fetchLogs();
    });
    const interval = setInterval(() => {
      findLeader();
      fetchFiles();
      fetchLogs();
    }, 2000);
    return () => clearInterval(interval);
  }, []);

  const handleAPIRequest = async (path, method, body, isFormData = false) => {
    let currentLeader = leaderNode;
    if (!currentLeader) currentLeader = await findLeader();
    if (!currentLeader) {
      alert("No active Raft Leader found. The cluster might be down or electing.");
      return false;
    }

    try {
      const headers = { 'X-User-ID': clientId };
      if (!isFormData) headers['Content-Type'] = 'application/json';

      const res = await fetch(`http://${currentLeader}${path}`, {
        method,
        headers,
        body: isFormData ? body : (body ? JSON.stringify(body) : null)
      });
      
      if (res.status === 409) {
        const errData = await res.json();
        console.warn("Leader changed to", errData.leader);
        setLeaderNode(errData.leader.replace('http://', ''));
        alert("Leader changed during request. Please try again.");
        return false;
      }
      
      if (!res.ok) throw new Error("Request failed");
      return true;
    } catch (e) {
      alert(`Action failed: ${e.message}`);
      return false;
    }
  };

  const handleUpload = async (e) => {
    e.preventDefault();
    const file = e.dataTransfer ? e.dataTransfer.files[0] : e.target.files[0];
    if (!file) return;

    setUploading(true);
    const formData = new FormData();
    formData.append('file', file);

    const success = await handleAPIRequest('/api/files/upload', 'POST', formData, true);
    if (success) {
      setTimeout(() => { fetchFiles(); fetchLogs(); }, 500);
    }
    setDragging(false);
    setUploading(false);
  };

  const handleDelete = async (name) => {
    setConfirmModal({ show: false, fileName: '' });
    const success = await handleAPIRequest(`/api/files/${name}`, 'DELETE', null);
    if (success) { fetchFiles(); fetchLogs(); }
  };

  const handleRename = async (oldName) => {
    if (!newName.trim() || newName === oldName) {
      setEditingFile(null);
      return;
    }
    const success = await handleAPIRequest(`/api/files/${oldName}/rename`, 'PATCH', { newName });
    if (success) {
      setEditingFile(null);
      fetchFiles();
      fetchLogs();
    }
  };

  const formatSize = (bytes) => {
    if (bytes === 0) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="file-dashboard">
      <div className="main-content">
        <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <div></div>
          <div className="leader-badge" style={{background: leaderNode ? 'rgba(35, 134, 54, 0.15)' : 'rgba(218, 54, 51, 0.15)', borderColor: leaderNode ? 'var(--success-color)' : 'var(--danger-color)', color: leaderNode ? 'var(--success-color)' : 'var(--danger-color)'}}>
            {leaderNode ? `Connected to Leader: ${leaderNode}` : <><ServerCrash size={14}/> NO ACTIVE LEADER</>}
          </div>
        </div>

        <div 
          className={`glass-panel upload-area ${dragging ? 'dragging' : ''}`}
          onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
          onDragLeave={() => setDragging(false)}
          onDrop={handleUpload}
          onClick={() => !uploading && fileInputRef.current.click()}
          style={{opacity: uploading ? 0.5 : 1}}
        >
          <input 
            type="file" 
            ref={fileInputRef} 
            style={{display: 'none'}} 
            onChange={handleUpload} 
          />
          <UploadCloud size={48} color="var(--accent-color)" style={{marginBottom: '16px'}} />
          <h3>{uploading ? 'Processing via Raft Leader...' : 'Drag & Drop Files Here'}</h3>
          <p style={{color: 'var(--text-secondary)'}}>
            All uploads are strictly routed through the Active Leader and replicated.
          </p>
        </div>

        <div className="glass-panel" style={{padding: '24px'}}>
          <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px'}}>
            <h3 style={{margin: 0}}>Cluster Files</h3>
            <div style={{display: 'flex', gap: '8px', background: 'rgba(0,0,0,0.2)', padding: '4px', borderRadius: '8px'}}>
              <button 
                onClick={() => setViewMode('grid')}
                className={`btn-icon ${viewMode === 'grid' ? 'active' : ''}`}
                style={{background: viewMode === 'grid' ? 'rgba(88, 166, 255, 0.2)' : 'transparent'}}
              >
                <LayoutDashboard size={18} />
              </button>
              <button 
                onClick={() => setViewMode('list')}
                className={`btn-icon ${viewMode === 'list' ? 'active' : ''}`}
                style={{background: viewMode === 'list' ? 'rgba(88, 166, 255, 0.2)' : 'transparent'}}
              >
                <FileText size={18} />
              </button>
            </div>
          </div>

          {files.length === 0 ? (
            <p style={{color: 'var(--text-secondary)', textAlign: 'center', padding: '40px 0'}}>No files in consensus yet.</p>
          ) : (
            viewMode === 'grid' ? (
              <div className="file-grid-container">
                {files.map(f => (
                  <div key={f.name} className="glass-panel file-card">
                    <div className="file-card-actions">
                       <button className="btn-icon" title="Rename" onClick={() => { setEditingFile(f.name); setNewName(f.name); }}>
                          <Edit2 size={16} />
                        </button>
                        <button className="btn-icon danger" title="Delete" onClick={() => setConfirmModal({ show: true, fileName: f.name })}>
                          <Trash2 size={16} />
                        </button>
                    </div>
                    <div className="file-card-icon">
                      <FileText size={48} color="var(--accent-color)" />
                    </div>
                    {editingFile === f.name ? (
                       <div style={{display: 'flex', gap: '4px', marginTop: '8px'}}>
                        <input 
                          autoFocus
                          value={newName} 
                          onChange={(e) => setNewName(e.target.value)}
                          onKeyDown={(e) => e.key === 'Enter' && handleRename(f.name)}
                          style={{
                            background: 'rgba(0,0,0,0.3)',
                            border: '1px solid var(--accent-color)',
                            color: '#fff',
                            padding: '4px 8px',
                            borderRadius: '4px',
                            fontSize: '0.8rem',
                            width: '120px'
                          }}
                        />
                        <button className="btn-icon" onClick={() => handleRename(f.name)}><Check size={14} color="var(--success-color)"/></button>
                      </div>
                    ) : (
                      <div className="file-card-name" title={f.name}>{f.name}</div>
                    )}
                    <div className="file-card-meta">{formatSize(f.size)} • {f.mod_time.split(' ')[1]}</div>
                    <a href={`http://${nodes[0]}/api/files/${f.name}/download`} download style={{marginTop: '16px', width: '100%'}}>
                      <button className="btn-premium primary" style={{width: '100%', padding: '8px', fontSize: '0.75rem'}}>
                        Download
                      </button>
                    </a>
                  </div>
                ))}
              </div>
            ) : (
              <table className="file-list">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Size</th>
                    <th>Last Modified</th>
                    <th style={{textAlign: 'right'}}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {files.map(f => (
                    <tr key={f.name}>
                      <td>
                        <div style={{display: 'flex', alignItems: 'center', gap: '12px'}}>
                          <FileText size={18} color="var(--text-secondary)" />
                          {editingFile === f.name ? (
                            <div style={{display: 'flex', gap: '8px'}}>
                              <input 
                                autoFocus
                                value={newName} 
                                onChange={(e) => setNewName(e.target.value)}
                                onKeyDown={(e) => e.key === 'Enter' && handleRename(f.name)}
                                style={{
                                  background: 'transparent',
                                  border: '1px solid var(--accent-color)',
                                  color: '#fff',
                                  padding: '4px 8px',
                                  borderRadius: '4px',
                                  outline: 'none'
                                }}
                              />
                              <button className="btn-icon" onClick={() => handleRename(f.name)}><Check size={16} color="var(--success-color)"/></button>
                              <button className="btn-icon" onClick={() => setEditingFile(null)}><X size={16} color="var(--danger-color)"/></button>
                            </div>
                          ) : (
                            <span style={{fontWeight: 500, color: '#fff'}}>{f.name}</span>
                          )}
                        </div>
                      </td>
                      <td style={{color: 'var(--text-secondary)'}}>{formatSize(f.size)}</td>
                      <td style={{color: 'var(--text-secondary)'}}>{f.mod_time}</td>
                      <td style={{textAlign: 'right'}}>
                        <div className="action-btns" style={{justifyContent: 'flex-end'}}>
                          <a href={`http://${nodes[0]}/api/files/${f.name}/download`} download>
                            <button className="btn-icon" title="Download"><Download size={18} /></button>
                          </a>
                          <button className="btn-icon" title="Rename" onClick={() => { setEditingFile(f.name); setNewName(f.name); }}>
                            <Edit2 size={18} />
                          </button>
                          <button className="btn-icon danger" title="Delete" onClick={() => setConfirmModal({ show: true, fileName: f.name })}>
                            <Trash2 size={18} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )
          )}
        </div>
      </div>

      <div className="glass-panel activity-log">
        <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px', paddingBottom: '8px', borderBottom: '1px solid var(--border-color)'}}>
          <h3 style={{margin: 0}}>Global Activity Log</h3>
        </div>
        {logs.length === 0 ? (
          <p style={{color: 'var(--text-secondary)'}}>No recent activity.</p>
        ) : (
          logs.map((log, i) => (
            <div key={i} className="log-entry">
              <span className="log-time">{log.time}</span>
              <div style={{flex: 1}}>
                <span className="user-badge" style={{
                  background: 'rgba(255,255,255,0.05)', 
                  color: getUserPersona(log.user).color,
                  border: `1px solid ${getUserPersona(log.user).color}44`
                }}>
                  {getUserPersona(log.user).name}
                </span>
                <span style={{color: 'var(--text-secondary)'}}> {log.action} </span>
                {log.file && <span style={{color: '#fff', fontWeight: 500}}>{log.file}</span>}
                {log.details && <div style={{fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '4px'}}>{log.details}</div>}
              </div>
            </div>
          ))
        )}
      </div>
      {confirmModal.show && (
        <div className="modal-backdrop">
          <div className="glass-panel modal-content">
            <Trash2 size={48} color="var(--danger-color)" style={{marginBottom: '16px'}} />
            <h3>Delete File?</h3>
            <p style={{color: 'var(--text-secondary)', fontSize: '0.875rem'}}>
              Are you sure you want to permanently delete <b style={{color: '#fff'}}>{confirmModal.fileName}</b> from the distributed cluster?
            </p>
            <div className="modal-actions">
              <button 
                className="btn-premium secondary" 
                onClick={() => setConfirmModal({ show: false, fileName: '' })}
              >
                Cancel
              </button>
              <button 
                className="btn-premium danger" 
                onClick={() => handleDelete(confirmModal.fileName)}
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
