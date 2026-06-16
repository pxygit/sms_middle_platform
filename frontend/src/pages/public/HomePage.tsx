import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Alert, Button, Input, Space, Spin, Tag, message } from 'antd';
import { Copy, History, KeyRound, Phone, ShieldCheck, Sparkles, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cancelOrder, createOrder, getHistory, getOrder, recordVisit, verifyCard } from '../../api/public';
import type { CardVerifyResult, ReceiveOrder } from '../../types/api';
import { PreferenceBar } from '../../components/PreferenceBar';
import { localizedError } from '../../utils/errors';
import { formatPhone } from '../../utils/phone';
import { statusColor } from '../../utils/status';

const finalStatuses = ['sms_received', 'cancelled', 'expired', 'failed'];
const cancelWaitMs = 120000;

type ActiveOrderItem = {
  cardCode: string;
  order: ReceiveOrder;
};

function statusText(status?: string, t?: (key: string) => string) {
  const map: Record<string, string> = {
    created: t?.('created') || 'Waiting',
    active: t?.('waiting') || 'Waiting for SMS',
    cancel_requested: t?.('cancel_requested') || 'Cancelling',
    sms_received: t?.('received') || 'Received',
    cancelled: t?.('cancelled') || 'Cancelled',
    failed: t?.('failed') || 'Failed',
    expired: t?.('expired') || 'Expired',
  };
  return map[status || ''] || status || '-';
}

