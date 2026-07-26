import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Alert, Button, Input, Modal, Space, Spin, Tag, Tooltip, message } from 'antd';
import { BellRing, Copy, History, KeyRound, Phone, ShieldCheck, Sparkles, XCircle } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { cancelOrder, checkOrder, createOrder, getHistory, getOrder, listPublicAnnouncements, markAnnouncementRead, recordVisit, verifyCard } from '../../api/public';
import type { Announcement, CardVerifyResult, ReceiveOrder } from '../../types/api';
import { PreferenceBar } from '../../components/PreferenceBar';
import { localizedError } from '../../utils/errors';
import { formatDateTime } from '../../utils/format';
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
    completed: t?.('completed') || 'Completed',
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
  const navigate = useNavigate();
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
  const [announcementOpen, setAnnouncementOpen] = useState(false);
  const [activeAnnouncement, setActiveAnnouncement] = useState<Announcement | null>(null);
  const [readerId] = useState(getAnnouncementReaderId);
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

  const historyQuery = useQuery({
    queryKey: ['history', cardCode],
    queryFn: () => getHistory(cardCode),
    enabled: showHistory && cardCode.length > 0,
  });
  const announcementsQuery = useQuery({
    queryKey: ['public-announcements', readerId],
    queryFn: () => listPublicAnnouncements(readerId),
  });

  const readAnnouncementMutation = useMutation({
    mutationFn: (id: number) => markAnnouncementRead(id, readerId),
    onSuccess: () => announcementsQuery.refetch(),
  });

  useEffect(() => {
    const items = announcementsQuery.data || [];
    const modalAnnouncement = items.find((item) => item.notifyMode === 'modal' && item.unread && !localStorage.getItem(`announcementModalSeen:${item.id}`));
    if (modalAnnouncement) {
      localStorage.setItem(`announcementModalSeen:${modalAnnouncement.id}`, '1');
      setActiveAnnouncement(modalAnnouncement);
    }
  }, [announcementsQuery.data]);

  const openAnnouncement = (item: Announcement) => {
    setActiveAnnouncement({ ...item, unread: false });
    setAnnouncementOpen(false);
    if (item.unread) {
      readAnnouncementMutation.mutate(item.id);
    }
  };
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
  const historyItems = (historyQuery.data || []).filter(hasReceivedPhone);

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
          <AnnouncementBoard
            open={announcementOpen}
            items={announcementsQuery.data || []}
            loading={announcementsQuery.isLoading}
            onToggle={() => setAnnouncementOpen((value) => !value)}
            onClose={() => setAnnouncementOpen(false)}
            onOpen={openAnnouncement}
          />
          <Button
            shape="round"
            type="text"
            onClick={() => navigate(localStorage.getItem('adminToken') ? '/admin' : '/admin/login')}
          >
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
            {!historyQuery.isLoading && historyItems.length === 0 && (
              <div className="empty-state">
                <History size={28} />
                <span>{t('noRecords')}</span>
              </div>
            )}
            {historyItems.map((item) => (
              <div className="history-item" key={item.orderNo}>
                <span>{formatPhone(item).display || item.orderNo}</span>
                <Tag color={statusColor(item.status)}>{statusText(item.status, t)}</Tag>
              </div>
            ))}
          </div>
        )}
      </section>

      <AnnouncementModal
        announcement={activeAnnouncement}
        onClose={() => {
          if (activeAnnouncement?.unread) {
            readAnnouncementMutation.mutate(activeAnnouncement.id);
          }
          setActiveAnnouncement(null);
        }}
      />
      <PublicFooter />
    </main>
  );
}

