import { ConfigProvider, theme as antdTheme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import enUS from 'antd/locale/en_US';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { PreferencesProvider, usePreferences } from './preferences';
import { HomePage } from '../pages/public/HomePage';
import { AdminLoginPage } from '../pages/admin/AdminLoginPage';
import { AdminLayout } from '../pages/admin/AdminLayout';

function Shell() {
  const { theme } = usePreferences();
  const { i18n } = useTranslation();

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
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/admin/login" element={<AdminLoginPage />} />
          <Route path="/admin/*" element={<AdminLayout />} />
        </Routes>
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
