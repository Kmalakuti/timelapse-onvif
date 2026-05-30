import axios from 'axios';
import type {
  Camera,
  CameraRequest,
  CameraStats,
  GlobalStats,
  ImageListResponse,
  ProbeRequest,
  ProbeResponse,
  Profile,
  ScanResponse,
  SnapshotResponse,
  StorageStats,
  SuccessResponse,
} from '../types';

const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Discovery API
export const discoveryApi = {
  scan: async (timeoutSeconds = 5): Promise<{ scan_id: string; status: string }> => {
    const response = await api.post('/discovery/scan', { timeout_seconds: timeoutSeconds });
    return response.data;
  },

  getResults: async (): Promise<ScanResponse> => {
    const response = await api.get('/discovery/results');
    return response.data;
  },

  probe: async (request: ProbeRequest): Promise<ProbeResponse> => {
    const response = await api.post('/discovery/probe', request);
    return response.data;
  },
};

// Camera API
export const cameraApi = {
  list: async (): Promise<{ cameras: Camera[]; total: number }> => {
    const response = await api.get('/cameras');
    return response.data;
  },

  get: async (uuid: string): Promise<Camera> => {
    const response = await api.get(`/cameras/${uuid}`);
    return response.data;
  },

  create: async (camera: CameraRequest): Promise<Camera> => {
    const response = await api.post('/cameras', camera);
    return response.data;
  },

  update: async (uuid: string, camera: CameraRequest): Promise<Camera> => {
    const response = await api.put(`/cameras/${uuid}`, camera);
    return response.data;
  },

  delete: async (uuid: string): Promise<SuccessResponse> => {
    const response = await api.delete(`/cameras/${uuid}`);
    return response.data;
  },
};

// Profile API
export const profileApi = {
  list: async (cameraUuid: string): Promise<{ profiles: Profile[] }> => {
    const response = await api.get(`/cameras/${cameraUuid}/profiles`);
    return response.data;
  },

  select: async (cameraUuid: string, profileToken: string): Promise<SuccessResponse> => {
    const response = await api.put(`/cameras/${cameraUuid}/profiles/${profileToken}`);
    return response.data;
  },
};

// Capture API
export const captureApi = {
  start: async (cameraUuid: string): Promise<SuccessResponse> => {
    const response = await api.post(`/cameras/${cameraUuid}/start`);
    return response.data;
  },

  stop: async (cameraUuid: string): Promise<SuccessResponse> => {
    const response = await api.post(`/cameras/${cameraUuid}/stop`);
    return response.data;
  },

  snapshot: async (cameraUuid: string): Promise<SnapshotResponse> => {
    const response = await api.post(`/cameras/${cameraUuid}/snapshot`);
    return response.data;
  },
};

// Image API
export const imageApi = {
  list: async (
    cameraUuid: string,
    limit = 50,
    offset = 0
  ): Promise<ImageListResponse> => {
    const response = await api.get(`/cameras/${cameraUuid}/images`, {
      params: { limit, offset },
    });
    return response.data;
  },

  getUrl: (filename: string): string => {
    return `/api/v1/images/${filename}`;
  },
};

// Stats API
export const statsApi = {
  getGlobal: async (): Promise<GlobalStats> => {
    const response = await api.get('/stats');
    return response.data;
  },

  getCamera: async (cameraUuid: string): Promise<CameraStats> => {
    const response = await api.get(`/cameras/${cameraUuid}/stats`);
    return response.data;
  },

  getStorage: async (): Promise<StorageStats> => {
    const response = await api.get('/stats/storage');
    return response.data;
  },
};

export default api;
