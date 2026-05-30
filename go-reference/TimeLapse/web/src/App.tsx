import { useState, useEffect } from 'react';
import { statsApi, cameraApi } from './api/client';
import type { GlobalStats, Camera } from './types';
import DiscoveryPanel from './components/DiscoveryPanel';
import CameraList from './components/CameraList';
import StatsPanel from './components/StatsPanel';

function App() {
  const [stats, setStats] = useState<GlobalStats | null>(null);
  const [cameras, setCameras] = useState<Camera[]>([]);
  const [activeTab, setActiveTab] = useState<'dashboard' | 'discovery' | 'cameras'>('dashboard');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async () => {
    try {
      const [statsData, camerasData] = await Promise.all([
        statsApi.getGlobal(),
        cameraApi.list(),
      ]);
      setStats(statsData);
      setCameras(camerasData.cameras);
      setError(null);
    } catch (err) {
      setError('Failed to fetch data from server');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCameraAdded = (camera: Camera) => {
    setCameras((prev) => [...prev, camera]);
    fetchData();
  };

  const handleCameraDeleted = (uuid: string) => {
    setCameras((prev) => prev.filter((c) => c.uuid !== uuid));
    fetchData();
  };

  return (
    <div className="container">
      <header style={{ marginBottom: '24px' }}>
        <h1 style={{ fontSize: '1.5rem', fontWeight: 'bold', marginBottom: '8px' }}>
          TimeLapse Camera System
        </h1>
        <p style={{ color: '#6b7280' }}>Phase 3 - Verification Frontend</p>
      </header>

      <nav style={{ marginBottom: '24px' }}>
        <div className="flex gap-2">
          <button
            className={`btn ${activeTab === 'dashboard' ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => setActiveTab('dashboard')}
          >
            Dashboard
          </button>
          <button
            className={`btn ${activeTab === 'discovery' ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => setActiveTab('discovery')}
          >
            Discovery
          </button>
          <button
            className={`btn ${activeTab === 'cameras' ? 'btn-primary' : 'btn-secondary'}`}
            onClick={() => setActiveTab('cameras')}
          >
            Cameras ({cameras.length})
          </button>
          <button className="btn btn-secondary" onClick={fetchData} disabled={loading}>
            {loading ? <span className="spinner" /> : 'Refresh'}
          </button>
        </div>
      </nav>

      {error && <div className="alert alert-error">{error}</div>}

      {activeTab === 'dashboard' && (
        <div>
          {stats && <StatsPanel stats={stats} />}
          {cameras.length > 0 && (
            <div className="card">
              <div className="card-header">Recent Cameras</div>
              <CameraList
                cameras={cameras.slice(0, 3)}
                onCameraDeleted={handleCameraDeleted}
                onRefresh={fetchData}
              />
            </div>
          )}
        </div>
      )}

      {activeTab === 'discovery' && (
        <DiscoveryPanel onCameraAdded={handleCameraAdded} />
      )}

      {activeTab === 'cameras' && (
        <div className="card">
          <div className="card-header flex justify-between items-center">
            <span>All Cameras</span>
          </div>
          {cameras.length === 0 ? (
            <p style={{ color: '#6b7280', textAlign: 'center', padding: '40px' }}>
              No cameras configured. Use the Discovery tab to find and add cameras.
            </p>
          ) : (
            <CameraList
              cameras={cameras}
              onCameraDeleted={handleCameraDeleted}
              onRefresh={fetchData}
            />
          )}
        </div>
      )}
    </div>
  );
}

export default App;
