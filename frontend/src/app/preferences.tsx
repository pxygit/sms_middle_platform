import { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { getLocalStorageItem, setLocalStorageItem } from '../utils/storage';

type Theme = 'light' | 'dark';

interface Preferences {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

const PreferencesContext = createContext<Preferences | null>(null);

function initialTheme(): Theme {
  const stored = getLocalStorageItem('theme');
  if (stored === 'light' || stored === 'dark') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export function PreferencesProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(initialTheme);

  const setTheme = (next: Theme) => {
    setLocalStorageItem('theme', next);
    setThemeState(next);
  };

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  const value = useMemo(() => ({ theme, setTheme }), [theme]);
  return <PreferencesContext.Provider value={value}>{children}</PreferencesContext.Provider>;
}

export function usePreferences() {
  const value = useContext(PreferencesContext);
  if (!value) throw new Error('usePreferences must be used inside PreferencesProvider');
  return value;
}
