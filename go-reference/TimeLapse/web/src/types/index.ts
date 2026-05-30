// Camera types
export interface Camera {
  uuid: string;
  name: string;
  type: 'onvif' | 'rtsp';
  connection_url: string;
  enabled: boolean;
  schedule?: Schedule;
  connection_status: string;
  active_profile?: Profile;
  created_at: string;
  updated_at: string;
}

export interface Schedule {
  interval: string;
  days_of_week: string[];
  time_window?: TimeWindow;
  start_date?: string;
  end_date?: string;
}

export interface TimeWindow {
  start: string; // HH:MM format
  end: string;   // HH:MM format
}

export interface CameraRequest {
  name: string;
  type: 'onvif' | 'rtsp';
  connection_url: string;
  username?: string;
  password?: string;
  enabled?: boolean;
  schedule?: {
    interval?: string;
    days_of_week?: string[];
    time_window?: { start?: string; end?: string };
    start_date?: string;
    end_date?: string;
  };
}

// Profile types
export interface Profile {
  token: string;
  name: string;
  resolution?: string;
  video_codec?: string;
  snapshot_uri?: string;
  stream_uri?: string;
  is_active: boolean;
}

// Discovery types
export interface DiscoveredDevice {
  ip: string;
  port: number;
  manufacturer?: string;
  model?: string;
  firmware?: string;
  onvif_url?: string;
}

export interface ProbeRequest {
  ip: string;
  port?: number;
  username?: string;
  password?: string;
}

export interface ProbeResponse {
  success: boolean;
  device?: DiscoveredDevice;
  error?: string;
}

export interface ScanResponse {
  status: string;
  devices: DiscoveredDevice[];
}

// Image types
export interface ImageInfo {
  filename: string;
  camera_uuid: string;
  timestamp: string;
  size: number;
  url: string;
  thumbnail_url?: string;
}

export interface ImageListResponse {
  images: ImageInfo[];
  total: number;
  limit: number;
  offset: number;
}

// Statistics types
export interface CameraStats {
  camera_uuid: string;
  camera_name: string;
  total_captures: number;
  successful_captures: number;
  failed_captures: number;
  last_capture_time?: string;
  last_error?: string;
  is_connected: boolean;
  is_capturing: boolean;
}

export interface StorageStats {
  total_images: number;
  total_size_bytes: number;
  oldest_image?: string;
  newest_image?: string;
}

export interface GlobalStats {
  cameras: {
    total: number;
    enabled: number;
    connected: number;
    capturing: number;
  };
  capture: {
    total_captures: number;
    successful_captures: number;
    failed_captures: number;
  };
  storage: StorageStats;
}

// Snapshot types
export interface SnapshotResponse {
  success: boolean;
  filename?: string;
  size?: number;
  url?: string;
  error?: string;
}

// Generic response types
export interface SuccessResponse {
  success: boolean;
  message?: string;
}

export interface ErrorResponse {
  error: string;
  details?: string;
}
