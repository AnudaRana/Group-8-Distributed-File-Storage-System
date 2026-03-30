import { BrowserRouter as Router, Routes, Route, NavLink } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Server, LayoutDashboard, FolderKanban } from 'lucide-react';
import FileDashboard from './FileDashboard';
import CommandCenter from './CommandCenter';
import './index.css';

const SEED_NODES = ['localhost:8001', 'localhost:8002', 'localhost:8003', 'localhost:8004', 'localhost:8005'];

function App() {
  const [nodes, setNodes] = useState(SEED_NODES);
  useEffect(() => {
    if (!sessionStorage.getItem('client_id')) {
      const id = 'user_' + Math.random().toString(36).substring(2, 6);
      sessionStorage.setItem('client_id', id);
    }

    const fetchNodes = async () => {
      const promises = SEED_NODES.map(url => fetch(`http://${url}/api/nodes`).then(res => {
        if (!res.ok) throw new Error('Not ok');
        return res.json();
      }));

      try {
        const discovered = await Promise.any(promises);
        if (discovered && discovered.length > 0) {
          setNodes(discovered);
        }
      } catch (e) {
      }
    };

    fetchNodes();
    const interval = setInterval(fetchNodes, 2000);
    return () => clearInterval(interval);
  }, []);

  return (
    <Router>
      <div className="app-container">
        <nav className="glass-panel nav-bar">
          <div className="nav-logo">
            <Server className="text-accent" size={24} color="#58a6ff" />
            Distributed Storage System
          </div>
          
          <div className="nav-links">
            <NavLink to="/" className={({isActive}) => isActive ? "nav-link active" : "nav-link"} style={{display: 'flex', gap: '8px', alignItems: 'center'}}>
              <FolderKanban size={18} /> File Manager
            </NavLink>
            <NavLink to="/command-center" className={({isActive}) => isActive ? "nav-link active" : "nav-link"} style={{display: 'flex', gap: '8px', alignItems: 'center'}}>
              <LayoutDashboard size={18} /> Command Center
            </NavLink>
          </div>
          
          <div className="node-selector" style={{color: 'var(--success-color)', fontWeight: 600}}>
            Cluster Active
          </div>
        </nav>

        <Routes>
          <Route path="/" element={<FileDashboard nodes={nodes} />} />
          <Route path="/command-center" element={<CommandCenter nodes={nodes} />} />
        </Routes>
      </div>
    </Router>
  );
}

export default App;
