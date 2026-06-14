import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Alert, Button, Input, Space, Spin, Tag, message } from 'antd';
import { Copy, History, KeyRound, Phone, ShieldCheck, Sparkles, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cancelOrder, createOrder, getHistory, getOrder, verifyCard } from '../../api/public';
import type { CardVerifyResult, ReceiveOrder } from '../../types/api';
import { PreferenceBar } from '../../components/PreferenceBar';

const finalStatuses = ['sms_received', 'cancelled', 'expired', 'failed'];

function statusText(status?: string, t?: (key: string) => string) {
  const map: Record<string, string> = {
    active: t?.('active') || 'Active',
    sms_received: t?.('received') || 'Received',
    cancelled: t?.('cancelled') || 'Cancelled',
    failed: t?.('failed') || 'Failed',
    expired: t?.('expired') || 'Expired',
    created: t?.('waiting') || 'Waiting',
    cancel_requested: t?.('waiting') || 'Waiting',
  };
  return map[status || ''] || status || '-';
}

export function HomePage() {
  const { t } = useTranslation();
  const [cardCode, setCardCode] = useState(localStorage.getItem('lastCardCode') || '');
  const [verified, setVerified] = useState<CardVerifyResult | null>(null);
  const [order, setOrder] = useState<ReceiveOrder | null>(null);
  const [showHistory, setShowHistory] = useState(false);
  const [msg, contextHolder] = message.useMessage();

  const canPoll = Boolean(order?.orderNo && cardCode && !finalStatuses.includes(order.status));

  const orderQuery = useQuery({
    queryKey: ['order', order?.orderNo, cardCode],
    queryFn: () => getOrder(order!.orderNo, cardCode),
    enabled: canPoll,
    refetchInterval: canPoll ? 5000 : false,
  });

  useEffect(() => {
    if (orderQuery.data) setOrder(orderQuery.data);
  }, [orderQuery.data]);

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
    onError: (error: Error) => msg.error(error.message),
  });

  const createMutation = useMutation({
    mutationFn: createOrder,
    onSuccess: (data) => {
      localStorage.setItem('lastCardCode', cardCode);
      setOrder(data);
      msg.success(t('requestNumber'));
    },
    onError: (error: Error) => msg.error(error.message),
  });

  const cancelMutation = useMutation({
    mutationFn: () => cancelOrder(order!.orderNo, cardCode),
    onSuccess: (data) => {
      setOrder(data);
      msg.success(t('cancelled'));
    },
    onError: (error: Error) => msg.error(error.message),
  });

  const serviceLine = useMemo(() => {
    const service = verified?.serviceConfig || order?.serviceConfig;
    if (!service) return '';
    return `${service.displayName} · ${service.countryName || service.countryCode}`;
  }, [verified, order]);

  const copy = async (value?: string) => {
    if (!value) return;
    await navigator.clipboard.writeText(value);
    msg.success('Copied');
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
          <Button href="/admin/login" shape="round" type="text">
            {t('admin')}
          </Button>
          <PreferenceBar />
        </Space>
      </header>

      <section className="hero-grid">
        <div className="hero-copy">
          <div className="eyebrow">
            <ShieldCheck size={16} />
            <span>SMS Receive Platform</span>
          </div>
          <h1>{t('subtitle')}</h1>
          <p>Fast, clean, and ready for one-time verification workflows.</p>
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
            <Button
              size="large"
              shape="round"
              onClick={() => verifyMutation.mutate(cardCode)}
              loading={verifyMutation.isPending}
            >
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
            <Button
              size="large"
              shape="round"
              icon={<History size={17} />}
              onClick={() => setShowHistory((value) => !value)}
            >
              {t('history')}
            </Button>
          </Space>

          {serviceLine && (
            <div className="service-pill">
              <span>{t('service')}</span>
              <b>{serviceLine}</b>
              {verified && <Tag color="cyan">{t('remainingUses')}: {verified.remainingUses}</Tag>}
            </div>
          )}
        </div>
      </section>

      <section className="status-section">
        <div className="glass-panel order-panel">
          {!order ? (
            <div className="empty-state">
              <Phone size={42} />
              <span>{t('noOrder')}</span>
            </div>
          ) : (
            <>
              <div className="order-head">
                <div>
                  <small>{order.orderNo}</small>
                  <h2>{statusText(order.status, t)}</h2>
                </div>
                {!finalStatuses.includes(order.status) && <Spin />}
              </div>
              <div className="metric-grid">
                <button className="metric" onClick={() => copy(order.phoneNumber)}>
                  <span>{t('phoneNumber')}</span>
                  <strong>{order.phoneNumber || '-'}</strong>
                  <Copy size={16} />
                </button>
                <button className="metric code-metric" onClick={() => copy(order.verificationCode)}>
                  <span>{t('verificationCode')}</span>
                  <strong>{order.verificationCode || '••••••'}</strong>
                  <Copy size={16} />
                </button>
              </div>
              {order.smsContent && <Alert type="success" showIcon message={order.smsContent} />}
              {order.failureReason && <Alert type="error" showIcon message={order.failureReason} />}
              {!finalStatuses.includes(order.status) && (
                <Button
                  danger
                  shape="round"
                  icon={<XCircle size={17} />}
                  onClick={() => cancelMutation.mutate()}
                  loading={cancelMutation.isPending}
                >
                  {t('cancelNumber')}
                </Button>
              )}
            </>
          )}
        </div>

        {showHistory && (
          <div className="glass-panel history-panel">
            <div className="section-title">{t('history')}</div>
            {historyQuery.isLoading && <Spin />}
            {historyQuery.data?.map((item) => (
              <div className="history-item" key={item.orderNo}>
                <span>{item.phoneNumber || item.orderNo}</span>
                <Tag>{statusText(item.status, t)}</Tag>
              </div>
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
