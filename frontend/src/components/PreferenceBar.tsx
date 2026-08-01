import { Button, Segmented, Space } from 'antd';
import { Languages, Moon, Sun } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePreferences } from '../app/preferences';
import { setLocalStorageItem } from '../utils/storage';

export function PreferenceBar({ compact = false }: { compact?: boolean }) {
  const { theme, setTheme } = usePreferences();
  const { i18n, t } = useTranslation();

  const changeLanguage = (language: string) => {
    setLocalStorageItem('language', language);
    void i18n.changeLanguage(language);
  };

  return (
    <Space size={compact ? 6 : 10} wrap>
      <Segmented
        size={compact ? 'small' : 'middle'}
        value={theme}
        onChange={(value) => setTheme(value as 'light' | 'dark')}
        options={[
          { label: <Sun size={16} aria-label={t('light')} />, value: 'light' },
          { label: <Moon size={16} aria-label={t('dark')} />, value: 'dark' },
        ]}
      />
      <Button
        shape="round"
        size={compact ? 'small' : 'middle'}
        icon={<Languages size={16} />}
        onClick={() => changeLanguage(i18n.language === 'en-US' ? 'zh-CN' : 'en-US')}
      >
        {i18n.language === 'en-US' ? '中文' : 'EN'}
      </Button>
    </Space>
  );
}