function AnnouncementBoard({
  open,
  items,
  loading,
  onToggle,
  onClose,
  onOpen,
}: {
  open: boolean;
  items: Announcement[];
  loading: boolean;
  onToggle: () => void;
  onClose: () => void;
  onOpen: (item: Announcement) => void;
}) {
  const { t } = useTranslation();
  const unreadCount = items.filter((item) => item.unread).length;
  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target?.closest('.announcement-board')) onClose();
    };
    window.addEventListener('mousedown', close);
    return () => window.removeEventListener('mousedown', close);
  }, [onClose, open]);
  return (
    <div className={`announcement-board${open ? ' is-open' : ''}`}>
      <Tooltip title={t('announcements')}>
        <Button className="announcement-trigger" shape="circle" type="text" icon={<BellRing size={17} />} onClick={onToggle} aria-label={t('announcements')}>
          {unreadCount > 0 && <span className="announcement-unread-badge" />}
        </Button>
      </Tooltip>
      {open && (
        <div className="announcement-popover">
          <div className="announcement-popover-head">
            <strong>{t('announcements')}</strong>
            {unreadCount > 0 && <Tag color="blue">{t('unreadCount', { count: unreadCount })}</Tag>}
          </div>
          <div className="announcement-list">
            {loading && <Spin />}
            {!loading && items.length === 0 && <div className="announcement-empty">{t('noAnnouncements')}</div>}
            {items.map((item) => (
              <button className="announcement-list-item" key={item.id} onClick={() => onOpen(item)}>
                <span>{item.title}</span>
                <small>{formatDateTime(item.publishedAt || item.createdAt)}</small>
                {item.unread && <i />}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function AnnouncementModal({ announcement, onClose }: { announcement: Announcement | null; onClose: () => void }) {
  const { t } = useTranslation();
  return (
    <Modal
      open={Boolean(announcement)}
      title={announcement?.title}
      onCancel={onClose}
      onOk={onClose}
      okText={t('read')}
      cancelButtonProps={{ style: { display: 'none' } }}
      className="announcement-modal"
    >
      <div className="announcement-modal-meta">{announcement && formatDateTime(announcement.publishedAt || announcement.createdAt)}</div>
      <div className="markdown-body">{renderMarkdown(announcement?.content || '')}</div>
    </Modal>
  );
}

function renderMarkdown(text: string) {
  const blocks = text.split(/\n{2,}/).map((block, index) => {
    const trimmed = block.trim();
    if (!trimmed) return null;
    if (trimmed.startsWith('### ')) return <h3 key={index}>{renderInlineMarkdown(trimmed.slice(4))}</h3>;
    if (trimmed.startsWith('## ')) return <h2 key={index}>{renderInlineMarkdown(trimmed.slice(3))}</h2>;
    if (trimmed.startsWith('# ')) return <h1 key={index}>{renderInlineMarkdown(trimmed.slice(2))}</h1>;
    const lines = trimmed.split('\n');
    if (/^[-*] /.test(trimmed)) {
      return <ul key={index}>{lines.map((line, lineIndex) => <li key={lineIndex}>{renderInlineMarkdown(line.replace(/^[-*] /, ''))}</li>)}</ul>;
    }
    return <p key={index}>{lines.map((line, lineIndex) => <span key={lineIndex}>{renderInlineMarkdown(line)}{lineIndex < lines.length - 1 && <br />}</span>)}</p>;
  });
  return blocks;
}

function renderInlineMarkdown(text: string) {
  const parts: ReactNode[] = [];
  const pattern = /(\*\*[^*]+\*\*|\[[^\]]+\]\([^)]+\))/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(text)) !== null) {
    if (match.index > lastIndex) parts.push(text.slice(lastIndex, match.index));
    const token = match[0];
    if (token.startsWith('**')) {
      parts.push(<strong key={parts.length}>{token.slice(2, -2)}</strong>);
    } else {
      const link = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
      if (link) {
        const href = safeMarkdownHref(link[2]);
        parts.push(href ? <a key={parts.length} href={href} target="_blank" rel="noreferrer">{link[1]}</a> : link[1]);
      }
    }
    lastIndex = pattern.lastIndex;
  }
  if (lastIndex < text.length) parts.push(text.slice(lastIndex));
  return parts;
}

function safeMarkdownHref(value: string) {
  const trimmed = value.trim();
  try {
    const url = new URL(trimmed, window.location.origin);
    return ['http:', 'https:', 'mailto:'].includes(url.protocol) ? trimmed : '';
  } catch {
    return '';
  }
}

function getAnnouncementReaderId() {
  const key = 'announcementReaderId';
  const existing = localStorage.getItem(key);
  if (existing) return existing;
  const value = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  localStorage.setItem(key, value);
  return value;
}
function PublicFooter() {
  const { i18n, t } = useTranslation();
  return <footer className="public-footer" key={i18n.resolvedLanguage || i18n.language}>{t('copyright')}</footer>;
}

function hasReceivedPhone(order: ReceiveOrder) {
  return Boolean(order.phoneNumber || order.phoneNationalNumber);
}

function OrderCard({
  item,
  cancelling,
  onCancel,
  onCopy,
  onUpdate,
}: {
  item: ActiveOrderItem;
  cancelling: boolean;
  onCancel: (orderNo: string, cardCode: string) => void;
  onCopy: (value?: string, successKey?: string) => void;
  onUpdate: (order: ReceiveOrder) => void;
}) {
  const { t } = useTranslation();
  const { order, cardCode } = item;
  const phone = formatPhone(order || {});
  const cancelRemaining = useCancelCountdown(order);
  const isManual = Boolean(order.manualCheck);
  const canCancel = Boolean(order.status === 'active' && !order.verificationCode && cancelRemaining <= 0 && !isManual);
  const canPoll = Boolean(order.orderNo && cardCode && !finalStatuses.includes(order.status) && !isManual && order.status !== 'completed');
  const orderQuery = useQuery({
    queryKey: ['order', order.orderNo, cardCode],
    queryFn: () => getOrder(order.orderNo, cardCode),
    enabled: canPoll,
    refetchInterval: canPoll ? 5000 : false,
  });
  const checkMutation = useMutation({
    mutationFn: () => checkOrder(order.orderNo, cardCode),
    onSuccess: (data) => {
      onUpdate(data);
      if (data.verificationCode) {
        message.success(t('received'));
      } else {
        message.info(t('noSMSYet'));
      }
    },
    onError: (error: Error) => message.error(localizedError(error.message, t)),
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
          {!isManual && order.status !== 'completed' && !finalStatuses.includes(order.status) && <Spin />}
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
        {isManual && (
          <div className="manual-check-row">
            <button className="manual-check-url" onClick={() => onCopy(order.messageUrl)} disabled={!order.messageUrl}>
              <span>{t('messageUrl')}</span>
              <strong>{order.messageUrl || '-'}</strong>
              <Copy size={16} />
            </button>
            <Button
              shape="round"
              type="primary"
              loading={checkMutation.isPending}
              onClick={() => checkMutation.mutate()}
            >
              {t('checkSMS')}
            </Button>
          </div>
        )}
        {order.smsContent && <Alert type="success" showIcon message={order.smsContent} />}
        {order.failureReason && <Alert type="error" showIcon message={localizedError(order.failureReason, t)} />}
        {!isManual && order.status !== 'completed' && !finalStatuses.includes(order.status) && !order.verificationCode && (
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

/** Ticks once per second only while the countdown is still running, so the rest of the page stays untouched. */
function useCancelCountdown(order: ReceiveOrder) {
  const [now, setNow] = useState(() => Date.now());
  const remaining = cancelRemainingSeconds(order, now);
  const running = Boolean(order.startedAt) && remaining > 0 && !finalStatuses.includes(order.status) && !order.verificationCode;

  useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [running]);

  return remaining;
}

function formatCountdown(seconds: number) {
  const minutes = Math.floor(seconds / 60).toString().padStart(2, '0');
  const rest = (seconds % 60).toString().padStart(2, '0');
  return `${minutes}:${rest}`;
}
