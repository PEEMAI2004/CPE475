import axios from 'axios';

// Under development, use the local go backend. Once deployed, use relative paths.
const API_URL = import.meta.env.DEV ? 'http://localhost:8081/api' : '/api';

export const api = axios.create({
  baseURL: API_URL,
});

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
}

export const getProfiles = async () => (await api.get<Profile[]>('/profiles')).data;
export const createProfile = async (p: Partial<Profile>) => (await api.post<Profile>('/profiles', p)).data;
export const updateProfile = async (id: number, p: Partial<Profile>) => await api.put(`/profiles/${id}`, p);
export const deleteProfile = async (id: number) => await api.delete(`/profiles/${id}`);

export const getDevices = async () => (await api.get<Device[]>('/devices')).data;
export const updateDeviceProfile = async (id: string, profile_id: number) => await api.put(`/devices/${id}`, { profile_id });
