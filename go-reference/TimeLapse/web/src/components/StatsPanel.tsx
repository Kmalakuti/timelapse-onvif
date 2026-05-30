import type { GlobalStats } from '../types';

interface StatsPanelProps {
  stats: GlobalStats;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export default function StatsPanel({ stats }: StatsPanelProps) {
  const successRate =
    stats.capture.total_captures > 0
      ? ((stats.capture.successful_captures / stats.capture.total_captures) * 100).toFixed(1)
      : '0';

  return (
    <div className="grid grid-4" style={{ marginBottom: '20px' }}>
      <div className="card">
        <div className="stat-label">Total Cameras</div>
        <div className="stat-value">{stats.cameras.total}</div>
        <div style={{ fontSize: '12px', color: '#6b7280', marginTop: '4px' }}>
          {stats.cameras.connected} connected, {stats.cameras.capturing} capturing
        </div>
      </div>

      <div className="card">
        <div className="stat-label">Total Captures</div>
        <div className="stat-value">{stats.capture.total_captures.toLocaleString()}</div>
        <div style={{ fontSize: '12px', color: '#6b7280', marginTop: '4px' }}>
          {successRate}% success rate
        </div>
      </div>

      <div className="card">
        <div className="stat-label">Total Images</div>
        <div className="stat-value">{stats.storage.total_images.toLocaleString()}</div>
        <div style={{ fontSize: '12px', color: '#6b7280', marginTop: '4px' }}>
          {formatBytes(stats.storage.total_size_bytes)}
        </div>
      </div>

      <div className="card">
        <div className="stat-label">Failed Captures</div>
        <div className="stat-value" style={{ color: stats.capture.failed_captures > 0 ? '#ef4444' : '#10b981' }}>
          {stats.capture.failed_captures}
        </div>
        <div style={{ fontSize: '12px', color: '#6b7280', marginTop: '4px' }}>
          {stats.capture.successful_captures} successful
        </div>
      </div>
    </div>
  );
}
