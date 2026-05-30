import { useState, useEffect } from 'react';
import { cameraApi, profileApi, captureApi, statsApi, imageApi } from '../api/client';
import type { Camera, Profile, CameraStats, ImageInfo, Schedule } from '../types';
import ImageModal from './ImageModal';

// Predefined interval options
const INTERVALS = [
  { value: '5s', label: '5 seconds' },
  { value: '10s', label: '10 seconds' },
  { value: '30s', label: '30 seconds' },
  { value: '1m', label: '1 minute' },
  { value: '5m', label: '5 minutes' },
  { value: '10m', label: '10 minutes' },
  { value: '30m', label: '30 minutes' },
  { value: '1h', label: '1 hour' },
];

// Days of week for schedule
const DAYS_OF_WEEK = [
  { value: 'monday', label: 'Mon' },
  { value: 'tuesday', label: 'Tue' },
  { value: 'wednesday', label: 'Wed' },
  { value: 'thursday', label: 'Thu' },
  { value: 'friday', label: 'Fri' },
  { value: 'saturday', label: 'Sat' },
  { value: 'sunday', label: 'Sun' },
];

interface CameraCardProps {
  camera: Camera;
  isExpanded: boolean;
  onToggleExpand: () => void;
  onDeleted: () => void;
  onRefresh: () => void;
}

