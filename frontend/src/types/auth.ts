export interface AuthConfig {
  admin: {
    enabled: boolean;
  };
  oidc: {
    enabled: boolean;
    provider?: string;
  };
  token: {
    enabled: boolean;
  };
  passkey: {
    enabled: boolean;
  };
  server: {
    host: string;
    port: number;
    protocol?: string;
    root_url?: string;
    allowed_ips?: string[];
    max_body_size?: number;
    max_header_size?: number;
    read_buffer_size?: number;
    write_buffer_size?: number;
  };
  logging: {
    level: string;
    format: string;
  };
}

export interface AuthUser {
  username: string;
  email?: string;
  name?: string;
  auth_method?: 'admin' | 'passkey' | 'oidc' | 'token' | string;
}

export interface Passkey {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
}

export interface Account extends AuthUser {
  auth_method: string;
  totp_enabled: boolean;
  recovery_codes_remaining: number;
  passkeys: Passkey[];
}

export interface SensitiveAccountInput {
  current_password: string;
  second_factor?: string;
}

export interface TotpEnrollment {
  enrollment_id: string;
  secret: string;
  otpauth_uri: string;
  qr_code_data_url: string;
  qr_png_data_url?: string;
}

export interface PasskeyCeremony<T> {
  ceremony_id: string;
  publicKey?: T;
  public_key?: T;
  options?: { publicKey: T };
}

export type LoginResult = { status: 'authenticated' } | { status: 'mfa-required' };

export interface AuthState {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
}
