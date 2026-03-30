import { useState, useEffect } from 'react';
import { Activity, Clock, ShieldAlert, GitMerge } from 'lucide-react';
import { LineChart, Line, ResponsiveContainer, YAxis, Tooltip } from 'recharts';

export default function CommandCenter({ nodes }) {
  const [clusterData, setClusterData] = useState({
    status: {},
    consensus: {},
    clock: {},
    replication: [],
    replicationMap: {},
    faultStats: {}
  });

  const fetchMetrics = async () => {
    // Fetch from all nodes concurrently
    await Promise.all(nodes.map(async (node) => {
      const baseUrl = `http://${node}/api`;
      try {
        const [stRes, coRes, clRes, reRes, fsRes] = await Promise.all([
          fetch(`${baseUrl}/status`, { signal: AbortSignal.timeout(2000) }).catch(()=>null),
          fetch(`${baseUrl}/consensus`, { signal: AbortSignal.timeout(2000) }).catch(()=>null),
          fetch(`${baseUrl}/clock`, { signal: AbortSignal.timeout(2000) }).catch(()=>null),
          fetch(`${baseUrl}/replication`, { signal: AbortSignal.timeout(2000) }).catch(()=>null),
          fetch(`${baseUrl}/fault-stats`, { signal: AbortSignal.timeout(2000) }).catch(()=>null)
        ]);
        
        const updates = {};
        
        if (stRes?.ok) {
          updates.status = await stRes.json();
        }
        if (coRes?.ok) {
          const co = await coRes.json();
          updates.consensus = { [node]: co };
        }
        if (clRes?.ok) {
          const cl = await clRes.json();
          updates.clock = { [node]: cl };
        }
        if (reRes?.ok) {
          const re = await reRes.json();
          updates.replication = { [node]: re };
        }
        if (fsRes?.ok) {
          const fs = await fsRes.json();
          updates.faultStats = { [node]: fs };
        }

        setClusterData(prev => {
          const newData = { ...prev };
          if (updates.status) Object.assign(newData.status, updates.status);
          if (updates.consensus) Object.assign(newData.consensus, updates.consensus);
          if (updates.clock) Object.assign(newData.clock, updates.clock);
          if (updates.replication) Object.assign(newData.replicationMap, updates.replication);
          if (updates.faultStats) Object.assign(newData.faultStats, updates.faultStats);

          const leaderNode = Object.keys(newData.consensus).find(n => newData.consensus[n]?.state === 'Leader');
          if (leaderNode && newData.replicationMap[leaderNode]) {
            newData.replication = newData.replicationMap[leaderNode];
          } else if (newData.replication.length === 0 && Object.keys(newData.replicationMap).length > 0) {
             // Fallback to first available if no leader found yet
             newData.replication = Object.values(newData.replicationMap)[0];
          }

          return newData;
        });
      } catch (e) { /* ignore offline node */ }
    }));
  };

  useEffect(() => {
    fetchMetrics();
    const interval = setInterval(fetchMetrics, 1000); 
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="grid-2x2">
      {/* PANEL 1: Fault Tolerance */}
      <div className="glass-panel panel">
        <div className="panel-header">
          <div className="panel-title">
            <ShieldAlert size={20} color="var(--accent-color)" /> Fault Tolerance
          </div>
        </div>
        
        <div style={{display: 'flex', flexDirection: 'column', gap: '8px'}}>
          {Object.entries(clusterData.status).sort().map(([nodeId, data]) => {
            const isDead = data.status === 'failed';
            let hbSent = 0;
            let gossipCount = 0;
            
            const nodeStat = Object.values(clusterData.faultStats).find(fs => fs.self_id === nodeId);
            if (nodeStat) {
              hbSent = nodeStat.heartbeats_sent;
            }
            const anyStat = Object.values(clusterData.faultStats)[0];
            if (anyStat && anyStat.gossip_rounds && anyStat.gossip_rounds[nodeId]) {
               gossipCount = anyStat.gossip_rounds[nodeId];
            }

            return (
              <div key={nodeId} style={{
                background: isDead ? 'rgba(248, 81, 73, 0.05)' : 'rgba(0,0,0,0.3)',
                border: `1px solid ${isDead ? 'rgba(248, 81, 73, 0.3)' : 'var(--border-color)'}`,
                padding: '12px', borderRadius: '8px', fontSize: '0.875rem'
              }}>
                <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                  <div style={{display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 600, color: '#fff'}}>
                    <span className={`status-dot ${isDead ? 'dead' : 'alive'}`}></span>
                    {nodeId} 
                    {data.self && <span style={{fontSize: '0.65rem', padding: '1px 4px', background: 'rgba(255,255,255,0.1)', borderRadius: '4px'}}>SELF</span>}
                  </div>
                  <span style={{color: isDead ? 'var(--danger-color)' : 'var(--success-color)', fontSize: '0.75rem', fontWeight: 600}}>
                    {isDead ? 'OFFLINE' : 'ALIVE'}
                  </span>
                </div>
                
                <div style={{display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '8px', borderTop: '1px dashed var(--border-color)', paddingTop: '8px'}}>
                  <span>Heartbeats Sent: <b style={{color: '#fff'}}>{hbSent || 0}</b></span>
                  <span>Gossip Rumours: <b style={{color: '#fff'}}>{gossipCount || 0}</b></span>
                </div>

                {data.state && (
                  <div style={{fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '4px'}}>
                    Recovery: <span style={{color: data.state === 'complete' ? 'var(--success-color)' : 'var(--warning-color)'}}>{data.state}</span>
                    {data.downtime && ` (downtime: ${data.downtime})`}
                  </div>
                )}
              </div>
            );
          })}
          {Object.keys(clusterData.status).length === 0 && (
             <p style={{color: 'var(--text-secondary)'}}>No nodes responding.</p>
          )}
        </div>
      </div>

      {/* PANEL 2: Consensus (Raft) */}
      <div className="glass-panel panel">
        <div className="panel-header">
          <div className="panel-title">
            <GitMerge size={20} color="#8957e5" /> Consensus (Raft)
          </div>
        </div>
        
        <div style={{display: 'flex', flexDirection: 'column', gap: '12px'}}>
          {nodes.map(node => {
            const data = clusterData.consensus[node];
            const isLeader = data?.state === 'Leader';
            let fallbackName = "node(?)";
            if (node) {
               const portMatch = node.match(/:(\d+)$/);
               if (portMatch) {
                 const p = portMatch[1];
                 fallbackName = `node${p.slice(-1)}`;
               }
            }
            const nodeName = data?.node_id || fallbackName;

            return (
              <div key={node} style={{
                background: isLeader ? 'rgba(210, 153, 34, 0.05)' : 'rgba(0,0,0,0.3)',
                border: `1px solid ${isLeader ? 'rgba(210, 153, 34, 0.3)' : 'var(--border-color)'}`,
                padding: '12px', borderRadius: '8px', fontSize: '0.875rem'
              }}>
                <div style={{display: 'flex', justifyContent: 'space-between', marginBottom: '8px'}}>
                  <span style={{fontWeight: 600, color: '#fff'}}>{nodeName}</span>
                  {isLeader ? (
                    <span className="leader-badge">LEADER</span>
                  ) : (
                    <span style={{color: 'var(--text-secondary)'}}>{data?.state || 'Offline'}</span>
                  )}
                </div>
                
                {data && (
                  <div style={{display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', color: 'var(--text-secondary)'}}>
                    <span>Term: <b>{data.term}</b></span>
                    <span>Logs: <b>{data.log_length}</b></span>
                    <span>Voted: <b>{data.voted_for || 'none'}</b></span>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* PANEL 3: Time Synchronization */}
      <div className="glass-panel panel">
        <div className="panel-header">
          <div className="panel-title">
            <Clock size={20} color="#3fb950" /> Time Synchronization
          </div>
        </div>
        
        {/* Pick the first node that is successfully returning clock data */}
        {(() => {
          const activeNode = nodes.find(n => clusterData.clock[n]);
          if (!activeNode) return <p style={{color: 'var(--text-secondary)'}}>No resilient clock data available.</p>;
          const data = clusterData.clock[activeNode];

          return (
            <div key={activeNode}>
              <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '24px'}}>
                <div>
                  <div style={{fontSize: '2rem', fontWeight: 700, lineHeight: 1}}>{data.offset_ms?.toFixed(2) || '0.00'} <span style={{fontSize: '1rem', color: 'var(--text-secondary)'}}>ms</span></div>
                  <div style={{fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '4px'}}>Offset from Reference Peer</div>
                </div>
                <div style={{textAlign: 'right'}}>
                  <div style={{fontSize: '0.875rem', color: data.synced ? 'var(--success-color)' : 'var(--danger-color)', fontWeight: 600}}>
                    {data.synced ? 'SYNKED TO PEER' : 'LOCAL CLOCK ONLY'}
                  </div>
                  <div style={{fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '4px'}}>Algorithm: Cristian&apos;s</div>
                </div>
              </div>

              <div>
                <div style={{fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '8px'}}>Skew Drift History (ns)</div>
                <div style={{height: '150px', width: '100%', background: 'rgba(0,0,0,0.2)', borderRadius: '8px', padding: '8px 0'}}>
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={data.skew_history || []}>
                      <YAxis domain={['auto', 'auto']} tick={{fontSize: 10, fill: 'var(--text-secondary)'}} width={80} />
                      <Tooltip 
                        contentStyle={{background: 'var(--panel-bg)', border: '1px solid var(--border-color)', borderRadius: '4px'}}
                        labelStyle={{display: 'none'}}
                      />
                      <Line type="monotone" dataKey="skew_ns" stroke="#3fb950" strokeWidth={2} dot={false} isAnimationActive={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </div>
            </div>
          );
        })()}
      </div>

      {/* PANEL 4: Data Replication */}
      <div className="glass-panel panel" style={{overflow: 'hidden'}}>
        <div className="panel-header">
          <div className="panel-title">
            <Activity size={20} color="#f85149" /> Data Replication Map
          </div>
        </div>
        
        {(!clusterData.replication || clusterData.replication.length === 0) ? (
          <div style={{textAlign: 'center', color: 'var(--text-secondary)', padding: '40px 0'}}>
            No files available.
          </div>
        ) : (
          <div style={{overflowY: 'auto', maxHeight: '280px'}}>
            <table className="file-list" style={{fontSize: '0.875rem'}}>
              <thead>
                <tr>
                  <th style={{padding: '8px 12px'}}>File</th>
                  <th style={{padding: '8px 12px'}}>Factor</th>
                  <th style={{padding: '8px 12px'}}>Locality Map</th>
                </tr>
              </thead>
              <tbody>
                {clusterData.replication.map(f => (
                  <tr key={f.name}>
                    <td style={{padding: '12px', fontWeight: 500, maxWidth: '100px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}} title={f.name}>{f.name}</td>
                    <td style={{padding: '12px'}}>
                      <div style={{display: 'flex', alignItems: 'center', gap: '8px'}}>
                        <span style={{color: f.replicas.length >= 3 ? 'var(--success-color)' : 'var(--warning-color)', fontWeight: 600}}>
                          {f.replicas.length} / 3
                        </span>
                        {f.replicas.length >= 3 && (
                          <span style={{
                            fontSize: '0.65rem', 
                            padding: '2px 6px', 
                            background: 'rgba(63, 185, 80, 0.15)', 
                            color: 'var(--success-color)', 
                            border: '1px solid rgba(63, 185, 80, 0.3)',
                            borderRadius: '12px',
                            fontWeight: 700
                          }}>FULLY REPLICATED</span>
                        )}
                      </div>
                    </td>
                    <td style={{padding: '12px'}}>
                      <div style={{display: 'flex', gap: '4px', flexWrap: 'wrap'}}>
                        {f.replicas.map(r => (
                          <span key={r} style={{
                            background: 'rgba(248, 81, 73, 0.1)',
                            border: '1px solid rgba(248, 81, 73, 0.3)',
                            padding: '2px 6px',
                            borderRadius: '4px',
                            fontSize: '0.7rem'
                          }}>{r}</span>
                        ))}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

    </div>
  );
}