export default function CameraCard({
  camera,
  isExpanded,
  onToggleExpand,
  onDeleted,
  onRefresh,
}: CameraCardProps) {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [stats, setStats] = useState<CameraStats | null>(null);
  const [images, setImages] = useState<ImageInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  // Image modal state
  const [selectedImage, setSelectedImage] = useState<ImageInfo | null>(null);
  const [selectedImageIndex, setSelectedImageIndex] = useState(0);

  // Capture settings state
  const [isCapturing, setIsCapturing] = useState(false);
  const [selectedInterval, setSelectedInterval] = useState(camera.schedule?.interval || '30s');
  const [activeDays, setActiveDays] = useState<string[]>(
    camera.schedule?.days_of_week || DAYS_OF_WEEK.map((d) => d.value)
  );
  const [timeWindowStart, setTimeWindowStart] = useState(camera.schedule?.time_window?.start || '');
  const [timeWindowEnd, setTimeWindowEnd] = useState(camera.schedule?.time_window?.end || '');
  const [startDate, setStartDate] = useState(camera.schedule?.start_date || '');
  const [endDate, setEndDate] = useState(camera.schedule?.end_date || '');

  useEffect(() => {
    if (isExpanded) {
      fetchDetails();
    }
  }, [isExpanded, camera.uuid]);

  // Sync capture state from stats
  useEffect(() => {
    if (stats) {
      setIsCapturing(stats.is_capturing);
    }
  }, [stats]);

  // Sync schedule state from camera prop
  useEffect(() => {
    setSelectedInterval(camera.schedule?.interval || '30s');
    setActiveDays(camera.schedule?.days_of_week || DAYS_OF_WEEK.map((d) => d.value));
    setTimeWindowStart(camera.schedule?.time_window?.start || '');
    setTimeWindowEnd(camera.schedule?.time_window?.end || '');
    setStartDate(camera.schedule?.start_date || '');
    setEndDate(camera.schedule?.end_date || '');
  }, [camera]);

  const fetchDetails = async () => {
    setLoading(true);
    try {
      const [profilesData, statsData, imagesData] = await Promise.all([
        profileApi.list(camera.uuid).catch(() => ({ profiles: [] })),
        statsApi.getCamera(camera.uuid).catch(() => null),
        imageApi.list(camera.uuid, 4, 0).catch(() => ({ images: [] })), // Changed from 6 to 4
      ]);
      setProfiles(profilesData.profiles);
      setStats(statsData);
      setImages(imagesData.images);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleSelectProfile = async (token: string) => {
    setLoading(true);
    setError(null);
    try {
      await profileApi.select(camera.uuid, token);
      setMessage('Profile selected');
      setTimeout(() => setMessage(null), 2000);
      fetchDetails();
    } catch (err) {
      setError('Failed to select profile');
    } finally {
      setLoading(false);
    }
  };

  const handleSnapshot = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await captureApi.snapshot(camera.uuid);
      if (response.success) {
        setMessage(`Snapshot saved: ${response.filename}`);
        setTimeout(() => setMessage(null), 3000);
        fetchDetails();
        onRefresh();
      } else {
        setError(response.error || 'Failed to capture snapshot');
      }
    } catch (err) {
      setError('Failed to capture snapshot');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!confirm('Are you sure you want to delete this camera?')) return;
    setLoading(true);
    try {
      await cameraApi.delete(camera.uuid);
      onDeleted();
    } catch (err) {
      setError('Failed to delete camera');
      setLoading(false);
    }
  };

  // Toggle capture start/stop
  const handleToggleCapture = async () => {
    setLoading(true);
    setError(null);
    try {
      if (isCapturing) {
        await captureApi.stop(camera.uuid);
        setIsCapturing(false);
        setMessage('Capture stopped (resources released)');
      } else {
        await captureApi.start(camera.uuid);
        setIsCapturing(true);
        setMessage('Capture started');
      }
      setTimeout(() => setMessage(null), 2000);
      fetchDetails();
    } catch (err) {
      setError(isCapturing ? 'Failed to stop capture' : 'Failed to start capture');
    } finally {
      setLoading(false);
    }
  };

  // Update camera schedule
  const updateSchedule = async (updates: Partial<Schedule>) => {
    setLoading(true);
    setError(null);
    try {
      const newSchedule = {
        interval: updates.interval ?? selectedInterval,
        days_of_week: updates.days_of_week ?? activeDays,
        time_window:
          updates.time_window !== undefined
            ? updates.time_window
            : timeWindowStart && timeWindowEnd
            ? { start: timeWindowStart, end: timeWindowEnd }
            : undefined,
        start_date: updates.start_date !== undefined ? updates.start_date : startDate || undefined,
        end_date: updates.end_date !== undefined ? updates.end_date : endDate || undefined,
      };

      await cameraApi.update(camera.uuid, {
        name: camera.name,
        type: camera.type,
        connection_url: camera.connection_url,
        schedule: newSchedule,
      });

      setMessage('Schedule updated');
      setTimeout(() => setMessage(null), 2000);
      onRefresh();
    } catch (err) {
      setError('Failed to update schedule');
    } finally {
      setLoading(false);
    }
  };

  // Handle interval change
  const handleIntervalChange = (newInterval: string) => {
    setSelectedInterval(newInterval);
    updateSchedule({ interval: newInterval });
  };

  // Toggle day active status
  const toggleDay = (day: string) => {
    const newDays = activeDays.includes(day)
      ? activeDays.filter((d) => d !== day)
      : [...activeDays, day];
    setActiveDays(newDays);
    updateSchedule({ days_of_week: newDays });
  };

  // Update time window
  const handleTimeWindowChange = (start: string, end: string) => {
    setTimeWindowStart(start);
    setTimeWindowEnd(end);
    if (start && end) {
      updateSchedule({ time_window: { start, end } });
    }
  };

  // Clear time window
  const clearTimeWindow = () => {
    setTimeWindowStart('');
    setTimeWindowEnd('');
    updateSchedule({ time_window: undefined });
  };

  // Update date range
  const handleDateRangeChange = (start: string, end: string) => {
    setStartDate(start);
    setEndDate(end);
    updateSchedule({ start_date: start || undefined, end_date: end || undefined });
  };

  // Clear date range
  const clearDateRange = () => {
    setStartDate('');
    setEndDate('');
    updateSchedule({ start_date: undefined, end_date: undefined });
  };

  // Image modal navigation
  const handleImageClick = (image: ImageInfo, index: number) => {
    setSelectedImage(image);
    setSelectedImageIndex(index);
  };

  const handlePreviousImage = () => {
    if (selectedImageIndex > 0) {
      setSelectedImageIndex(selectedImageIndex - 1);
      setSelectedImage(images[selectedImageIndex - 1]);
    }
  };

  const handleNextImage = () => {
    if (selectedImageIndex < images.length - 1) {
      setSelectedImageIndex(selectedImageIndex + 1);
      setSelectedImage(images[selectedImageIndex + 1]);
    }
  };

  const statusClass =
    camera.connection_status === 'connected' ? 'status-connected' : 'status-disconnected';

  return (
    <div style={{ borderBottom: '1px solid #eee', paddingBottom: '16px', marginBottom: '16px' }}>
      <div className="flex justify-between items-center" style={{ marginBottom: '8px' }}>
        <div>
          <h3 style={{ fontWeight: '600', marginBottom: '4px' }}>{camera.name}</h3>
          <p style={{ fontSize: '12px', color: '#6b7280' }}>
            {camera.type.toUpperCase()} | {camera.connection_url}
          </p>
        </div>
        <div className="flex gap-2 items-center">
          <span className={`status-badge ${statusClass}`}>{camera.connection_status}</span>
          <button className="btn btn-secondary" onClick={onToggleExpand}>
            {isExpanded ? 'Collapse' : 'Expand'}
          </button>
        </div>
      </div>

      {isExpanded && (
        <div style={{ marginTop: '16px', paddingLeft: '16px', borderLeft: '3px solid #e5e7eb' }}>
          {error && <div className="alert alert-error">{error}</div>}
          {message && <div className="alert alert-success">{message}</div>}

          {/* Stats Section */}
          {stats && (
            <div style={{ marginBottom: '20px' }}>
              <h4 style={{ fontWeight: '500', marginBottom: '8px' }}>Statistics</h4>
              <div className="grid grid-4">
                <div>
                  <div style={{ fontSize: '12px', color: '#6b7280' }}>Total Captures</div>
                  <div style={{ fontWeight: '600' }}>{stats.total_captures}</div>
                </div>
                <div>
                  <div style={{ fontSize: '12px', color: '#6b7280' }}>Successful</div>
                  <div style={{ fontWeight: '600', color: '#10b981' }}>{stats.successful_captures}</div>
                </div>
                <div>
                  <div style={{ fontSize: '12px', color: '#6b7280' }}>Failed</div>
                  <div style={{ fontWeight: '600', color: '#ef4444' }}>{stats.failed_captures}</div>
                </div>
                <div>
                  <div style={{ fontSize: '12px', color: '#6b7280' }}>Status</div>
                  <div style={{ fontWeight: '600' }}>
                    {stats.is_capturing ? (
                      <span style={{ color: '#10b981' }}>Capturing</span>
                    ) : stats.is_connected ? (
                      'Connected'
                    ) : (
                      <span style={{ color: '#ef4444' }}>Disconnected</span>
                    )}
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Capture Settings Section */}
          <div style={{ marginBottom: '20px' }}>
            <h4 style={{ fontWeight: '500', marginBottom: '12px' }}>Capture Settings</h4>

            {/* Row 1: Capture toggle + Interval */}
            <div className="flex gap-4 items-center" style={{ marginBottom: '12px', flexWrap: 'wrap' }}>
              <button
                className={isCapturing ? 'btn btn-warning' : 'btn btn-success'}
                onClick={handleToggleCapture}
                disabled={loading}
                style={{ minWidth: '130px' }}
              >
                {loading ? <span className="spinner" /> : isCapturing ? 'Stop Capture' : 'Start Capture'}
              </button>

              <div className="flex items-center gap-2">
                <label style={{ fontSize: '12px', color: '#6b7280' }}>Interval:</label>
                <select
                  value={selectedInterval}
                  onChange={(e) => handleIntervalChange(e.target.value)}
                  disabled={loading}
                  style={{
                    padding: '6px 12px',
                    borderRadius: '4px',
                    border: '1px solid #d1d5db',
                    fontSize: '14px',
                  }}
                >
                  {INTERVALS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            {/* Row 2: Days of week checkboxes */}
            <div style={{ marginBottom: '12px' }}>
              <label style={{ fontSize: '12px', color: '#6b7280', display: 'block', marginBottom: '6px' }}>
                Active Days:
              </label>
              <div className="flex gap-3" style={{ flexWrap: 'wrap' }}>
                {DAYS_OF_WEEK.map((day) => (
                  <label
                    key={day.value}
                    style={{ fontSize: '13px', display: 'flex', alignItems: 'center', gap: '4px', cursor: 'pointer' }}
                  >
                    <input
                      type="checkbox"
                      checked={activeDays.includes(day.value)}
                      onChange={() => toggleDay(day.value)}
                      disabled={loading}
                    />
                    {day.label}
                  </label>
                ))}
              </div>
            </div>

            {/* Row 3: Time window */}
            <div style={{ marginBottom: '12px' }}>
              <label style={{ fontSize: '12px', color: '#6b7280', display: 'block', marginBottom: '6px' }}>
                Time Window (optional):
              </label>
              <div className="flex gap-2 items-center" style={{ flexWrap: 'wrap' }}>
                <input
                  type="time"
                  value={timeWindowStart}
                  onChange={(e) => handleTimeWindowChange(e.target.value, timeWindowEnd)}
                  disabled={loading}
                  style={{ padding: '4px 8px', borderRadius: '4px', border: '1px solid #d1d5db' }}
                />
                <span style={{ color: '#6b7280' }}>to</span>
                <input
                  type="time"
                  value={timeWindowEnd}
                  onChange={(e) => handleTimeWindowChange(timeWindowStart, e.target.value)}
                  disabled={loading}
                  style={{ padding: '4px 8px', borderRadius: '4px', border: '1px solid #d1d5db' }}
                />
                {(timeWindowStart || timeWindowEnd) && (
                  <button
                    className="btn btn-secondary"
                    onClick={clearTimeWindow}
                    disabled={loading}
                    style={{ padding: '4px 8px', fontSize: '12px' }}
                  >
                    Clear
                  </button>
                )}
              </div>
            </div>

            {/* Row 4: Date range */}
            <div>
              <label style={{ fontSize: '12px', color: '#6b7280', display: 'block', marginBottom: '6px' }}>
                Date Range (optional):
              </label>
              <div className="flex gap-2 items-center" style={{ flexWrap: 'wrap' }}>
                <input
                  type="date"
                  value={startDate}
                  onChange={(e) => handleDateRangeChange(e.target.value, endDate)}
                  disabled={loading}
                  style={{ padding: '4px 8px', borderRadius: '4px', border: '1px solid #d1d5db' }}
                />
                <span style={{ color: '#6b7280' }}>to</span>
                <input
                  type="date"
                  value={endDate}
                  onChange={(e) => handleDateRangeChange(startDate, e.target.value)}
                  disabled={loading}
                  style={{ padding: '4px 8px', borderRadius: '4px', border: '1px solid #d1d5db' }}
                />
                {(startDate || endDate) && (
                  <button
                    className="btn btn-secondary"
                    onClick={clearDateRange}
                    disabled={loading}
                    style={{ padding: '4px 8px', fontSize: '12px' }}
                  >
                    Clear
                  </button>
                )}
              </div>
              <p style={{ fontSize: '11px', color: '#9ca3af', marginTop: '4px' }}>
                Leave empty for indefinite capture
              </p>
            </div>
          </div>

          {/* Profiles Section */}
          {profiles.length > 0 && (
            <div style={{ marginBottom: '20px' }}>
              <h4 style={{ fontWeight: '500', marginBottom: '8px' }}>ONVIF Profiles</h4>
              <table className="table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Resolution</th>
                    <th>Codec</th>
                    <th>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {profiles.map((profile) => (
                    <tr key={profile.token}>
                      <td>
                        {profile.name}
                        {profile.is_active && (
                          <span
                            style={{
                              marginLeft: '8px',
                              fontSize: '10px',
                              background: '#3b82f6',
                              color: 'white',
                              padding: '2px 6px',
                              borderRadius: '4px',
                            }}
                          >
                            ACTIVE
                          </span>
                        )}
                      </td>
                      <td>{profile.resolution || 'N/A'}</td>
                      <td>{profile.video_codec || 'N/A'}</td>
                      <td>
                        {!profile.is_active && (
                          <button
                            className="btn btn-secondary"
                            onClick={() => handleSelectProfile(profile.token)}
                            disabled={loading}
                            style={{ padding: '4px 8px', fontSize: '12px' }}
                          >
                            Select
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-2" style={{ marginBottom: '20px' }}>
            <button className="btn btn-success" onClick={handleSnapshot} disabled={loading}>
              {loading ? <span className="spinner" /> : 'Take Snapshot'}
            </button>
            <button className="btn btn-danger" onClick={handleDelete} disabled={loading}>
              Delete Camera
            </button>
          </div>

          {/* Recent Images */}
          {images.length > 0 && (
            <div>
              <h4 style={{ fontWeight: '500', marginBottom: '8px' }}>Recent Images</h4>
              <div className="image-grid">
                {images.map((image, index) => (
                  <div
                    key={image.filename}
                    className="image-card"
                    onClick={() => handleImageClick(image, index)}
                    style={{ cursor: 'pointer' }}
                  >
                    <img src={imageApi.getUrl(image.filename)} alt={image.filename} loading="lazy" />
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Image Modal */}
      {selectedImage && (
        <ImageModal
          image={selectedImage}
          onClose={() => setSelectedImage(null)}
          onPrevious={handlePreviousImage}
          onNext={handleNextImage}
          hasPrevious={selectedImageIndex > 0}
          hasNext={selectedImageIndex < images.length - 1}
        />
      )}
    </div>
  );
}
