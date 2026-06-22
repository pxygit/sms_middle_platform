import { useEffect, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Button,
  DatePicker,
  Drawer,
  Popconfirm,
  Form,
  Input,
  InputNumber,
  Layout,
  Menu,
  Modal,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import {
  BellRing,
  Boxes,
  ClipboardList,
  Copy,
  Database,
  Download,
  Eye,
  EyeOff,
  Pencil,
  Radar,
  Home,
  KeyRound,
  LockKeyhole,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCw,
  Search,
  ScrollText,
  Settings2,
  Sparkles,
  Trash2,
} from 'lucide-react';
import { Link, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  changePassword,
  createAnnouncement,
  createCardBatch,
  deleteAnnouncement,
  createServiceConfig,
  deleteServiceConfig,
  deleteCardBatch,
  deleteCardCode,
  downloadCardBatch,
  getDashboardStats,
  listAnnouncements,
  getProviderBalance,
  getProviderQuote,
  getProviderValidityOptions,
  listAuditLogs,
  listCardBatches,
  listCardCodes,
  listOrders,
  listProviderCountries,
  listProviderServices,
  listProviders,
  listServiceConfigs,
  revealCardCode,
  updateAnnouncement,
  updateProvider,
  updateServiceConfig,
  updateCardStatus,
} from '../../api/admin';
import { PreferenceBar } from '../../components/PreferenceBar';
import { formatDateTime } from '../../utils/format';
import type { Announcement, AuditLog, DashboardRank, ProviderCountry, ProviderPrice, ProviderService, ProviderStock, ProviderValidityOption, SMSProvider } from '../../types/api';
import { statusColor } from '../../utils/status';
import { localizedError } from '../../utils/errors';

const { Header, Sider, Content } = Layout;
const currencyOptions = ['USD', 'CNY', 'EUR', 'GBP', 'HKD', 'JPY', 'USDT'];

export function AdminLayout() {
  const token = localStorage.getItem('adminToken');
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState(false);
  const [passwordOpen, setPasswordOpen] = useState(false);

  if (!token) return <Navigate to="/admin/login" replace />;

  const logout = () => {
    localStorage.removeItem('adminToken');
    localStorage.removeItem('adminUser');
    navigate('/admin/login');
  };

  const selected = location.pathname.split('/')[2] || 'dashboard';

  return (
    <Layout className={`admin-shell${collapsed ? ' is-collapsed' : ''}`}>
      <Sider width={244} collapsedWidth={78} collapsed={collapsed} trigger={null} className="admin-sider">
        <div className="admin-brand-row">
          <Link to="/" className="admin-brand">
            <Sparkles size={collapsed ? 28 : 30} />
            {!collapsed && <span>{t('brand')}</span>}
          </Link>
          <Tooltip title={collapsed ? t('unfoldMenu') : t('foldMenu')}>
            <Button
              type="text"
              shape="circle"
              icon={collapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
              onClick={() => setCollapsed((value) => !value)}
            />
          </Tooltip>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selected]}
          inlineCollapsed={collapsed}
          items={[
            { key: 'dashboard', icon: <Home size={18} />, label: <Link to="/admin">{t('dashboard')}</Link> },
            { key: 'announcements', icon: <BellRing size={18} />, label: <Link to="/admin/announcements">{t('announcements')}</Link> },
            { key: 'providers', icon: <Database size={18} />, label: <Link to="/admin/providers">{t('providers')}</Link> },
            { key: 'services', icon: <Settings2 size={18} />, label: <Link to="/admin/services">{t('serviceConfigs')}</Link> },
            { key: 'batches', icon: <Boxes size={18} />, label: <Link to="/admin/batches">{t('cardBatches')}</Link> },
            { key: 'cards', icon: <KeyRound size={18} />, label: <Link to="/admin/cards">{t('cardCodes')}</Link> },
            { key: 'orders', icon: <ClipboardList size={18} />, label: <Link to="/admin/orders">{t('orders')}</Link> },
            { key: 'audit', icon: <ScrollText size={18} />, label: <Link to="/admin/audit">{t('auditLogs')}</Link> },
          ]}
        />
      </Sider>
      <Layout className="admin-main-layout">
        <Header className="admin-header">
          <PreferenceBar compact />
          <Space>
            <Button shape="round" icon={<LockKeyhole size={16} />} onClick={() => setPasswordOpen(true)}>
              {t('changePassword')}
            </Button>
            <Button shape="round" icon={<LogOut size={16} />} onClick={logout}>
              {t('logout')}
            </Button>
          </Space>
        </Header>
        <Content className="admin-content">
          <Routes>
            <Route index element={<Dashboard />} />
            <Route path="announcements" element={<AnnouncementsPage />} />
            <Route path="providers" element={<ProvidersPage />} />
            <Route path="services" element={<ServicesPage />} />
            <Route path="batches" element={<BatchesPage />} />
            <Route path="cards" element={<CardsPage />} />
            <Route path="orders" element={<OrdersPage />} />
            <Route path="audit" element={<AuditPage />} />
          </Routes>
        </Content>
      </Layout>
      <PasswordModal open={passwordOpen} onClose={() => setPasswordOpen(false)} />
    </Layout>
  );
}

function Dashboard() {
  const { t } = useTranslation();
  const [balances, setBalances] = useState<Record<string, { balance?: string; error?: string; checkedAt?: string }>>({});
  const [checkingProviders, setCheckingProviders] = useState<Record<string, boolean>>({});
  const stats = useQuery({ queryKey: ['dashboard'], queryFn: getDashboardStats, refetchInterval: 30000 });
  const providers = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const checkProviderBalance = async (provider: SMSProvider) => {
    setCheckingProviders((current) => ({ ...current, [provider.code]: true }));
    try {
      const data = await getProviderBalance(provider.code);
      setBalances((current) => ({ ...current, [provider.code]: { balance: data.balance, checkedAt: data.checkedAt || new Date().toISOString() } }));
      void providers.refetch();
    } catch (error: any) {
      setBalances((current) => ({ ...current, [provider.code]: { error: localizedError(error?.message || t('balanceCheckFailed'), t), checkedAt: new Date().toISOString() } }));
    } finally {
      setCheckingProviders((current) => ({ ...current, [provider.code]: false }));
    }
  };
  const balanceMutation = useMutation({
    mutationFn: async () => {
      const targets = providers.data || [];
      const results = await Promise.all(
        targets.map(async (provider) => {
          setCheckingProviders((current) => ({ ...current, [provider.code]: true }));
          try {
            const data = await getProviderBalance(provider.code);
            return [provider.code, { balance: data.balance, checkedAt: data.checkedAt || new Date().toISOString() }] as const;
          } catch (error: any) {
            return [provider.code, { error: localizedError(error?.message || t('balanceCheckFailed'), t), checkedAt: new Date().toISOString() }] as const;
          }
        }),
      );
      return Object.fromEntries(results);
    },
    onSuccess: (result) => {
      setBalances((current) => ({ ...current, ...result }));
      void providers.refetch();
    },
    onSettled: () => setCheckingProviders({}),
  });
  const data = stats.data;
  return (
    <div className="admin-page">
      <PageHead title={t('dashboard')} onRefresh={() => stats.refetch()} />
      <div className="dashboard-hero">
        <div>
          <span>{t('dashboardToday')}</span>
          <h2>{t('dashboardTitle')}</h2>
        </div>
        <Tag color="cyan">{t('updatedAt')}: {formatDateTime(new Date().toISOString())}</Tag>
      </div>
      <div className="admin-stat-grid dashboard-stat-grid">
        <Stat title={t('completedOrders')} value={data?.totalCompletedOrders || 0} tone="mint" />
        <Stat title={t('todayCompleted')} value={data?.todayCompletedOrders || 0} tone="sky" />
        <Stat title={t('homeVisits')} value={data?.totalVisits || 0} tone="sun" />
        <Stat title={t('todayVisits')} value={data?.todayVisits || 0} tone="violet" />
        <Stat title={t('activeOrders')} value={data?.activeOrders || 0} tone="sky" />
        <Stat title={t('availableCards')} value={data?.availableCards || 0} tone="mint" />
      </div>
      <BalancePanel
        providers={providers.data || []}
        balances={balances}
        allLoading={providers.isLoading || balanceMutation.isPending}
        loadingByCode={checkingProviders}
        onCheck={(provider) => void checkProviderBalance(provider)}
        onCheckAll={() => balanceMutation.mutate()}
      />
      <div className="dashboard-grid">
        <RankPanel title={t('providerRank')} rows={data?.providerRanking || []} loading={stats.isLoading} />
        <RankPanel title={t('serviceRank')} rows={data?.serviceRanking || []} loading={stats.isLoading} />
        <RankPanel title={t('statusOverview')} rows={data?.statusSummary || []} loading={stats.isLoading} translateName />
      </div>
    </div>
  );
}

