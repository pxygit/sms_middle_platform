import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Button,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Layout,
  Menu,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  message,
} from 'antd';
import { Sparkles } from 'lucide-react';
import { Link, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  createCardBatch,
  createServiceConfig,
  downloadCardBatch,
  listAuditLogs,
  listCardBatches,
  listCardCodes,
  listOrders,
  listProviders,
  listServiceConfigs,
  updateCardStatus,
} from '../../api/admin';
import { PreferenceBar } from '../../components/PreferenceBar';

const { Header, Sider, Content } = Layout;

export function AdminLayout() {
  const token = localStorage.getItem('adminToken');
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();

  if (!token) return <Navigate to="/admin/login" replace />;

  const logout = () => {
    localStorage.removeItem('adminToken');
    localStorage.removeItem('adminUser');
    navigate('/admin/login');
  };

  const selected = location.pathname.split('/')[2] || 'dashboard';

  return (
    <Layout className="admin-shell">
      <Sider width={232} className="admin-sider">
        <div className="admin-brand">
          <Sparkles size={20} />
          <span>{t('brand')}</span>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selected]}
          items={[
            { key: 'dashboard', label: <Link to="/admin">{t('dashboard')}</Link> },
            { key: 'services', label: <Link to="/admin/services">{t('serviceConfigs')}</Link> },
            { key: 'batches', label: <Link to="/admin/batches">{t('cardBatches')}</Link> },
            { key: 'cards', label: <Link to="/admin/cards">{t('cardCodes')}</Link> },
            { key: 'orders', label: <Link to="/admin/orders">{t('orders')}</Link> },
            { key: 'audit', label: <Link to="/admin/audit">{t('auditLogs')}</Link> },
          ]}
        />
      </Sider>
      <Layout>
        <Header className="admin-header">
          <PreferenceBar compact />
          <Button shape="round" onClick={logout}>
            {t('logout')}
          </Button>
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
    </Layout>
  );
}

function Dashboard() {
  const { t } = useTranslation();
  const providers = useQuery({ queryKey: ['providers'], queryFn: listProviders });
  const services = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const orders = useQuery({ queryKey: ['orders'], queryFn: listOrders });
  return (
    <div className="admin-page">
      <h1>{t('dashboard')}</h1>
      <div className="admin-stat-grid">
        <Stat title={t('providers')} value={providers.data?.length || 0} />
        <Stat title={t('serviceConfigs')} value={services.data?.length || 0} />
        <Stat title={t('orders')} value={orders.data?.length || 0} />
      </div>
    </div>
  );
}

function Stat({ title, value }: { title: string; value: number }) {
  return (
    <div className="stat-card">
      <span>{title}</span>
      <strong>{value}</strong>
    </div>
  );
}

function ServicesPage() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const qc = useQueryClient();
  const query = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const mutation = useMutation({
    mutationFn: createServiceConfig,
    onSuccess: () => {
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ['service-configs'] });
    },
  });

  return (
    <div className="admin-page">
      <PageHead title={t('serviceConfigs')} onCreate={() => setOpen(true)} />
      <Table
        rowKey="id"
        dataSource={query.data || []}
        loading={query.isLoading}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: t('service'), dataIndex: 'displayName' },
          { title: 'Provider', dataIndex: 'providerCode' },
          { title: 'Platform', dataIndex: 'targetPlatform' },
          { title: t('country'), dataIndex: 'countryCode' },
          { title: 'Max Price', dataIndex: 'maxPrice' },
          { title: t('status'), dataIndex: 'status', render: (v) => <Tag>{v}</Tag> },
        ]}
      />
      <Modal title={t('create')} open={open} footer={null} onCancel={() => setOpen(false)}>
        <Form layout="vertical" onFinish={(values) => mutation.mutate(values)} initialValues={{ providerCode: 'smspool', timeoutSeconds: 1200, status: 'enabled' }}>
          <Form.Item name="providerCode" label="Provider" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="displayName" label={t('service')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="targetPlatform" label="Platform" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="countryCode" label={t('country')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="countryName" label="Country Name"><Input /></Form.Item>
          <Form.Item name="providerCountryId" label="Provider Country ID" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="providerServiceId" label="Provider Service ID" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="providerPoolId" label="Provider Pool ID"><Input /></Form.Item>
          <Form.Item name="maxPrice" label="Max Price"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="timeoutSeconds" label="Timeout Seconds"><InputNumber min={60} style={{ width: '100%' }} /></Form.Item>
          <Button htmlType="submit" type="primary" shape="round" loading={mutation.isPending}>{t('save')}</Button>
        </Form>
      </Modal>
    </div>
  );
}

