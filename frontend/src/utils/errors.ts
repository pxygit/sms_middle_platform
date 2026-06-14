export function localizedError(message: string, t: (key: string) => string) {
  const raw = message || '';
  const code = raw.includes(':') ? raw.split(':')[0].trim() : '';
  const lower = raw.toLowerCase();
  const candidates = [
    code && `error_${code}`,
    lower.includes('two minutes') && 'error_CANCEL_WAIT',
    lower.includes('unfinished order') && 'error_ORDER_UNFINISHED',
    lower.includes('not found') && 'error_CARD_NOT_FOUND',
    lower.includes('not enabled') && 'error_CARD_DISABLED',
    lower.includes('expired') && 'error_CARD_EXPIRED',
    lower.includes('no remaining uses') && 'error_CARD_NO_USES',
    lower.includes('service is disabled') && 'error_SERVICE_DISABLED',
    lower.includes('cannot be cancelled') && 'error_CANNOT_CANCEL',
  ].filter(Boolean) as string[];
  for (const key of candidates) {
    const translated = t(key);
    if (translated !== key) return translated;
  }
  return raw;
}