export function HomePage() {
  const { t } = useTranslation();
  const [cardCode, setCardCode] = useState(localStorage.getItem('lastCardCode') || '');
  const [verified, setVerified] = useState<CardVerifyResult | null>(null);
  const [orders, setOrders] = useState<ActiveOrderItem[]>(() => {
    const stored = localStorage.getItem('activeOrders') || localStorage.getItem('activeOrder');
    if (!stored) return [];
    try {
      const parsed = JSON.parse(stored);
      if (Array.isArray(parsed)) {
        return parsed.filter((item) => item?.cardCode && item?.order && !finalStatuses.includes(item.order.status));
      }
      if (parsed?.order && parsed?.cardCode) {
        return finalStatuses.includes(parsed.order.status) ? [] : [parsed];
      }
      return finalStatuses.includes(parsed.status) ? [] : [{ cardCode: localStorage.getItem('lastCardCode') || '', order: parsed }];
    } catch {
      return [];
    }
  });
  const [showHistory, setShowHistory] = useState(false);
  const [now, setNow] = useState(Date.now());
  const [msg, contextHolder] = message.useMessage();

  useEffect(() => {
    void recordVisit(window.location.pathname).catch(() => undefined);
  }, []);

  useEffect(() => {
    const active = orders.filter((item) => !finalStatuses.includes(item.order.status));
    if (active.length > 0) {
      localStorage.setItem('activeOrders', JSON.stringify(active));
      localStorage.removeItem('activeOrder');
      return;
    }
    localStorage.removeItem('activeOrders');
    localStorage.removeItem('activeOrder');
  }, [orders]);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const historyQuery = useQuery({
    queryKey: ['history', cardCode],
    queryFn: () => getHistory(cardCode),
    enabled: showHistory && cardCode.length > 0,
  });

  const verifyMutation = useMutation({
    mutationFn: verifyCard,
    onSuccess: (data) => {
      localStorage.setItem('lastCardCode', cardCode);
      setVerified(data);
      msg.success(t('verify'));
    },
    onError: (error: Error) => msg.error(localizedError(error.message, t)),
  });

  const createMutation = useMutation({
    mutationFn: createOrder,
    onSuccess: (data) => {
      localStorage.setItem('lastCardCode', cardCode);
      setOrders((current) => [{ cardCode, order: data }, ...current.filter((item) => item.order.orderNo !== data.orderNo)]);
      msg.success(t('requestNumber'));
    },
    onError: (error: Error) => msg.error(localizedError(error.message, t)),
  });

  const cancelMutation = useMutation({
    mutationFn: ({ orderNo, code }: { orderNo: string; code: string }) => cancelOrder(orderNo, code),
    onSuccess: (data) => {
      setOrders((current) => current.filter((item) => item.order.orderNo !== data.orderNo));
      void historyQuery.refetch();
      msg.success(t('cancelled'));
    },
    onError: (error: Error) => msg.error(localizedError(error.message, t)),
  });
  const cancellingOrderNo = cancelMutation.variables?.orderNo;
  const updateOrder = useCallback((nextOrder: ReceiveOrder) => {
    setOrders((current) => current.map((entry) => (
      entry.order.orderNo === nextOrder.orderNo && entry.order.updatedAt !== nextOrder.updatedAt
        ? { ...entry, order: nextOrder }
        : entry
    )));
  }, []);

  const serviceInfo = useMemo(() => {
    const service = verified?.serviceConfig || orders[0]?.order.serviceConfig;
    if (!service) return null;
    return {
      service: service.displayName || service.targetPlatform,
      country: service.countryName || service.countryCode,
    };
  }, [verified, orders]);

  const copy = async (value?: string, successKey = 'copied') => {
    if (!value) return;
    await navigator.clipboard.writeText(value);
    msg.success(t(successKey));
  };

  return (
    <main className="public-shell">
      {contextHolder}
      <div className="sunburst" />
      <header className="topbar">
        <div className="brand">
          <Sparkles size={24} />
          <span>{t('brand')}</span>
        </div>
        <Space>
          <Button href={localStorage.getItem('adminToken') ? '/admin' : '/admin/login'} shape="round" type="text">
            {t('admin')}
          </Button>
          <PreferenceBar />
        </Space>
      </header>

      <section className="hero-grid">
        <div className="hero-copy">
          <div className="eyebrow">
            <ShieldCheck size={16} />
            <span>{t('heroEyebrow')}</span>
          </div>
          <h1>{t('subtitle')}</h1>
          <p>{t('heroDescription')}</p>
        </div>

        <div className="glass-panel flow-panel">
          <label>{t('cardCode')}</label>
          <Input.Password
            size="large"
            value={cardCode}
            onChange={(event) => setCardCode(event.target.value)}
            placeholder={t('cardPlaceholder')}
            prefix={<KeyRound size={18} />}
          />
          <Space wrap>
            <Button size="large" shape="round" onClick={() => verifyMutation.mutate(cardCode)} loading={verifyMutation.isPending}>
              {t('verify')}
            </Button>
            <Button
              size="large"
              shape="round"
              type="primary"
              onClick={() => createMutation.mutate(cardCode)}
              loading={createMutation.isPending}
              disabled={!cardCode}
            >
              {t('requestNumber')}
            </Button>
            <Button size="large" shape="round" icon={<History size={17} />} onClick={() => setShowHistory((value) => !value)}>
              {t('history')}
            </Button>
          </Space>

          {serviceInfo && (
            <div className="service-pill">
              <div className="service-lines">
                <div className="service-line">
                  <span>{t('service')}</span>
                  <b>{serviceInfo.service}</b>
                </div>
                <div className="service-line">
                  <span>{t('country')}</span>
                  <small>{serviceInfo.country}</small>
                </div>
              </div>
              {verified && <Tag color="cyan">{t('remainingUses')}: {verified.remainingUses}</Tag>}
            </div>
          )}
        </div>
      </section>

      <section className="status-section">
        <div className="orders-stack">
          {orders.length === 0 ? (
            <div className="glass-panel order-panel">
              <div className="empty-state">
                <Phone size={42} />
                <span>{t('noOrder')}</span>
              </div>
            </div>
          ) : (
            orders.map((item) => (
              <OrderCard
                key={item.order.orderNo}
                item={item}
                now={now}
                cancelling={cancelMutation.isPending && cancellingOrderNo === item.order.orderNo}
                onCancel={(orderNo, code) => cancelMutation.mutate({ orderNo, code })}
                onCopy={copy}
                onUpdate={updateOrder}
              />
            ))
          )}
        </div>

        {showHistory && (
          <div className="glass-panel history-panel">
            <div className="section-title">{t('history')}</div>
            {historyQuery.isLoading && <Spin />}
            {historyQuery.data?.map((item) => (
              <div className="history-item" key={item.orderNo}>
                <span>{formatPhone(item).display || item.orderNo}</span>
                <Tag color={statusColor(item.status)}>{statusText(item.status, t)}</Tag>
              </div>
            ))}
          </div>
        )}
      </section>
    </main>
  );
}