function BatchesPage() {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const qc = useQueryClient();
  const batches = useQuery({ queryKey: ['card-batches'], queryFn: listCardBatches });
  const services = useQuery({ queryKey: ['service-configs'], queryFn: listServiceConfigs });
  const mutation = useMutation({
    mutationFn: createCardBatch,
    onSuccess: () => {
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ['card-batches'] });
    },
  });

  return (
    <div className="admin-page">
      <PageHead title={t('cardBatches')} onCreate={() => setOpen(true)} />
      <Table
        rowKey="id"
        dataSource={batches.data || []}
        loading={batches.isLoading}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: 'Name', dataIndex: 'name' },
          { title: 'Provider', dataIndex: 'providerCode' },
          { title: 'Quantity', dataIndex: 'quantity' },
          { title: 'Uses', dataIndex: 'usesPerCode' },
          {
            title: t('exportTxt'),
            render: (_, row) => (
              <Button shape="round" onClick={() => void downloadCardBatch(row.id)}>
                {t('exportTxt')}
              </Button>
            ),
          },
        ]}
      />
      <Modal title={t('create')} open={open} footer={null} onCancel={() => setOpen(false)}>
        <Form layout="vertical" onFinish={(values) => {
          const expiresAt = values.expiresAt?.toISOString?.();
          mutation.mutate({ ...values, expiresAt });
        }}>
          <Form.Item name="name" label="Name" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="serviceConfigId" label={t('service')} rules={[{ required: true }]}>
            <Select options={(services.data || []).map((item) => ({ label: item.displayName, value: item.id }))} />
          </Form.Item>
          <Form.Item name="quantity" label="Quantity" rules={[{ required: true }]}><InputNumber min={1} max={10000} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="usesPerCode" label="Uses" rules={[{ required: true }]}><InputNumber min={1} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="expiresAt" label="Expires At"><DatePicker showTime style={{ width: '100%' }} /></Form.Item>
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
  const query = useQuery({ queryKey: ['card-codes'], queryFn: listCardCodes });
  const mutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: string }) => updateCardStatus(id, status),
    onSuccess: () => {
      msg.success(t('save'));
      void qc.invalidateQueries({ queryKey: ['card-codes'] });
    },
  });
  return (
    <div className="admin-page">
      {contextHolder}
      <PageHead title={t('cardCodes')} />
      <Table
        rowKey="id"
        dataSource={query.data || []}
        loading={query.isLoading}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: t('cardCode'), dataIndex: 'codeMask' },
          { title: t('remainingUses'), dataIndex: 'remainingUses' },
          { title: 'Total', dataIndex: 'totalUses' },
          { title: t('status'), dataIndex: 'status', render: (v) => <Tag>{v}</Tag> },
          {
            title: t('status'),
            render: (_, row) => (
              <Select
                value={row.status}
                style={{ width: 120 }}
                onChange={(status) => mutation.mutate({ id: row.id, status })}
                options={['enabled', 'disabled', 'voided'].map((item) => ({ label: t(item), value: item }))}
              />
            ),
          },
        ]}
      />
    </div>
  );
}

function OrdersPage() {
  const { t } = useTranslation();
  const query = useQuery({ queryKey: ['orders'], queryFn: listOrders, refetchInterval: 8000 });
  return (
    <div className="admin-page">
      <PageHead title={t('orders')} />
      <Table
        rowKey="id"
        dataSource={query.data || []}
        loading={query.isLoading}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: 'Order No', dataIndex: 'orderNo' },
          { title: t('phoneNumber'), dataIndex: 'phoneNumber' },
          { title: t('verificationCode'), dataIndex: 'verificationCode' },
          { title: 'Provider', dataIndex: 'providerCode' },
          { title: 'Cost', dataIndex: 'cost' },
          { title: t('status'), dataIndex: 'status', render: (v) => <Tag>{v}</Tag> },
        ]}
      />
    </div>
  );
}

function AuditPage() {
  const { t } = useTranslation();
  const query = useQuery({ queryKey: ['audit-logs'], queryFn: listAuditLogs });
  return (
    <div className="admin-page">
      <PageHead title={t('auditLogs')} />
      <Table
        rowKey="id"
        dataSource={query.data || []}
        loading={query.isLoading}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 70 },
          { title: 'Action', dataIndex: 'action' },
          { title: 'Resource', dataIndex: 'resourceType' },
          { title: 'IP', dataIndex: 'ip' },
          { title: 'Created', dataIndex: 'createdAt' },
        ]}
      />
    </div>
  );
}

function PageHead({ title, onCreate }: { title: string; onCreate?: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="page-head">
      <h1>{title}</h1>
      <Space>{onCreate && <Button type="primary" shape="round" onClick={onCreate}>{t('create')}</Button>}</Space>
    </div>
  );
}
