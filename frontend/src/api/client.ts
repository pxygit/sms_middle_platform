import axios from 'axios';
import type { ApiResponse } from '../types/api';

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('adminToken');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export async function unwrap<T>(promise: Promise<{ data: ApiResponse<T> }>): Promise<T> {
  try {
    const response = await promise;
    return response.data.data;
  } catch (error: any) {
    throw new Error(error?.response?.data?.message || error?.message || 'Request failed');
  }
}
