import { useState } from 'react';
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
  getProviderPrice,
  getProviderStock,
  listAuditLogs,
  listCardBatches,
  listCardCodes,
  listOrders,
  listProviderCountries,
  listProviderServices,
  listProviders,
  listServiceConfigs,
  revealCardCode,
  updateServiceConfig,
  updateCardStatus,
} from '../../api/admin';
import { PreferenceBar } from '../../components/PreferenceBar';
import { formatDateTime } from '../../utils/format';
import type { DashboardRank, ProviderCountry, ProviderService } from '../../types/api';
import { statusColor } from '../../utils/status';

const { Header, Sider, Content } = Layout;

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
  const stats = useQuery({ queryKey: ['dashboard'], queryFn: getDashboardStats, refetchInterval: 30000 });
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
      <div className="dashboard-grid">
        <RankPanel title={t('providerRank')} rows={data?.providerRanking || []} loading={stats.isLoading} />
        <RankPanel title={t('serviceRank')} rows={data?.serviceRanking || []} loading={stats.isLoading} />
        <RankPanel title={t('statusOverview')} rows={data?.statusSummary || []} loading={stats.isLoading} translateName />
      </div>
    </div>
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
  const poolId = Form.useWatch('providerPoolId', form);
  const query = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const providers = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const countries = useQuery({
    queryKey: ['provider-countries', providerCode],
    queryFn: () => listProviderCountries(providerCode),
    enabled: open && providerCode === 'smspool',
  });
  const services = useQuery({
    queryKey: ['provider-services', providerCode, countryId],
    queryFn: () => listProviderServices(providerCode, countryId),
    enabled: open && providerCode === 'smspool' && Boolean(countryId),
  });
  const quote = useMutation({
    mutationFn: async () => ({
      price: await getProviderPrice(providerCode, { countryId, serviceId, poolId }),
      stock: await getProviderStock(providerCode, { countryId, serviceId, poolId }),
    }),
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
    const selected = countries.data?.find((item) => String(item.id) === value);
    form.setFieldsValue({
      providerCountryId: value,
      countryCode: selected?.shortName || '',
      countryName: selected?.name || '',
      providerServiceId: undefined,
      targetPlatform: undefined,
    });
  };

  const onServiceChange = (value: string) => {
    const selected = services.data?.find((item) => String(item.id) === value);
    const country = countries.data?.find((item) => String(item.id) === countryId);
    form.setFieldsValue({
      providerServiceId: value,
      displayName: selected?.name || '',
      targetPlatform: buildServiceKey(providerCode, country?.shortName || country?.name || '', selected?.name || ''),
    });
  };

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
          centerColumn({ title: t('maxPrice'), dataIndex: 'maxPrice' }),
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
            <Select options={(providers.data || []).map((item) => ({ label: item.name, value: item.code }))} />
          </Form.Item>
          <Form.Item name="providerCountryId" label={t('providerCountry')} rules={[{ required: true }]}>
            <Select
              showSearch
              loading={countries.isLoading}
              optionFilterProp="label"
              placeholder={t('selectProviderFirst')}
              onChange={onCountryChange}
              options={(countries.data || []).map((item: ProviderCountry) => ({
                label: `${item.name} (${item.shortName})`,
                value: String(item.id),
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
              onChange={onServiceChange}
              options={(services.data || []).map((item: ProviderService) => ({
                label: item.name,
                value: String(item.id),
              }))}
            />
          </Form.Item>
          <Form.Item name="targetPlatform" label={t('platform')} rules={[{ required: true, pattern: /^\S+$/, message: 'no spaces' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="displayName" hidden><Input /></Form.Item>
          <Form.Item name="providerPoolId" label={t('providerPoolId')} tooltip={t('poolHelp')}>
            <Input />
          </Form.Item>
          <Space className="quote-row" wrap>
            <Button
              shape="round"
              icon={<Database size={16} />}
              onClick={() => quote.mutate()}
              loading={quote.isPending}
              disabled={!countryId || !serviceId}
            >
              {t('refreshQuote')}
            </Button>
            {quote.data?.stock && <Tag color="cyan">{t('stock')}: {quote.data.stock.amount}</Tag>}
            {quote.data?.price && (
              <Tag color="green">
                {t('lowPrice')}: {quote.data.price.lowPrice || quote.data.price.price} / {t('highPrice')}: {quote.data.price.highPrice} / {t('successRate')}: {quote.data.price.successRate}%
              </Tag>
            )}
          </Space>
          {quote.error && <Alert className="form-help" type="warning" showIcon message={(quote.error as Error).message} />}
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
  const rows = filterRows(query.data || [], keyword);
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
          centerColumn({ title: t('cost'), dataIndex: 'cost' }),
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
  onCreate,
  onRefresh,
}: {
  title: string;
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  onCreate?: () => void;
  onRefresh?: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="page-head">
      <h1>{title}</h1>
      <Space>
        {onSearchChange && <SearchPanel value={searchValue || ''} onChange={onSearchChange} />}
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
