// Response shapes from docs/API.md.
export interface Me {
  id: string;
  email: string | null;
  display_name: string | null;
  identities: string[];
  limits: { max_devices: number };
}
export interface Methods {
  github: boolean;
  google: boolean;
  email: boolean;
}
export interface Session {
  id: string;
  client: 'web' | 'ios' | 'desktop';
  created_at: string;
  last_used_at: string;
  current: boolean;
}
export interface Device {
  id: string;
  name: string;
  model: string | null;
  ios_version: string | null;
  apple_team_id: string | null;
  apple_label: string | null;
  app_ids_used_7d: number | null;
  vpn_ok: boolean | null;
  pairing_ok: boolean | null;
  last_seen_at: string | null;
}
export interface App {
  bundle_id: string;
  name: string;
  version: string | null;
  expires_at: string | null;
  source_url: string | null;
}
export interface RefreshEvent {
  id: string;
  device_id: string;
  device_name: string;
  kind: 'refresh' | 'install' | 'remove' | 'repair';
  trigger: string | null;
  result: 'success' | 'failure';
  error_code: string | null;
  occurred_at: string;
}
export interface Day {
  date: string;
  success: number;
  failure: number;
}
export interface Source {
  id: string;
  url: string;
  name: string | null;
}
export interface SourcePreview {
  url: string;
  name: string;
  app_count: number;
  apps: string[];
}