function BalancePanel({
  providers,
  balances,
  allLoading,
  loadingByCode,
  onCheck,
  onCheckAll,
}: {
  providers: SMSProvider[];
  balances: Record<string, { balance?: string; error?: string; checkedAt?: string }>;
  allLoading: boolean;
  loadingByCode: Record<string, boolean>;
  onCheck: (provider: SMSProvider) => void;
  onCheckAll: () => void;
}) {
  const { t } = useTranslation();
  return (
    <section className="balance-panel">
      <div className="rank-head">
        <div>
          <h2>{t('providerBalance')}</h2>
          <small>{t('providerBalanceHelp')}</small>
        </div>
        <Button shape="round" loading={allLoading} onClick={onCheckAll}>
          {t('checkAllBalances')}
        </Button>
      </div>
      <div className="balance-grid">
        {providers.length === 0 ? (
          <div className="rank-empty">{t('noData')}</div>
        ) : (
          providers.map((provider) => {
            const result = balances[provider.code] || { balance: provider.lastBalance, checkedAt: provider.lastBalanceCheckedAt };
            return (
              <div className="balance-card" key={provider.code}>
                <div>
                  <span>{provider.name || provider.code}</span>
                  <strong>{result.error ? t('checkFailed') : formatMoney(result.balance, provider.currencyCode)}</strong>
                  {result.checkedAt && <small>{formatDateTime(result.checkedAt)}</small>}
                  {result.error && <em>{result.error}</em>}
                </div>
                <Button size="small" shape="circle" icon={<RefreshCw size={15} />} loading={Boolean(loadingByCode[provider.code])} onClick={() => onCheck(provider)} />
              </div>
            );
          })
        )}
      </div>
    </section>
  );
}

function Stat({ title, value, tone = 'mint' }: { title: string; value: number; tone?: string }) {
  return (
    <div className={`stat-card stat-${tone}`}>
      <span>{title}</span>
      <strong>{value}</strong>
    </div>
  );
}

function RankPanel({ title, rows, loading, translateName = false }: { title: string; rows: DashboardRank[]; loading: boolean; translateName?: boolean }) {
  const { t } = useTranslation();
  const max = Math.max(...rows.map((row) => row.count), 1);
  return (
    <section className="rank-panel">
      <div className="rank-head">
        <h2>{title}</h2>
        {loading && <Tag color="processing">{t('loading')}</Tag>}
      </div>
      {rows.length === 0 ? (
        <div className="rank-empty">{t('noData')}</div>
      ) : (
        rows.map((row, index) => (
          <div className="rank-row" key={`${row.key}-${index}`}>
            <div className="rank-row-main">
              <span>{index + 1}</span>
              <b>{translateName ? translatedStatus(row.name, t) : row.name}</b>
              <strong>{row.count}</strong>
            </div>
            <div className="rank-track">
              <i style={{ width: `${Math.max((row.count / max) * 100, 8)}%` }} />
            </div>
          </div>
        ))
      )}
    </section>
  );
}

function AnnouncementsPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string | undefined>();
  const [notifyMode, setNotifyMode] = useState<string | undefined>();
  const [editing, setEditing] = useState<Announcement | null>(null);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const query = useQuery({ queryKey: ['announcements', keyword, status, notifyMode], queryFn: () => listAnnouncements({ keyword, status, notifyMode, limit: 200 }) });
  const saveMutation = useMutation({
    mutationFn: (values: Partial<Announcement>) => editing ? updateAnnouncement(editing.id, values) : createAnnouncement(values),
    onSuccess: () => {
      form.resetFields();
      setEditing(null);
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ['announcements'] });
    },
    onError: (error: Error) => message.error(localizedError(error.message, t)),
  });
  const deleteMutation = useMutation({
    mutationFn: deleteAnnouncement,
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['announcements'] }),
    onError: (error: Error) => message.error(localizedError(error.message, t)),
  });
  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ status: 'draft', notifyMode: 'silent' });
    setOpen(true);
  };
  const openEdit = (record: Announcement) => {
    setEditing(record);
    form.setFieldsValue({
      ...record,
      startAtInput: toDateTimeInput(record.startAt),
      endAtInput: toDateTimeInput(record.endAt),
    });
    setOpen(true);
  };
  const submit = (values: any) => {
    saveMutation.mutate({
      title: values.title,
      content: values.content,
      status: values.status,
      notifyMode: values.notifyMode,
      startAt: fromDateTimeInput(values.startAtInput),
      endAt: fromDateTimeInput(values.endAtInput),
    });
  };
  const extraActions = (
    <Space wrap>
      <Select
        allowClear
        className="admin-filter-select"
        placeholder={t('announcementStatus')}
        value={status}
        onChange={setStatus}
        options={announcementStatusOptions(t)}
      />
      <Select
        allowClear
        className="admin-filter-select"
        placeholder={t('notifyMode')}
        value={notifyMode}
        onChange={setNotifyMode}
        options={announcementNotifyOptions(t)}
      />
    </Space>
  );
  return (
    <div className="admin-page">
      <PageHead
        title={t('announcements')}
        searchValue={keyword}
        onSearchChange={setKeyword}
        extraActions={extraActions}
        onCreate={openCreate}
        onRefresh={() => query.refetch()}
      />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="id"
        loading={query.isLoading}
        dataSource={query.data || []}
        pagination={tablePagination(t)}
        columns={[
          centerColumn({ title: t('announcementTitle'), dataIndex: 'title', width: 260 }),
          centerColumn({ title: t('announcementStatus'), dataIndex: 'status', render: (value: string) => <Tag color={announcementStatusColor(value)}>{announcementStatusText(value, t)}</Tag> }),
          centerColumn({ title: t('notifyMode'), dataIndex: 'notifyMode', render: (value: string) => <Tag color={value === 'modal' ? 'blue' : 'cyan'}>{announcementNotifyText(value, t)}</Tag> }),
          centerColumn({ title: t('readCount'), dataIndex: 'readCount' }),
          centerColumn({ title: t('validPeriod'), render: (_: unknown, row: Announcement) => formatAnnouncementPeriod(row, t) }),
          centerColumn({ title: t('createdAt'), dataIndex: 'createdAt', render: formatDateTime, sorter: (a: Announcement, b: Announcement) => dateSorter(a.createdAt, b.createdAt) }),
          centerColumn({
            title: t('actions'),
            fixed: 'right',
            render: (_: unknown, row: Announcement) => (
              <Space>
                <Tooltip title={t('edit')}>
                  <Button size="small" shape="circle" icon={<Pencil size={15} />} onClick={() => openEdit(row)} />
                </Tooltip>
                <Popconfirm title={t('confirmDelete')} onConfirm={() => deleteMutation.mutate(row.id)}>
                  <Button size="small" shape="circle" danger icon={<Trash2 size={15} />} />
                </Popconfirm>
              </Space>
            ),
          }),
        ]}
      />
      <Drawer title={editing ? t('editAnnouncement') : t('createAnnouncement')} width={860} open={open} onClose={() => setOpen(false)}>
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item name="title" label={t('announcementTitle')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="content" label={t('announcementContent')} rules={[{ required: true }]}>
            <Input.TextArea autoSize={{ minRows: 7, maxRows: 14 }} />
          </Form.Item>
          <div className="announcement-form-grid">
            <Form.Item name="status" label={t('announcementStatus')} rules={[{ required: true }]}>
              <Select options={announcementStatusOptions(t)} />
            </Form.Item>
            <Form.Item name="notifyMode" label={t('notifyMode')} tooltip={t('notifyModeHelp')} rules={[{ required: true }]}>
              <Select options={announcementNotifyOptions(t)} />
            </Form.Item>
          </div>
          <div className="announcement-form-grid">
            <Form.Item name="startAtInput" label={t('startAt')} tooltip={t('announcementTimeHelp')}>
              <Input type="datetime-local" />
            </Form.Item>
            <Form.Item name="endAtInput" label={t('endAt')} tooltip={t('announcementTimeHelp')}>
              <Input type="datetime-local" />
            </Form.Item>
          </div>
          <Button htmlType="submit" type="primary" shape="round" loading={saveMutation.isPending}>{t('save')}</Button>
        </Form>
      </Drawer>
    </div>
  );
}
function ProvidersPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<SMSProvider | null>(null);
  const [keyword, setKeyword] = useState('');
  const [balances, setBalances] = useState<Record<string, { balance?: string; error?: string; checkedAt?: string }>>({});
  const [checkingProviders, setCheckingProviders] = useState<Record<string, boolean>>({});
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 });
  const [form] = Form.useForm();
  const query = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const rows = filterRows(query.data || [], keyword);
  const mutation = useMutation({
    mutationFn: ({ code, values }: { code: string; values: any }) => updateProvider(code, values),
    onSuccess: () => {
      form.resetFields();
      setEditing(null);
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ['providers'] });
      void qc.invalidateQueries({ queryKey: ['dashboard'] });
    },
  });
  const checkProviderBalance = async (provider: SMSProvider) => {
    setCheckingProviders((current) => ({ ...current, [provider.code]: true }));
    try {
      const data = await getProviderBalance(provider.code);
      setBalances((current) => ({ ...current, [provider.code]: { balance: data.balance, checkedAt: data.checkedAt || new Date().toISOString() } }));
      void qc.invalidateQueries({ queryKey: ['providers'] });
    } catch (error: any) {
      setBalances((current) => ({ ...current, [provider.code]: { error: localizedError(error?.message || t('balanceCheckFailed'), t), checkedAt: new Date().toISOString() } }));
    } finally {
      setCheckingProviders((current) => ({ ...current, [provider.code]: false }));
    }
  };
  const currentPageRows = rows.slice((pagination.current - 1) * pagination.pageSize, pagination.current * pagination.pageSize);
  const checkingCurrentPage = currentPageRows.some((provider) => checkingProviders[provider.code]);

  const openEdit = (record: SMSProvider) => {
    setEditing(record);
    form.setFieldsValue({
      ...record,
      apiKey: '',
      loginCredential: '',
      authMode: record.code === '62-us' ? (record.authMode || 'account_password') : undefined,
      account: '',
      password: '',
      sms68Token: '',
      sms68Cookie: '',
      sms68Communication: '',
    });
    setOpen(true);
  };
  const submitProvider = (values: any) => {
    if (!editing) return;
    const nextValues = { ...values };
    let hasCredential = Boolean(String(values.loginCredential || '').trim()) || Boolean(editing.loginCredentialSet || editing.metadataTokenSet);

    if (editing.code === '62-us') {
      const authMode = values.authMode === 'openapi_token' ? 'openapi_token' : 'account_password';
      nextValues.authMode = authMode;
      if (authMode === 'account_password') {
        delete nextValues.apiKey;
        hasCredential = Boolean(String(values.account || '').trim() && String(values.password || '').trim()) || Boolean(editing.loginCredentialSet || editing.metadataTokenSet);
      } else {
        delete nextValues.account;
        delete nextValues.password;
        hasCredential = Boolean(String(values.apiKey || '').trim()) || Boolean(editing.apiKeySet);
      }
    }

    if (editing.code !== '62-us') {
      delete nextValues.authMode;
      delete nextValues.account;
      delete nextValues.password;
    }

    if (editing.code === '68sms') {
      const loginCredential = buildSMS68LoginCredential(values);
      if (loginCredential) {
        nextValues.loginCredential = loginCredential;
      } else {
        delete nextValues.loginCredential;
      }
      hasCredential = Boolean(loginCredential) || Boolean(editing.loginCredentialSet || editing.metadataTokenSet);
    }

    delete nextValues.sms68Token;
    delete nextValues.sms68Cookie;
    delete nextValues.sms68Communication;

    if (editing.requiresLoginCredential && !hasCredential) {
      Modal.confirm({
        title: t('loginCredentialRequiredTitle'),
        content: t('loginCredentialRequiredConfirm'),
        okText: t('save'),
        cancelText: t('cancel'),
        onOk: () => mutation.mutate({ code: editing.code, values: nextValues }),
      });
      return;
    }
    mutation.mutate({ code: editing.code, values: nextValues });
  };

  return (
    <div className="admin-page">
      <PageHead
        title={t('providers')}
        searchValue={keyword}
        onSearchChange={setKeyword}
        extraActions={(
          <Tooltip title={t('checkCurrentPage')}>
            <Button
              shape="circle"
              icon={<Radar size={16} />}
              loading={checkingCurrentPage}
              onClick={() => void Promise.all(currentPageRows.map((provider) => checkProviderBalance(provider)))}
            />
          </Tooltip>
        )}
        onRefresh={() => query.refetch()}
      />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="code"
        dataSource={rows}
        loading={query.isLoading}
        pagination={{
          ...tablePagination(t),
          current: pagination.current,
          pageSize: pagination.pageSize,
        }}
        onChange={(nextPagination) => {
          setPagination({
            current: nextPagination.current || 1,
            pageSize: nextPagination.pageSize || 10,
          });
        }}
        columns={[
          centerColumn({ title: t('providerCode'), dataIndex: 'code' }),
          centerColumn({ title: t('providerName'), dataIndex: 'name' }),
          centerColumn({ title: t('baseUrl'), dataIndex: 'baseUrl', width: 260 }),
          centerColumn({ title: t('currencyCode'), dataIndex: 'currencyCode' }),
          centerColumn({ title: t('apiKey'), dataIndex: 'apiKeySet', render: (value: boolean) => <Tag color={value ? 'green' : 'orange'}>{value ? t('configured') : t('notConfigured')}</Tag> }),
          centerColumn({
            title: t('providerBalance'),
            width: 170,
            render: (_: unknown, row: SMSProvider) => {
              const result = balances[row.code] || { balance: row.lastBalance, checkedAt: row.lastBalanceCheckedAt };
              if (!result) return <span>--</span>;
              if (result.error) return <Tooltip title={result.error}><Tag color="red">{t('checkFailed')}</Tag></Tooltip>;
              return (
                <Space direction="vertical" size={0}>
                  <strong>{formatMoney(result.balance, row.currencyCode)}</strong>
                  {result.checkedAt && <small>{formatDateTime(result.checkedAt)}</small>}
                </Space>
              );
            },
          }),
          centerColumn({ title: t('status'), dataIndex: 'status', render: (value: string) => <StatusTag status={value} /> }),
          centerColumn({ title: t('updatedAt'), dataIndex: 'updatedAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.updatedAt, b.updatedAt) }),
          centerColumn({
            title: t('actions'),
            render: (_: unknown, row: SMSProvider) => (
              <Space>
                <Tooltip title={t('checkProvider')}>
                  <Button
                    size="small"
                    shape="circle"
                    icon={<Radar size={15} />}
                    loading={Boolean(checkingProviders[row.code])}
                    onClick={() => void checkProviderBalance(row)}
                  />
                </Tooltip>
                <Tooltip title={t('edit')}>
                  <Button size="small" shape="circle" icon={<Pencil size={15} />} onClick={() => openEdit(row)} />
                </Tooltip>
              </Space>
            ),
          }),
        ]}
      />
      <Modal title={t('providerSettings')} open={open} footer={null} onCancel={() => setOpen(false)} width={640}>
        <Form
          form={form}
          layout="vertical"
          onFinish={submitProvider}
        >
          <Form.Item name="name" label={t('providerName')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="baseUrl" label={t('baseUrl')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="currencyCode" label={t('currencyCode')} rules={[{ required: true }]}>
            <Select showSearch options={currencyOptions.map((value) => ({ label: value, value }))} />
          </Form.Item>
          {editing?.code === '62-us' ? (
            <SMS62USCredentialFields editing={editing} />
          ) : (
            <Form.Item name="apiKey" label={t('apiKey')} tooltip={t('apiKeyHelp')}>
              <Input.Password placeholder={editing?.apiKeySet ? t('leaveBlankToKeep') : undefined} />
            </Form.Item>
          )}
          {editing?.requiresLoginCredential && editing.code === '68sms' && (
            <div className="sms68-credential-grid">
              <Form.Item name="sms68Token" label="Token" tooltip={t('sms68TokenHelp')}>
                <Input.Password placeholder={(editing.loginCredentialSet || editing.metadataTokenSet) ? t('leaveBlankToKeep') : t('sms68TokenPlaceholder')} />
              </Form.Item>
              <Form.Item name="sms68Cookie" label="Cookie" tooltip={t('sms68CookieHelp')}>
                <Input placeholder={(editing.loginCredentialSet || editing.metadataTokenSet) ? t('leaveBlankToKeep') : t('sms68CookiePlaceholder')} />
              </Form.Item>
              <Form.Item name="sms68Communication" label="Communication" tooltip={t('sms68CommunicationHelp')}>
                <Input placeholder={(editing.loginCredentialSet || editing.metadataTokenSet) ? t('leaveBlankToKeep') : t('sms68CommunicationPlaceholder')} />
              </Form.Item>
            </div>
          )}
          {editing?.requiresLoginCredential && editing.code !== '68sms' && editing.code !== '62-us' && (
            <Form.Item name="loginCredential" label={t('loginCredential')} tooltip={t('loginCredentialHelp')}>
              <Input.TextArea
                autoSize={{ minRows: 4, maxRows: 8 }}
                placeholder={(editing.loginCredentialSet || editing.metadataTokenSet) ? t('leaveBlankToKeep') : t('loginCredentialPlaceholder')}
              />
            </Form.Item>
          )}
          <Form.Item name="status" label={t('status')}>
            <Select options={['enabled', 'disabled'].map((item) => ({ label: t(item), value: item }))} />
          </Form.Item>
          <Button htmlType="submit" type="primary" shape="round" loading={mutation.isPending}>
            {t('save')}
          </Button>
        </Form>
      </Modal>
    </div>
  );
}

function ServicesPage() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<any | null>(null);
  const [keyword, setKeyword] = useState('');
  const [form] = Form.useForm();
  const qc = useQueryClient();
  const providerCode = Form.useWatch('providerCode', form) || 'smspool';
  const countryId = Form.useWatch('providerCountryId', form);
  const serviceId = Form.useWatch('providerServiceId', form);
  const validityType = Form.useWatch('validityType', form);
  const quoteUnsupported = providerCode === '68sms' && Boolean(countryId) && Boolean(serviceId);
  const query = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const providers = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const currencyByProvider = providerCurrencyMap(providers.data || []);
  const selectedProvider = (providers.data || []).find((item) => item.code === providerCode);
  const isLongLivedProvider = Boolean(selectedProvider?.manualCheck || selectedProvider?.providerKind === 'long_lived' || providerCode === '68sms');
  const countries = useQuery({
    queryKey: ['provider-countries', providerCode],
    queryFn: () => listProviderCountries(providerCode),
    enabled: open && Boolean(providerCode),
  });
  const services = useQuery({
    queryKey: ['provider-services', providerCode, countryId],
    queryFn: () => listProviderServices(providerCode, countryId),
    enabled: open && Boolean(providerCode) && Boolean(countryId),
  });
  const quote = useQuery({
    queryKey: ['provider-quote', providerCode, countryId, serviceId, validityType],
    queryFn: () => getProviderQuote(providerCode, { countryId, serviceId, poolId: validityType }),
    enabled: open && Boolean(providerCode) && Boolean(countryId) && Boolean(serviceId) && !quoteUnsupported,
  });
  const validityOptions = useQuery({
    queryKey: ['provider-validity-options', providerCode, countryId, serviceId],
    queryFn: () => getProviderValidityOptions(providerCode, { countryId, serviceId }),
    enabled: open && isLongLivedProvider && Boolean(countryId) && Boolean(serviceId),
  });
  const mutation = useMutation({
    mutationFn: (values: any) => {
      const selectedValidity = (validityOptions.data || []).find((item: ProviderValidityOption) => item.value === values.validityType);
      const metadata = isLongLivedProvider ? {
        ...(values.metadata || {}),
        validityType: values.validityType,
        endDay: values.validityType,
        validityLabel: selectedValidity ? validityLabel(selectedValidity, t) : undefined,
        validityStock: selectedValidity?.stock,
      } : values.metadata;
      const payload = { ...values, timeoutSeconds: isLongLivedProvider ? 0 : values.timeoutSeconds, metadata };
      delete payload.validityType;
      return editing ? updateServiceConfig(editing.id, payload) : createServiceConfig(payload);
    },
    onSuccess: () => {
      form.resetFields();
      setEditing(null);
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ['service-configs'] });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: deleteServiceConfig,
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['service-configs'] }),
    onError: (error: Error) => message.error(localizedError(error.message, t)),
  });
  const rows = filterRows(query.data || [], keyword);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ providerCode: 'smspool', timeoutSeconds: 1200, status: 'enabled', metadata: undefined, validityType: undefined });
    setOpen(true);
  };

  const openEdit = (record: any) => {
    setEditing(record);
    const provider = (providers.data || []).find((item) => item.code === record.providerCode);
    const longLived = Boolean(provider?.manualCheck || provider?.providerKind === 'long_lived' || record.providerCode === '68sms');
    form.setFieldsValue({ ...record, timeoutSeconds: longLived ? 0 : record.timeoutSeconds, validityType: serviceConfigValidityType(record.metadata) });
    setOpen(true);
  };

  const onCountryChange = (value: string) => {
    const selected = countries.data?.find((item) => countryValue(item) === value);
    form.setFieldsValue({
      providerCountryId: value,
      countryCode: selected?.shortName || '',
      countryName: selected?.name || '',
      providerServiceId: undefined,
      displayName: '',
      targetPlatform: undefined,
      maxPrice: undefined,
      metadata: undefined,
      validityType: undefined,
    });
  };

  const onServiceChange = (value: string) => {
    const selected = services.data?.find((item) => serviceValue(item) === value);
    const country = countries.data?.find((item) => countryValue(item) === countryId);
    form.setFieldsValue({
      providerServiceId: value,
      displayName: selected?.name || '',
      targetPlatform: buildServiceKey(providerCode, country?.shortName || country?.name || '', selected?.name || ''),
      maxPrice: undefined,
      metadata: undefined,
      validityType: undefined,
    });
  };

  useEffect(() => {
    if (quoteUnsupported) return;
    const quotePrice = quote.data?.price;
    const lowPrice = quotePrice?.lowPrice || quotePrice?.price;
    if (open && lowPrice && !editing) {
      form.setFieldValue('maxPrice', Number(lowPrice));
    }
  }, [editing, form, open, providerCode, quote.data, quoteUnsupported]);

  useEffect(() => {
    if (!open) return;
    if (isLongLivedProvider) {
      form.setFieldValue('timeoutSeconds', 0);
    } else if (form.getFieldValue('timeoutSeconds') == null || form.getFieldValue('timeoutSeconds') === 0) {
      form.setFieldValue('timeoutSeconds', 1200);
    }
  }, [form, isLongLivedProvider, open]);

  return (
    <div className="admin-page">
      <PageHead title={t('serviceConfigs')} searchValue={keyword} onSearchChange={setKeyword} onCreate={openCreate} onRefresh={() => query.refetch()} />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={rows}
        loading={query.isLoading}
        pagination={tablePagination(t)}
        columns={[
          centerColumn({ title: t('id'), dataIndex: 'id', width: 76 }),
          centerColumn({ title: t('serviceKey'), dataIndex: 'targetPlatform', width: 260, className: 'wide-key-column' }),
          centerColumn({ title: t('provider'), dataIndex: 'providerCode' }),
          centerColumn({ title: t('country'), render: (_: unknown, row: any) => row.countryName || row.countryCode }),
          centerColumn({ title: t('service'), dataIndex: 'displayName' }),
          centerColumn({ title: t('providerServiceId'), dataIndex: 'providerServiceId' }),
          centerColumn({ title: t('maxPrice'), render: (_: unknown, row: any) => formatMoney(row.maxPrice, currencyByProvider[row.providerCode]) }),
          centerColumn({ title: t('numberValidity'), render: (_: unknown, row: any) => serviceConfigValidityDisplay(row.metadata, t) }),
          centerColumn({ title: t('createdAt'), dataIndex: 'createdAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.createdAt, b.createdAt) }),
          centerColumn({ title: t('status'), dataIndex: 'status', render: (value: string) => <StatusTag status={value} /> }),
          centerColumn({
            title: t('actions'),
            width: 160,
            render: (_: unknown, row: any) => (
              <Space>
                <Tooltip title={t('edit')}>
                  <Button size="small" shape="circle" icon={<Pencil size={15} />} onClick={() => openEdit(row)} />
                </Tooltip>
                <Popconfirm title={t('confirmDelete')} onConfirm={() => deleteMutation.mutate(row.id)}>
                  <Tooltip title={t('delete')}>
                    <Button size="small" shape="circle" danger icon={<Trash2 size={15} />} />
                  </Tooltip>
                </Popconfirm>
              </Space>
            ),
          }),
        ]}
      />
      <Modal title={editing ? t('save') : t('create')} open={open} footer={null} onCancel={() => setOpen(false)} width={720}>
        <Alert className="form-help" type="info" showIcon message={t('serviceConfigHelp')} />
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => mutation.mutate(values)}
          initialValues={{ providerCode: 'smspool', timeoutSeconds: 1200, status: 'enabled' }}
        >
          <Form.Item name="providerCode" label={t('provider')} rules={[{ required: true }]}>
            <Select
              options={(providers.data || []).map((item) => ({ label: item.name, value: item.code }))}
              onChange={(value) => {
                const provider = (providers.data || []).find((item) => item.code === value);
                const nextLongLived = Boolean(provider?.manualCheck || provider?.providerKind === 'long_lived' || value === '68sms');
                form.setFieldsValue({
                  providerCode: value,
                  providerCountryId: undefined,
                  providerServiceId: undefined,
                  countryCode: '',
                  countryName: '',
                  displayName: '',
                  targetPlatform: undefined,
                  maxPrice: undefined,
                  metadata: undefined,
                  validityType: undefined,
                  timeoutSeconds: nextLongLived ? 0 : 1200,
                });
              }}
            />
          </Form.Item>
          <Form.Item name="providerCountryId" label={t('providerCountry')} rules={[{ required: true }]}>
            <Select
              showSearch
              loading={countries.isLoading}
              optionFilterProp="label"
              placeholder={t('selectProviderFirst')}
              notFoundContent={countries.isLoading ? <Space><Spin size="small" />{t('syncingProviderCountries')}</Space> : null}
              onChange={onCountryChange}
              options={(countries.data || []).map((item: ProviderCountry) => ({
                label: countryLabel(item),
                value: countryValue(item),
              }))}
            />
          </Form.Item>
          <Form.Item name="providerServiceId" label={t('providerService')} rules={[{ required: true }]}>
            <Select
              showSearch
              loading={services.isLoading}
              disabled={!countryId}
              optionFilterProp="label"
              placeholder={t('selectCountryFirst')}
              notFoundContent={services.isLoading ? <Space><Spin size="small" />{t('syncingProviderServices')}</Space> : null}
              onChange={onServiceChange}
              options={(services.data || []).map((item: ProviderService) => ({
                label: serviceLabel(item),
                value: serviceValue(item),
              }))}
            />
          </Form.Item>
          <Form.Item name="targetPlatform" label={t('platform')} rules={[{ required: true, pattern: /^\S+$/, message: 'no spaces' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="displayName" hidden><Input /></Form.Item>
          <QuoteSummary
            unsupported={quoteUnsupported}
            loading={quote.isFetching}
            error={quote.error as Error | null}
            price={quote.data?.price}
            stock={quote.data?.stock}
            currency={currencyByProvider[providerCode]}
          />
          {isLongLivedProvider && (
            <Form.Item name="validityType" label={t('numberValidity')} tooltip={t('numberValidityHelp')} rules={[{ required: true }]}>
              <Select
                loading={validityOptions.isFetching}
                disabled={!countryId || !serviceId}
                placeholder={serviceId ? t('numberValidityPlaceholder') : t('selectServiceFirst')}
                optionLabelProp="label"
                options={(validityOptions.data || []).map((item: ProviderValidityOption) => ({
                  label: validityLabel(item, t),
                  value: item.value,
                }))}
                optionRender={(option) => {
                  const item = (validityOptions.data || []).find((entry: ProviderValidityOption) => entry.value === option.value);
                  return item ? <ValidityOption option={item} /> : option.label;
                }}
              />
            </Form.Item>
          )}
          <Form.Item name="maxPrice" label={t('maxPrice')} tooltip={t('maxPriceHelp')} rules={[{ required: true }]}>
            <InputNumber min={0} precision={4} style={{ width: '100%' }} />
          </Form.Item>
          {!isLongLivedProvider && (
            <Form.Item name="timeoutSeconds" label={t('timeoutSeconds')} tooltip={t('timeoutHelp')}>
              <InputNumber min={60} style={{ width: '100%' }} />
            </Form.Item>
          )}
          <Form.Item name="status" label={t('status')}>
            <Select options={['enabled', 'disabled'].map((item) => ({ label: t(item), value: item }))} />
          </Form.Item>
          <Form.Item name="countryCode" hidden><Input /></Form.Item>
          <Form.Item name="countryName" hidden><Input /></Form.Item>
          <Button htmlType="submit" type="primary" shape="round" loading={mutation.isPending}>
            {t('save')}
          </Button>
        </Form>
      </Modal>
    </div>
  );
}

function BatchesPage() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [permanent, setPermanent] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [form] = Form.useForm();
  const qc = useQueryClient();
  const batches = useQuery({ queryKey: ['card-batches'], queryFn: listCardBatches });
  const services = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const mutation = useMutation({
    mutationFn: createCardBatch,
    onSuccess: () => {
      form.resetFields();
      setPermanent(false);
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ['card-batches'] });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: deleteCardBatch,
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['card-batches'] }),
    onError: (error: Error) => message.error(localizedError(error.message, t)),
  });
  const rows = filterRows(
    (batches.data || []).map((row: any) => ({
      ...row,
      serviceKey: serviceKeyById(services.data || [], row.serviceConfigId),
    })),
    keyword,
  );

  return (
    <div className="admin-page">
      <PageHead title={t('cardBatches')} searchValue={keyword} onSearchChange={setKeyword} onCreate={() => setOpen(true)} onRefresh={() => batches.refetch()} />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={rows}
        loading={batches.isLoading}
        pagination={tablePagination(t)}
        columns={[
          centerColumn({ title: t('id'), dataIndex: 'id', width: 76 }),
          centerColumn({ title: t('batchName'), dataIndex: 'name' }),
          centerColumn({ title: t('provider'), dataIndex: 'providerCode' }),
          centerColumn({ title: t('serviceKey'), width: 260, render: (_: unknown, row: any) => row.serviceKey }),
          centerColumn({ title: t('quantity'), dataIndex: 'quantity' }),
          centerColumn({ title: t('usesPerCode'), dataIndex: 'usesPerCode' }),
          centerColumn({ title: t('expiresAt'), dataIndex: 'expiresAt', render: (value: string) => value ? formatDateTime(value) : t('noExpiry'), sorter: (a: any, b: any) => dateSorter(a.expiresAt, b.expiresAt) }),
          centerColumn({ title: t('createdAt'), dataIndex: 'createdAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.createdAt, b.createdAt) }),
          centerColumn({ title: t('exportedAt'), dataIndex: 'exportedAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.exportedAt, b.exportedAt) }),
          centerColumn({
            title: t('actions'),
            render: (_: unknown, row: any) => (
              <Space>
                <Tooltip title={t('exportTxt')}>
                  <Button shape="circle" icon={<Download size={16} />} onClick={() => void downloadCardBatch(row.id)} />
                </Tooltip>
                <Popconfirm
                  title={t('confirmDelete')}
                  description={t('deleteBatchDanger')}
                  okButtonProps={{ danger: true }}
                  onConfirm={() => deleteMutation.mutate(row.id)}
                >
                  <Tooltip title={t('delete')}>
                    <Button shape="circle" danger icon={<Trash2 size={16} />} />
                  </Tooltip>
                </Popconfirm>
              </Space>
            ),
          }),
        ]}
      />
      <Modal title={t('create')} open={open} footer={null} onCancel={() => setOpen(false)}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{ usesPerCode: 1, quantity: 1 }}
          onFinish={(values) => {
            const expiresAt = permanent ? undefined : values.expiresAt?.toISOString?.();
            mutation.mutate({ ...values, expiresAt });
          }}
        >
          <Form.Item name="name" label={t('batchName')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="serviceConfigId" label={t('service')} rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={(services.data || []).map((item) => ({ label: item.targetPlatform, value: item.id }))} />
          </Form.Item>
          <Form.Item name="quantity" label={t('quantity')} rules={[{ required: true }]}><InputNumber min={1} max={10000} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="usesPerCode" label={t('usesPerCode')} rules={[{ required: true }]}><InputNumber min={1} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label={t('permanent')}>
            <Switch checked={permanent} onChange={setPermanent} />
          </Form.Item>
          {!permanent && <Form.Item name="expiresAt" label={t('expiresAt')}><DatePicker showTime style={{ width: '100%' }} /></Form.Item>}
          <Button htmlType="submit" type="primary" shape="round" loading={mutation.isPending}>{t('create')}</Button>
        </Form>
      </Modal>
    </div>
  );
}

function CardsPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [msg, contextHolder] = message.useMessage();
  const [keyword, setKeyword] = useState('');
  const query = useQuery({ queryKey: ['card-codes'], queryFn: listCardCodes });
  const mutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) => updateCardStatus(id, status),
    onSuccess: () => {
      msg.success(t('save'));
      void qc.invalidateQueries({ queryKey: ['card-codes'] });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: deleteCardCode,
    onSuccess: () => {
      msg.success(t('delete'));
      void qc.invalidateQueries({ queryKey: ['card-codes'] });
    },
    onError: (error: Error) => msg.error(localizedError(error.message, t)),
  });
  const rows = filterRows(query.data || [], keyword);
  return (
    <div className="admin-page">
      {contextHolder}
      <PageHead title={t('cardCodes')} searchValue={keyword} onSearchChange={setKeyword} onRefresh={() => query.refetch()} />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={rows}
        loading={query.isLoading}
        pagination={tablePagination(t)}
        columns={[
          centerColumn({ title: t('id'), dataIndex: 'id', width: 76 }),
          centerColumn({
            title: t('cardCode'),
            dataIndex: 'codeMask',
            width: 330,
            className: 'card-code-column',
            render: (value: string, row: any) => <RevealableCardCode id={row.id} masked={value} />,
          }),
          centerColumn({ title: t('serviceKey'), width: 260, render: (_: unknown, row: any) => row.serviceConfig?.targetPlatform || '-' }),
          centerColumn({ title: t('usedAndTotal'), render: (_: unknown, row: any) => `${row.remainingUses}/${row.totalUses}` }),
          centerColumn({ title: t('expiresAt'), dataIndex: 'expiresAt', render: (value: string) => value ? formatDateTime(value) : t('noExpiry'), sorter: (a: any, b: any) => dateSorter(a.expiresAt, b.expiresAt) }),
          centerColumn({ title: t('createdAt'), dataIndex: 'createdAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.createdAt, b.createdAt) }),
          centerColumn({ title: t('status'), dataIndex: 'status', render: (value: string) => <StatusTag status={value} /> }),
          centerColumn({
            title: t('actions'),
            render: (_: unknown, row: any) => (
              <Space>
                <Tooltip title={row.status === 'enabled' ? t('enabled') : t('disabled')}>
                  <Switch
                    size="small"
                    checked={row.status === 'enabled'}
                    loading={mutation.isPending}
                    onChange={(checked) => mutation.mutate({ id: row.id, status: checked ? 'enabled' : 'disabled' })}
                  />
                </Tooltip>
                <Popconfirm title={t('confirmDelete')} okButtonProps={{ danger: true }} onConfirm={() => deleteMutation.mutate(row.id)}>
                  <Tooltip title={t('delete')}>
                    <Button size="small" shape="circle" danger icon={<Trash2 size={15} />} />
                  </Tooltip>
                </Popconfirm>
              </Space>
            ),
          }),
        ]}
      />
    </div>
  );
}

function OrdersPage() {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState('');
  const query = useQuery({ queryKey: ['orders'], queryFn: listOrders, refetchInterval: 8000 });
  const providers = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const rows = filterRows(query.data || [], keyword);
  const currencyByProvider = providerCurrencyMap(providers.data || []);
  return (
    <div className="admin-page">
      <PageHead title={t('orders')} searchValue={keyword} onSearchChange={setKeyword} onRefresh={() => query.refetch()} />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={rows}
        loading={query.isLoading}
        pagination={tablePagination(t)}
        columns={[
          centerColumn({ title: t('id'), dataIndex: 'id', width: 76 }),
          centerColumn({ title: t('orderNo'), dataIndex: 'orderNo' }),
          centerColumn({ title: t('serviceKey'), width: 260, render: (_: unknown, row: any) => row.serviceConfig?.targetPlatform || '-' }),
          centerColumn({ title: t('phoneNumber'), dataIndex: 'phoneNumber' }),
          centerColumn({ title: t('verificationCode'), dataIndex: 'verificationCode' }),
          centerColumn({ title: t('supplierOrderId'), dataIndex: 'supplierOrderId' }),
          centerColumn({ title: t('cost'), render: (_: unknown, row: any) => formatMoney(row.cost, currencyByProvider[row.providerCode]) }),
          centerColumn({ title: t('status'), dataIndex: 'status', render: (value: string) => <StatusTag status={value} /> }),
          centerColumn({ title: t('createdAt'), dataIndex: 'createdAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.createdAt, b.createdAt) }),
          centerColumn({ title: t('updatedAt'), dataIndex: 'updatedAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.updatedAt, b.updatedAt) }),
        ]}
      />
    </div>
  );
}

function AuditPage() {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState('');
  const query = useQuery({ queryKey: ['audit-logs'], queryFn: listAuditLogs });
  const rows = filterRows(query.data || [], keyword);
  return (
    <div className="admin-page">
      <PageHead title={t('auditLogs')} searchValue={keyword} onSearchChange={setKeyword} onRefresh={() => query.refetch()} />
      <Table
        className="center-table"
        scroll={{ x: 'max-content' }}
        rowKey="id"
        dataSource={rows}
        loading={query.isLoading}
        pagination={tablePagination(t)}
        columns={[
          centerColumn({ title: t('id'), dataIndex: 'id', width: 76 }),
          centerColumn({
            title: t('auditAction'),
            dataIndex: 'action',
            width: 230,
            render: (value: string) => (
              <Space direction="vertical" size={2} className="audit-action-cell">
                <Tag color="blue">{auditActionLabel(value, t)}</Tag>
                <small>{auditActionDescription(value, t)}</small>
              </Space>
            ),
          }),
          centerColumn({
            title: t('resource'),
            width: 160,
            render: (_: unknown, row: AuditLog) => `${auditResourceLabel(row.resourceType, t)}${row.resourceId ? ` #${row.resourceId}` : ''}`,
          }),
          centerColumn({
            title: t('auditActor'),
            width: 140,
            render: (_: unknown, row: AuditLog) => `${auditResourceLabel(row.actorType, t)} #${row.actorId || '-'}`,
          }),
          centerColumn({
            title: t('auditDetails'),
            dataIndex: 'metadata',
            width: 320,
            render: (value: AuditLog['metadata']) => {
              const text = formatAuditMetadata(value);
              return (
                <Tooltip title={text}>
                  <span className="audit-details-cell">{truncateText(text, 96)}</span>
                </Tooltip>
              );
            },
          }),
          centerColumn({ title: 'IP', dataIndex: 'ip' }),
          centerColumn({
            title: t('userAgent'),
            dataIndex: 'userAgent',
            width: 260,
            render: (value: string) => (
              <Tooltip title={value || '-'}>
                <span className="audit-details-cell">{truncateText(value || '-', 72)}</span>
              </Tooltip>
            ),
          }),
          centerColumn({ title: t('createdAt'), dataIndex: 'createdAt', render: formatDateTime, sorter: (a: any, b: any) => dateSorter(a.createdAt, b.createdAt) }),
        ]}
      />
    </div>
  );
}

function PasswordModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [msg, contextHolder] = message.useMessage();
  const mutation = useMutation({
    mutationFn: (values: { oldPassword: string; newPassword: string }) => changePassword(values.oldPassword, values.newPassword),
    onSuccess: () => {
      msg.success(t('passwordChanged'));
      form.resetFields();
      onClose();
    },
    onError: (error: Error) => msg.error(error.message),
  });
  return (
    <Modal title={t('changePassword')} open={open} footer={null} onCancel={onClose}>
      {contextHolder}
      <Form
        form={form}
        layout="vertical"
        onFinish={(values) => {
          if (values.newPassword !== values.confirmPassword) {
            msg.error(t('passwordMismatch'));
            return;
          }
          mutation.mutate(values);
        }}
      >
        <Form.Item name="oldPassword" label={t('oldPassword')} rules={[{ required: true }]}>
          <Input.Password />
        </Form.Item>
        <Form.Item name="newPassword" label={t('newPassword')} rules={[{ required: true, min: 8 }]}>
          <Input.Password />
        </Form.Item>
        <Form.Item name="confirmPassword" label={t('confirmPassword')} rules={[{ required: true }]}>
          <Input.Password />
        </Form.Item>
        <Button htmlType="submit" type="primary" shape="round" loading={mutation.isPending}>{t('save')}</Button>
      </Form>
    </Modal>
  );
}

function QuoteSummary({
  unsupported,
  loading,
  error,
  price,
  stock,
  currency,
}: {
  unsupported?: boolean;
  loading: boolean;
  error: Error | null;
  price?: ProviderPrice;
  stock?: ProviderStock;
  currency?: string;
}) {
  const { t } = useTranslation();
  const hasPrice = Boolean(price?.lowPrice || price?.price || price?.highPrice);
  const hasStock = stock?.amount !== undefined;
  const hasSuccessRate = Boolean(price && price.successRate > 0);
  if (unsupported) {
    return <Alert className="form-help" type="info" showIcon message={t('providerQuoteUnsupported')} />;
  }
  if (loading) {
    return (
      <Space className="quote-row" wrap>
        <Tag color="processing">{t('loading')}</Tag>
      </Space>
    );
  }
  if (error) {
    return <Alert className="form-help" type="warning" showIcon message={localizedError(error.message, t)} />;
  }
  if (!hasPrice && !hasStock && !hasSuccessRate) return null;
  return (
    <Space className="quote-row" wrap>
      {hasStock && <Tag color="cyan">{t('stock')}: {stock?.amount}</Tag>}
      {hasPrice && <Tag color="green">{t('lowPrice')}: {formatMoney(price?.lowPrice || price?.price, currency)}</Tag>}
      {price?.highPrice && <Tag color="blue">{t('highPrice')}: {formatMoney(price.highPrice, currency)}</Tag>}
      {hasSuccessRate && <Tag color="gold">{t('successRate')}: {price?.successRate}%</Tag>}
    </Space>
  );
}

function SearchPanel({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const active = expanded || Boolean(value);
  return (
    <div className={`admin-filter-bar${active ? ' is-open' : ''}`}>
      {active && (
        <Input
          autoFocus
          allowClear
          value={value}
          placeholder={t('searchPlaceholder')}
          onBlur={() => {
            if (!value) setExpanded(false);
          }}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      <Tooltip title={t('search')}>
        <Button shape="circle" icon={<Search size={16} />} onClick={() => setExpanded(true)} />
      </Tooltip>
    </div>
  );
}

function PageHead({
  title,
  searchValue,
  onSearchChange,
  extraActions,
  onCreate,
  onRefresh,
}: {
  title: string;
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  extraActions?: ReactNode;
  onCreate?: () => void;
  onRefresh?: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="page-head">
      <h1>{title}</h1>
      <Space>
        {onSearchChange && <SearchPanel value={searchValue || ''} onChange={onSearchChange} />}
        {extraActions}
        {onRefresh && (
          <Tooltip title={t('refresh')}>
            <Button shape="circle" icon={<RefreshCw size={16} />} onClick={onRefresh} />
          </Tooltip>
        )}
        {onCreate && <Button type="primary" shape="round" onClick={onCreate}>{t('create')}</Button>}
      </Space>
    </div>
  );
}

function RevealableCardCode({ id, masked }: { id: number; masked: string }) {
  const [msg, contextHolder] = message.useMessage();
  const { t } = useTranslation();
  const [plain, setPlain] = useState('');
  const [visible, setVisible] = useState(false);
  const reveal = async () => {
    try {
      const data = await revealCardCode(id);
      setPlain(data.code);
      setVisible(true);
    } catch (error: any) {
      msg.error(error?.message || t('codeHiddenUnavailable'));
    }
  };
  const value = visible && plain ? plain : masked;
  return (
    <Space className="card-code-cell">
      {contextHolder}
      <span className="card-code-text">{value}</span>
      <Button
        size="small"
        type="text"
        shape="circle"
        icon={visible ? <Eye size={15} /> : <EyeOff size={15} />}
        onClick={() => (plain ? setVisible((next) => !next) : void reveal())}
      />
      <Button
        size="small"
        type="text"
        shape="circle"
        icon={<Copy size={15} />}
        onClick={async () => {
          try {
            const copyValue = plain || (await revealCardCode(id)).code;
            setPlain(copyValue);
            await navigator.clipboard.writeText(copyValue);
            msg.success(t('copied'));
          } catch (error: any) {
            msg.error(error?.message || t('codeHiddenUnavailable'));
          }
        }}
      />
    </Space>
  );
}

function SMS62USCredentialFields({ editing }: { editing: SMSProvider }) {
  const { t } = useTranslation();
  const form = Form.useFormInstance();
  const authMode = Form.useWatch('authMode', form) || 'account_password';
  const useOpenAPI = authMode === 'openapi_token';
  return (
    <div className="sms62us-credential-panel">
      <div className="sms62us-auth-row">
        <span>{t('sms62usAuthMode')}</span>
        <Button
          size="small"
          shape="round"
          type={useOpenAPI ? 'primary' : 'default'}
          onClick={() => form.setFieldValue('authMode', useOpenAPI ? 'account_password' : 'openapi_token')}
        >
          {t('sms62usUseOpenAPIToken')}
        </Button>
      </div>
      <Form.Item name="authMode" hidden><Input /></Form.Item>
      {useOpenAPI ? (
        <Form.Item name="apiKey" label={t('apiKey')} tooltip={t('sms62usOpenAPITokenHelp')}>
          <Input.Password placeholder={editing.apiKeySet ? t('leaveBlankToKeep') : t('sms62usOpenAPITokenPlaceholder')} />
        </Form.Item>
      ) : (
        <div className="sms62us-account-grid">
          <Form.Item name="account" label={t('sms62usAccount')} tooltip={t('sms62usAccountHelp')}>
            <Input placeholder={editing.loginCredentialSet ? t('leaveBlankToKeep') : t('sms62usAccountPlaceholder')} />
          </Form.Item>
          <Form.Item name="password" label={t('sms62usPassword')} tooltip={t('sms62usPasswordHelp')}>
            <Input.Password placeholder={editing.loginCredentialSet ? t('leaveBlankToKeep') : t('sms62usPasswordPlaceholder')} />
          </Form.Item>
        </div>
      )}
    </div>
  );
}

function announcementStatusOptions(t: (key: string, options?: any) => string) {
  return ['draft', 'active', 'archived'].map((value) => ({ label: announcementStatusText(value, t), value }));
}

function announcementNotifyOptions(t: (key: string, options?: any) => string) {
  return ['silent', 'modal'].map((value) => ({ label: announcementNotifyText(value, t), value }));
}

function announcementStatusText(status: string, t: (key: string, options?: any) => string) {
  const key = `announcementStatus_${status}`;
  const translated = t(key);
  return translated === key ? status : translated;
}

function announcementNotifyText(mode: string, t: (key: string, options?: any) => string) {
  const key = `announcementNotify_${mode}`;
  const translated = t(key);
  return translated === key ? mode : translated;
}

function announcementStatusColor(status: string) {
  if (status === 'active') return 'green';
  if (status === 'archived') return 'gold';
  return 'default';
}

function formatAnnouncementPeriod(row: Announcement, t: (key: string, options?: any) => string) {
  if (!row.startAt && !row.endAt) return t('permanent');
  return `${row.startAt ? formatDateTime(row.startAt) : t('now')} - ${row.endAt ? formatDateTime(row.endAt) : t('noExpiry')}`;
}

function toDateTimeInput(value?: string) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  const pad = (num: number) => String(num).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function fromDateTimeInput(value?: string) {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}
function buildSMS68LoginCredential(values: any) {
  const token = String(values.sms68Token || '').trim();
  const cookie = String(values.sms68Cookie || '').trim();
  const communication = String(values.sms68Communication || '').trim();
  const lines = [
    token ? `Token: ${token}` : '',
    cookie ? `Cookie: ${cookie}` : '',
    communication ? `Communication: ${communication}` : '',
  ].filter(Boolean);
  return lines.join('\n');
}
function ValidityOption({ option }: { option: ProviderValidityOption }) {
  const { t } = useTranslation();
  return (
    <div className="validity-option-row">
      <span>{validityLabel(option, t)}</span>
      <small>{t('stock')} <b>{option.stock}</b></small>
    </div>
  );
}
function StatusTag({ status }: { status: string }) {
  const { t } = useTranslation();
  return <Tag color={statusColor(status)}>{translatedStatus(status, t)}</Tag>;
}

function translatedStatus(status: string, t: (key: string, options?: any) => string) {
  const statusKey = `status_${status}`;
  const translated = t(statusKey);
  if (translated !== statusKey) return translated;
  const fallback = t(status);
  return fallback === status ? status : fallback;
}

function auditActionKey(action: string) {
  return action.replace(/[.-]/g, '_');
}

function auditActionLabel(action: string, t: (key: string, options?: any) => string) {
  const key = `audit_${auditActionKey(action)}`;
  const translated = t(key);
  return translated === key ? action : translated;
}

function auditActionDescription(action: string, t: (key: string, options?: any) => string) {
  const key = `auditDesc_${auditActionKey(action)}`;
  const translated = t(key);
  return translated === key ? '-' : translated;
}

function auditResourceLabel(resourceType: string | undefined, t: (key: string, options?: any) => string) {
  if (!resourceType) return '-';
  const key = `resource_${resourceType}`;
  const translated = t(key);
  return translated === key ? resourceType : translated;
}

function formatAuditMetadata(metadata?: Record<string, unknown> | string) {
  if (!metadata) return '-';
  let value: unknown = metadata;
  if (typeof metadata === 'string') {
    try {
      value = JSON.parse(metadata);
    } catch {
      return metadata || '-';
    }
  }
  if (!value || typeof value !== 'object') return String(value ?? '-');
  const entries = Object.entries(value as Record<string, unknown>);
  if (entries.length === 0) return '-';
  return entries.map(([key, entry]) => `${key}: ${formatAuditValue(entry)}`).join(' | ');
}

function formatAuditValue(value: unknown): string {
  if (value === undefined || value === null || value === '') return '-';
  if (value instanceof Date) return value.toISOString();
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function truncateText(value: string, maxLength: number) {
  if (value.length <= maxLength) return value;
  return `${value.slice(0, maxLength - 1)}...`;
}

function centerColumn(column: any) {
  return { align: 'center' as const, ...column };
}

function tablePagination(t: (key: string, options?: any) => string) {
  return {
    showSizeChanger: true,
    pageSizeOptions: [10, 20, 50, 100],
    showTotal: (total: number) => t('tableTotal', { total }),
  };
}

function dateSorter(left?: string | null, right?: string | null) {
  const leftTime = left ? new Date(left).getTime() : 0;
  const rightTime = right ? new Date(right).getTime() : 0;
  return leftTime - rightTime;
}

function serviceKeyById(services: any[], id: number) {
  return services.find((item) => item.id === id)?.targetPlatform || id || '-';
}

function countryValue(item: ProviderCountry) {
  return item.providerCountryId || String(item.id);
}

function serviceValue(item: ProviderService) {
  return item.providerServiceId || String(item.id);
}

function countryLabel(item: ProviderCountry) {
  return `${item.name}${item.shortName ? ` (${item.shortName})` : ''}${item.providerCountryId ? ` [${item.providerCountryId}]` : ''}`;
}

function serviceLabel(item: ProviderService) {
  const parts = [item.name];
  if (item.providerServiceId) parts.push(item.providerServiceId);
  return parts.join(' 璺?');
}

function serviceConfigValidityType(metadata: unknown) {
  if (!metadata || typeof metadata !== 'object') return undefined;
  const value = (metadata as { validityType?: unknown }).validityType;
  return typeof value === 'string' ? value : undefined;
}

function serviceConfigValidityDisplay(metadata: unknown, t: (key: string, options?: any) => string) {
  if (!metadata || typeof metadata !== 'object') return '-';
  const value = metadata as { validityLabel?: unknown; validityMinDays?: unknown; validityMaxDays?: unknown; validityType?: unknown };
  const min = Number(value.validityMinDays || 0);
  const max = Number(value.validityMaxDays || 0);
  if (min && max) return t('numberValidityDays', { min, max });
  if (typeof value.validityLabel === 'string' && value.validityLabel.trim()) return value.validityLabel;
  return typeof value.validityType === 'string' && value.validityType ? value.validityType : '-';
}

function validityLabel(option: ProviderValidityOption, t: (key: string, options?: any) => string) {
  const min = option.minDays || 0;
  const max = option.maxDays || 0;
  if (min && max) return t('numberValidityDays', { min, max });
  return option.label || option.value;
}
function providerCurrencyMap(providers: SMSProvider[]) {
  return providers.reduce<Record<string, string>>((result, provider) => {
    result[provider.code] = provider.currencyCode || 'USD';
    return result;
  }, {});
}

function formatMoney(value?: string | number | null, currency = 'USD') {
  if (value === undefined || value === null || value === '') return '--';
  const numeric = Number(value);
  const normalizedCurrency = currency || 'USD';
  if (!Number.isFinite(numeric)) return `${value} ${normalizedCurrency}`;
  return `${numeric.toFixed(4).replace(/\.?0+$/, '')} ${normalizedCurrency}`;
}

function filterRows<T>(rows: T[], keyword: string) {
  const normalized = keyword.trim().toLowerCase();
  if (!normalized) return rows;
  return rows.filter((row) => JSON.stringify(row).toLowerCase().includes(normalized));
}

function buildServiceKey(provider: string, country: string, service: string) {
  return [provider, country, service]
    .filter(Boolean)
    .map((part) => part.trim().toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9._-]+/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, ''))
    .join('-');
}
