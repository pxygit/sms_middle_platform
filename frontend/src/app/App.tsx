import { ConfigProvider, Spin, theme as antdTheme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import enUS from 'antd/locale/en_US';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { Suspense, lazy, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { PreferencesProvider, usePreferences } from './preferences';
import { HomePage } from '../pages/public/HomePage';

const AdminLoginPage = lazy(() => import('../pages/admin/AdminLoginPage').then((module) => ({ default: module.AdminLoginPage })));
const AdminLayout = lazy(() => import('../pages/admin/AdminLayout').then((module) => ({ default: module.AdminLayout })));

function RouteFallback() {
  return (
    <div className="route-fallback">
      <Spin size="large" />
    </div>
  );
}

function Shell() {
  const { theme } = usePreferences();
  const { i18n, t } = useTranslation();

  useEffect(() => {
    document.title = t('documentTitle');
  }, [i18n.language, t]);

  return (
    <ConfigProvider
      locale={i18n.language === 'en-US' ? enUS : zhCN}
      theme={{
        algorithm: theme === 'dark' ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          borderRadius: 14,
          colorPrimary: '#14b8a6',
          fontFamily: 'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
      }}
    >
      <BrowserRouter>
        <Suspense fallback={<RouteFallback />}>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/admin/login" element={<AdminLoginPage />} />
            <Route path="/admin/*" element={<AdminLayout />} />
          </Routes>
        </Suspense>
      </BrowserRouter>
    </ConfigProvider>
  );
}

export function App() {
  return (
    <PreferencesProvider>
      <Shell />
    </PreferencesProvider>
  );
}
