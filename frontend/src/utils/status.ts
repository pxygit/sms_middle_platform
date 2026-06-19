export function statusColor(status?: string) {
  const colors: Record<string, string> = {
    created: 'default',
    active: 'processing',
    completed: 'processing',
    cancel_requested: 'warning',
    sms_received: 'success',
    cancelled: 'default',
    expired: 'orange',
    failed: 'error',
    enabled: 'success',
    disabled: 'default',
    voided: 'error',
  };
  return colors[status || ''] || 'default';
}
