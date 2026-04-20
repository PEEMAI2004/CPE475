import axios from 'axios';

// Under development, use the local go backend. Once deployed, use relative paths.
const API_URL = import.meta.env.DEV ? 'http://localhost:8081/api' : '/api';

export const api = axios.create({
  baseURL: API_URL,
});

// Add interceptor to inject JWT
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Add interceptor to handle 401 Unauthorized
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/'; // Redirect to login
    }
    return Promise.reject(error);
  }
);

export interface Profile {
  id: number;
  name: string;
  soil_inner_low: number;
  soil_inner_high: number;
  soil_outer_low: number;
  soil_outer_high: number;
  temp_inner_low: number;
  temp_inner_high: number;
  temp_outer_low: number;
  temp_outer_high: number;
  hum_inner_low: number;
  hum_inner_high: number;
  hum_outer_low: number;
  hum_outer_high: number;
  light_inner_low: number;
  light_inner_high: number;
  light_outer_low: number;
  light_outer_high: number;
}

export interface Device {
  device_id: string;
  profile_id: number;
  online: boolean;
  health: string;
}

export interface User {
  id: number;
  email: string;
  name: string;
  role: string;
  created_at: string;
}

export const login = async (idToken: string) => {
  const resp = await api.post<{ token: string; role: string; name: string }>('/auth/login', { idToken });
  localStorage.setItem('token', resp.data.token);
  localStorage.setItem('user', JSON.stringify({ name: resp.data.name, role: resp.data.role }));
  return resp.data;
};

export const logout = () => {
  localStorage.removeItem('token');
  localStorage.removeItem('user');
};

export const getProfiles = async () => (await api.get<Profile[]>('/profiles')).data;
export const createProfile = async (p: Partial<Profile>) => (await api.post<Profile>('/profiles', p)).data;
export const updateProfile = async (id: number, p: Partial<Profile>) => await api.put(`/profiles/${id}`, p);
export const deleteProfile = async (id: number) => await api.delete(`/profiles/${id}`);

export const getDevices = async () => (await api.get<Device[]>('/devices')).data;
export const updateDeviceProfile = async (id: string, profile_id: number) => await api.put(`/devices/${id}`, { profile_id });

export interface ServiceHealth {
  name: string;
  type: string;
  status: string;
  address: string;
}

export const getInfrastructureHealth = async () => (await api.get<ServiceHealth[]>('/infrastructure')).data;

// User Management
export const getUsers = async () => (await api.get<User[]>('/users')).data;
export const inviteUser = async (email: string, role: string) => (await api.post<User>('/users', { email, role })).data;
export const deleteUser = async (id: number) => await api.delete(`/users/${id}`);
