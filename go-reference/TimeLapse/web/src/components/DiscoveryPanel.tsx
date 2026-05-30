import { useState } from 'react';
import { discoveryApi, cameraApi } from '../api/client';
import type { Camera, DiscoveredDevice, ProbeRequest } from '../types';

interface DiscoveryPanelProps {
  onCameraAdded: (camera: Camera) => void;
}

export default function DiscoveryPanel({ onCameraAdded }: DiscoveryPanelProps) {
  const [probeIp, setProbeIp] = useState('');
  const [probePort, setProbePort] = useState('80');
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [discoveredDevice, setDiscoveredDevice] = useState<DiscoveredDevice | null>(null);
  const [cameraName, setCameraName] = useState('');

  const handleProbe = async () => {
    if (!probeIp) {
      setError('Please enter an IP address');
      return;
    }

    setLoading(true);
    setError(null);
    setDiscoveredDevice(null);

    try {
      const request: ProbeRequest = {
        ip: probeIp,
        port: parseInt(probePort) || 80,
        username: username || undefined,
        password: password || undefined,
      };

      const response = await discoveryApi.probe(request);

      if (response.success && response.device) {
        setDiscoveredDevice(response.device);
        setCameraName(response.device.model || `Camera at ${probeIp}`);
      } else {
        setError(response.error || 'No ONVIF device found at this address');
      }
    } catch (err) {
      setError('Failed to probe device. Check the IP address and credentials.');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleAddCamera = async () => {
    if (!discoveredDevice) return;

    setLoading(true);
    setError(null);

    try {
      const camera = await cameraApi.create({
        name: cameraName || `Camera at ${discoveredDevice.ip}`,
        type: 'onvif',
        connection_url: `http://${discoveredDevice.ip}:${discoveredDevice.port}`,
        username: username,
        password: password,
        enabled: true,
        schedule: {
          interval: '10s',
          days_of_week: ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'],
        },
      });

      onCameraAdded(camera);
      setDiscoveredDevice(null);
      setProbeIp('');
      setCameraName('');
    } catch (err) {
      setError('Failed to add camera');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="card">
      <div className="card-header">Discover Camera</div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="grid grid-2" style={{ marginBottom: '20px' }}>
        <div className="form-group">
          <label className="form-label">IP Address</label>
          <input
            type="text"
            className="input"
            placeholder="192.168.1.100"
            value={probeIp}
            onChange={(e) => setProbeIp(e.target.value)}
          />
        </div>
        <div className="form-group">
          <label className="form-label">Port</label>
          <input
            type="text"
            className="input"
            placeholder="80"
            value={probePort}
            onChange={(e) => setProbePort(e.target.value)}
          />
        </div>
      </div>

      <div className="grid grid-2" style={{ marginBottom: '20px' }}>
        <div className="form-group">
          <label className="form-label">Username</label>
          <input
            type="text"
            className="input"
            placeholder="admin"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </div>
        <div className="form-group">
          <label className="form-label">Password</label>
          <input
            type="password"
            className="input"
            placeholder="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
      </div>

      <button
        className="btn btn-primary"
        onClick={handleProbe}
        disabled={loading || !probeIp}
        style={{ marginBottom: '20px' }}
      >
        {loading ? <span className="spinner" /> : 'Probe Device'}
      </button>

      {discoveredDevice && (
        <div style={{ background: '#f0fdf4', padding: '20px', borderRadius: '8px', marginTop: '20px' }}>
          <h3 style={{ fontWeight: '600', marginBottom: '12px', color: '#065f46' }}>
            Device Found!
          </h3>
          <table className="table" style={{ marginBottom: '16px' }}>
            <tbody>
              <tr>
                <td style={{ fontWeight: '500' }}>Manufacturer</td>
                <td>{discoveredDevice.manufacturer || 'Unknown'}</td>
              </tr>
              <tr>
                <td style={{ fontWeight: '500' }}>Model</td>
                <td>{discoveredDevice.model || 'Unknown'}</td>
              </tr>
              <tr>
                <td style={{ fontWeight: '500' }}>Firmware</td>
                <td>{discoveredDevice.firmware || 'Unknown'}</td>
              </tr>
              <tr>
                <td style={{ fontWeight: '500' }}>IP:Port</td>
                <td>{discoveredDevice.ip}:{discoveredDevice.port}</td>
              </tr>
            </tbody>
          </table>

          <div className="form-group">
            <label className="form-label">Camera Name</label>
            <input
              type="text"
              className="input"
              value={cameraName}
              onChange={(e) => setCameraName(e.target.value)}
            />
          </div>

          <button
            className="btn btn-success"
            onClick={handleAddCamera}
            disabled={loading}
          >
            Add Camera
          </button>
        </div>
      )}
    </div>
  );
}
