import { api, unwrap } from './client';
import type { CardBatch, CardCode, LoginResult, ReceiveOrder, ServiceConfig } from '../types/api';

export function login(username: string, password: string) {
  return unwrap<LoginResult>(api.post('/admin/auth/login', { username, password }));
}

export function listProviders() {
  return unwrap<any[]>(api.get('/admin/providers'));
}

export function listServiceConfigs() {
  return unwrap<ServiceConfig[]>(api.get('/admin/service-configs'));
}

export function createServiceConfig(payload: Partial<ServiceConfig>) {
  return unwrap<ServiceConfig>(api.post('/admin/service-configs', payload));
}

export function createCardBatch(payload: {
  name: string;
  serviceConfigId: number;
  quantity: number;
  usesPerCode: number;
  expiresAt?: string;
}) {
  return unwrap<CardBatch>(api.post('/admin/card-batches', payload));
}

export function listCardBatches() {
  return unwrap<CardBatch[]>(api.get('/admin/card-batches'));
}

export async function downloadCardBatch(id: number) {
  const response = await api.get(`/admin/card-batches/${id}/export.txt`, { responseType: 'blob' });
  const blob = new Blob([response.data], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `card-batch-${id}.txt`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function listCardCodes() {
  return unwrap<CardCode[]>(api.get('/admin/card-codes'));
}

export function updateCardStatus(id: number, status: string) {
  return unwrap<{ id: number; status: string }>(api.patch(`/admin/card-codes/${id}/status`, { status }));
}

export function listOrders() {
  return unwrap<ReceiveOrder[]>(api.get('/admin/orders'));
}

export function listAuditLogs() {
  return unwrap<any[]>(api.get('/admin/audit-logs'));
}
