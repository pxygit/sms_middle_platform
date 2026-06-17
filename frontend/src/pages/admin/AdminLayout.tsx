import { useEffect, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Button,
  DatePicker,
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
  createCardBatch,
  createServiceConfig,
  deleteServiceConfig,
  deleteCardBatch,
  deleteCardCode,
  downloadCardBatch,
  getDashboardStats,
  getProviderBalance,
  getProviderQuote,
  listAuditLogs,
  listCardBatches,
  listCardCodes,
  listOrders,
  listProviderCountries,
  listProviderServices,
  listProviders,
  listServiceConfigs,
  revealCardCode,
  updateProvider,
  updateServiceConfig,
  updateCardStatus,
} from '../../api/admin';
import { PreferenceBar } from '../../components/PreferenceBar';
import { formatDateTime } from '../../utils/format';
import type { DashboardRank, ProviderCountry, ProviderPrice, ProviderService, ProviderStock, SMSProvider } from '../../types/api';
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
    <Layout className="admin-shell">
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
            { key: 'providers', icon: <Database size={18} />, label: <Link to="/admin/providers">{t('providers')}</Link> },
            { key: 'services', icon: <Settings2 size={18} />, label: <Link to="/admin/services">{t('serviceConfigs')}</Link> },
            { key: 'batches', icon: <Boxes size={18} />, label: <Link to="/admin/batches">{t('cardBatches')}</Link> },
            { key: 'cards', icon: <KeyRound size={18} />, label: <Link to="/admin/cards">{t('cardCodes')}</Link> },
            { key: 'orders', icon: <ClipboardList size={18} />, label: <Link to="/admin/orders">{t('orders')}</Link> },
            { key: 'audit', icon: <ScrollText size={18} />, label: <Link to="/admin/audit">{t('auditLogs')}</Link> },
          ]}
        />
      </Sider>
      <Layout>
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
              <b>{translateName ? t(row.name) : row.name}</b>
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
    form.setFieldsValue({ ...record, apiKey: '' });
    setOpen(true);
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
        <Alert className="form-help" type="info" showIcon message={t('providerSettingsHelp')} />
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => editing && mutation.mutate({ code: editing.code, values })}
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
          <Form.Item name="apiKey" label={t('apiKey')} tooltip={t('apiKeyHelp')}>
            <Input.Password placeholder={editing?.apiKeySet ? t('leaveBlankToKeep') : undefined} />
          </Form.Item>
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
  const query = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const providers = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const currencyByProvider = providerCurrencyMap(providers.data || []);
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
    queryKey: ['provider-quote', providerCode, countryId, serviceId],
    queryFn: () => getProviderQuote(providerCode, { countryId, serviceId }),
    enabled: open && Boolean(providerCode) && Boolean(countryId) && Boolean(serviceId),
  });
  const mutation = useMutation({
    mutationFn: (values: any) => editing ? updateServiceConfig(editing.id, values) : createServiceConfig(values),
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
  });
  const rows = filterRows(query.data || [], keyword);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ providerCode: 'smspool', timeoutSeconds: 1200, status: 'enabled' });
    setOpen(true);
  };

  const openEdit = (record: any) => {
    setEditing(record);
    form.setFieldsValue(record);
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
    });
  };

  useEffect(() => {
    const quotePrice = quote.data?.price;
    const lowPrice = quotePrice?.lowPrice || quotePrice?.price;
    if (open && lowPrice && !editing) {
      form.setFieldValue('maxPrice', Number(lowPrice));
    }
  }, [editing, form, open, quote.data]);

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
                form.setFieldsValue({
                  providerCode: value,
                  providerCountryId: undefined,
                  providerServiceId: undefined,
                  countryCode: '',
                  countryName: '',
                  displayName: '',
                  targetPlatform: undefined,
                  maxPrice: undefined,
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
            loading={quote.isFetching}
            error={quote.error as Error | null}
            price={quote.data?.price}
            stock={quote.data?.stock}
            currency={currencyByProvider[providerCode]}
          />
          <Form.Item name="maxPrice" label={t('maxPrice')} tooltip={t('maxPriceHelp')} rules={[{ required: true }]}>
            <InputNumber min={0} precision={4} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="timeoutSeconds" label={t('timeoutSeconds')} tooltip={t('timeoutHelp')}>
            <InputNumber min={60} style={{ width: '100%' }} />
          </Form.Item>
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
          centerColumn({ title: t('actions'), dataIndex: 'action' }),
          centerColumn({ title: t('resource'), dataIndex: 'resourceType' }),
          centerColumn({ title: 'IP', dataIndex: 'ip' }),
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
  loading,
  error,
  price,
  stock,
  currency,
}: {
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

function StatusTag({ status }: { status: string }) {
  const { t } = useTranslation();
  return <Tag color={statusColor(status)}>{t(status)}</Tag>;
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
  return parts.join(' · ');
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
