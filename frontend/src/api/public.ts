import { api, unwrap } from './client';
import type { CardVerifyResult, ReceiveOrder } from '../types/api';

export function verifyCard(cardCode: string) {
  return unwrap<CardVerifyResult>(api.post('/public/cards/verify', { cardCode }));
}

export function createOrder(cardCode: string) {
  return unwrap<ReceiveOrder>(api.post('/public/orders', { cardCode }));
}

export function getOrder(orderNo: string, cardCode: string) {
  return unwrap<ReceiveOrder>(api.get(`/public/orders/${orderNo}`, { params: { cardCode } }));
}

export function checkOrder(orderNo: string, cardCode: string) {
  return unwrap<ReceiveOrder>(api.post(`/public/orders/${orderNo}/check`, { cardCode }));
}

export function cancelOrder(orderNo: string, cardCode: string) {
  return unwrap<ReceiveOrder>(api.post(`/public/orders/${orderNo}/cancel`, { cardCode }));
}

export function getHistory(cardCode: string) {
  return unwrap<ReceiveOrder[]>(api.get('/public/cards/history', { params: { cardCode } }));
}

export function recordVisit(path = '/') {
  return unwrap<{ recorded: boolean }>(api.post('/public/visits', { path }));
}
