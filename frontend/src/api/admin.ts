import { api, unwrap } from './client';
import type {
  AuditLog,
  CardBatch,
  CardCode,
  DashboardStats,
  LoginResult,
  ProviderBalance,
  ProviderCountry,
  ProviderPrice,
  ProviderQuote,
  ProviderService,
  ProviderStock,
  ReceiveOrder,
  SMSProvider,
  ServiceConfig,
} from '../types/api';

export function login(username: string, password: string) {
  return unwrap<LoginResult>(api.post('/admin/auth/login', { username, password }));
}

export function changePassword(oldPassword: string, newPassword: string) {
  return unwrap<{ changed: boolean }>(api.post('/admin/auth/password', { oldPassword, newPassword }));
}

export function listProviders() {
  return unwrap<SMSProvider[]>(api.get('/admin/providers'));
}

export function updateProvider(provider: string, payload: Partial<SMSProvider> & { apiKey?: string }) {
  return unwrap<SMSProvider>(api.patch(`/admin/providers/${provider}`, payload));
}

export function listProviderCountries(provider: string) {
  return unwrap<ProviderCountry[]>(api.get(`/admin/providers/${provider}/countries`));
}

export function listProviderServices(provider: string, countryId?: string) {
  return unwrap<ProviderService[]>(api.get(`/admin/providers/${provider}/services`, { params: { countryId } }));
}

export function getProviderPrice(provider: string, params: { countryId?: string; serviceId?: string; poolId?: string }) {
  return unwrap<ProviderPrice>(api.get(`/admin/providers/${provider}/price`, { params }));
}

export function getProviderStock(provider: string, params: { countryId?: string; serviceId?: string; poolId?: string }) {
  return unwrap<ProviderStock>(api.get(`/admin/providers/${provider}/stock`, { params }));
}

export function getProviderQuote(provider: string, params: { countryId?: string; serviceId?: string; poolId?: string }) {
  return unwrap<ProviderQuote>(api.get(`/admin/providers/${provider}/quote`, { params }));
}

export function getProviderBalance(provider: string) {
  return unwrap<ProviderBalance>(api.get(`/admin/providers/${provider}/balance`));
}

export function listServiceConfigs() {
  return unwrap<ServiceConfig[]>(api.get('/admin/service-configs'));
}

export function createServiceConfig(payload: Partial<ServiceConfig>) {
  return unwrap<ServiceConfig>(api.post('/admin/service-configs', payload));
}

export function updateServiceConfig(id: number, payload: Partial<ServiceConfig>) {
  return unwrap<ServiceConfig>(api.patch(`/admin/service-configs/${id}`, payload));
}

export function deleteServiceConfig(id: number) {
  return unwrap<{ deleted: boolean }>(api.delete(`/admin/service-configs/${id}`));
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

export function deleteCardBatch(id: number) {
  return unwrap<{ deleted: boolean }>(api.delete(`/admin/card-batches/${id}`));
}

export function listCardCodes() {
  return unwrap<CardCode[]>(api.get('/admin/card-codes'));
}

export function updateCardStatus(id: number, status: string) {
  return unwrap<{ id: number; status: string }>(api.patch(`/admin/card-codes/${id}/status`, { status }));
}

export function revealCardCode(id: number) {
  return unwrap<{ code: string }>(api.get(`/admin/card-codes/${id}/reveal`));
}

export function deleteCardCode(id: number) {
  return unwrap<{ deleted: boolean }>(api.delete(`/admin/card-codes/${id}`));
}

export function listOrders() {
  return unwrap<ReceiveOrder[]>(api.get('/admin/orders'));
}

export function listAuditLogs() {
  return unwrap<AuditLog[]>(api.get('/admin/audit-logs'));
}

export function getDashboardStats() {
  return unwrap<DashboardStats>(api.get('/admin/dashboard'));
}