function OrderCard({
  item,
  now,
  cancelling,
  onCancel,
  onCopy,
  onUpdate,
}: {
  item: ActiveOrderItem;
  now: number;
  cancelling: boolean;
  onCancel: (orderNo: string, cardCode: string) => void;
  onCopy: (value?: string, successKey?: string) => void;
  onUpdate: (order: ReceiveOrder) => void;
}) {
  const { t } = useTranslation();
  const { order, cardCode } = item;
  const phone = formatPhone(order || {});
  const cancelRemaining = cancelRemainingSeconds(order, now);
  const canCancel = Boolean(order.status === 'active' && !order.verificationCode && cancelRemaining <= 0);
  const canPoll = Boolean(order.orderNo && cardCode && !finalStatuses.includes(order.status));
  const orderQuery = useQuery({
    queryKey: ['order', order.orderNo, cardCode],
    queryFn: () => getOrder(order.orderNo, cardCode),
    enabled: canPoll,
    refetchInterval: canPoll ? 5000 : false,
  });

  useEffect(() => {
    if (orderQuery.data) onUpdate(orderQuery.data);
  }, [onUpdate, orderQuery.data]);

  return (
    <div className="glass-panel order-panel">
      <>
        <div className="order-head">
          <div>
            <small>{order.orderNo}</small>
            <h2>{statusText(order.status, t)}</h2>
          </div>
          {!finalStatuses.includes(order.status) && <Spin />}
        </div>
        <div className="metric-grid">
          <button className="metric" onClick={() => onCopy(phone.segment, 'copiedPhoneSegment')}>
            <span>{t('phoneNumber')}</span>
            <strong>{phone.display}</strong>
            <Copy size={16} />
          </button>
          <button className="metric code-metric" onClick={() => onCopy(order.verificationCode)}>
            <span>{t('verificationCode')}</span>
            <strong>{order.verificationCode || '------'}</strong>
            <Copy size={16} />
          </button>
        </div>
        {order.smsContent && <Alert type="success" showIcon message={order.smsContent} />}
        {order.failureReason && <Alert type="error" showIcon message={localizedError(order.failureReason, t)} />}
        {!finalStatuses.includes(order.status) && !order.verificationCode && (
          <Space className="cancel-row" wrap>
            <Button
              danger
              shape="round"
              icon={<XCircle size={17} />}
              onClick={() => onCancel(order.orderNo, cardCode)}
              loading={cancelling}
              disabled={!canCancel}
              title={!canCancel ? t('cancelWaitTip') : undefined}
            >
              {t('cancelNumber')}
            </Button>
            {!canCancel && cancelRemaining > 0 && <Tag color="warning">{formatCountdown(cancelRemaining)}</Tag>}
          </Space>
        )}
      </>
    </div>
  );
}

function cancelRemainingSeconds(order: ReceiveOrder, now: number) {
  if (!order.startedAt) return Math.ceil(cancelWaitMs / 1000);
  const elapsed = now - new Date(order.startedAt).getTime();
  return Math.max(0, Math.ceil((cancelWaitMs - elapsed) / 1000));
}

function formatCountdown(seconds: number) {
  const minutes = Math.floor(seconds / 60).toString().padStart(2, '0');
  const rest = (seconds % 60).toString().padStart(2, '0');
  return `${minutes}:${rest}`;
}
