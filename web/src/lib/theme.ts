export type Theme = 'system' | 'light' | 'dark';

export function savedTheme(): Theme {
  try {
    const value = localStorage.getItem('orbit-theme');
    return value === 'light' || value === 'dark' ? value : 'system';
  } catch {
    return 'system';
  }
}

export function applyTheme(theme: Theme) {
  if (theme === 'system') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
  try {
    if (theme === 'system') localStorage.removeItem('orbit-theme');
    else localStorage.setItem('orbit-theme', theme);
  } catch {
    // Blocked storage: the choice lasts until reload.
  }
}
