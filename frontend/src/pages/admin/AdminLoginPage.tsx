import { Button, Form, Input, message } from 'antd';
import { LockKeyhole, UserRound } from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { login } from '../../api/admin';
import { PreferenceBar } from '../../components/PreferenceBar';
import { setLocalStorageItem } from '../../utils/storage';
import { localizedError } from '../../utils/errors';

export function AdminLoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation();
  const [msg, contextHolder] = message.useMessage();

  const mutation = useMutation({
    mutationFn: (values: { username: string; password: string }) => login(values.username, values.password),
    onSuccess: (data) => {
      setLocalStorageItem('adminToken', data.token);
      setLocalStorageItem('adminUser', JSON.stringify(data.admin));
      navigate(searchParams.get('redirect') || '/admin', { replace: true });
    },
    onError: (error: Error) => msg.error(localizedError(error.message, t)),
  });

  return (
    <main className="admin-login-shell">
      {contextHolder}
      <div className="login-card">
        <div className="login-head">
          <div>
            <small>{t('admin')}</small>
            <h1>{t('login')}</h1>
          </div>
          <PreferenceBar compact />
        </div>
        <Form layout="vertical" onFinish={(values) => mutation.mutate(values)}>
          <Form.Item name="username" label={t('username')} rules={[{ required: true }]}>
            <Input size="large" prefix={<UserRound size={17} />} />
          </Form.Item>
          <Form.Item name="password" label={t('password')} rules={[{ required: true }]}>
            <Input.Password size="large" prefix={<LockKeyhole size={17} />} />
          </Form.Item>
          <Button block size="large" type="primary" shape="round" htmlType="submit" loading={mutation.isPending}>
            {t('login')}
          </Button>
        </Form>
      </div>
    </main>
  );
}
