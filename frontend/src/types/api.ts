export interface Announcement {
  id: number;
  title: string;
  content: string;
  status: 'draft' | 'active' | 'archived' | string;
  notifyMode: 'modal' | 'silent' | string;
  readCount: number;
  startAt?: string;
  endAt?: string;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
  publishedAt?: string;
  unread?: boolean;
}

export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface ServiceConfig {
  id: number;
  providerCode: string;
  targetPlatform: string;
  displayName: string;
  countryCode: string;
  countryName?: string;
  providerCountryId: string;
  providerServiceId: string;
  providerPoolId?: string;
  maxPrice: number;
  timeoutSeconds: number;
  metadata?: Record<string, unknown>;
  status: string;
}

export interface SMSProvider {
  id: number;
  code: string;
  name: string;
  baseUrl: string;
  currencyCode: string;
  status: string;
  apiKeySet: boolean;
  metadataTokenSet?: boolean;
  loginCredentialSet?: boolean;
  requiresLoginCredential?: boolean;
  providerKind?: string;
  manualCheck?: boolean;
  authMode?: string;
  lastBalance?: string;
  lastBalanceCheckedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CardVerifyResult {
  codeMask: string;
  remainingUses: number;
  expiresAt?: string;
  serviceConfig: ServiceConfig;
}

export interface ReceiveOrder {
  id: number;
  orderNo: string;
  providerCode: string;
  supplierOrderId?: string;
  phoneNumber?: string;
  phoneCountryCode?: string;
  phoneNationalNumber?: string;
  verificationCode?: string;
  smsContent?: string;
  cost: number;
  maxPrice: number;
  status: string;
  supplierStatus?: string;
  providerKind?: string;
  manualCheck?: boolean;
  messageUrl?: string;
  failureReason?: string;
  startedAt?: string;
  receivedAt?: string;
  cancelledAt?: string;
  expiredAt?: string;
  createdAt: string;
  updatedAt: string;
  serviceConfig?: ServiceConfig;
}

export interface AdminUser {
  id: number;
  username: string;
  role: string;
  status: string;
}

export interface LoginResult {
  token: string;
  admin: AdminUser;
}

export interface CardBatch {
  id: number;
  name: string;
  providerCode: string;
  serviceConfigId: number;
  quantity: number;
  usesPerCode: number;
  expiresAt?: string;
  exportedAt?: string;
  createdAt: string;
}

export interface CardCode {
  id: number;
  codeMask: string;
  providerCode: string;
  totalUses: number;
  remainingUses: number;
  expiresAt?: string;
  status: string;
  serviceConfig?: ServiceConfig;
}

export interface ProviderCountry {
  id: number;
  providerCode?: string;
  providerCountryId?: string;
  name: string;
  shortName: string;
  region: string;
  dialCode?: string;
}

export interface ProviderService {
  id: number;
  providerCode?: string;
  providerCountryId?: string;
  providerServiceId?: string;
  name: string;
  countryName?: string;
  status?: string;
  syncedAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface ProviderPrice {
  pool: number;
  highPrice: string;
  price: string;
  lowPrice?: string;
  successRate: number;
}

export interface ProviderStock {
  amount: number;
}

export interface ProviderValidityOption {
  value: string;
  label: string;
  minDays: number;
  maxDays: number;
  stock: number;
}

export interface ProviderQuote {
  price?: ProviderPrice;
  stock?: ProviderStock;
}

export interface ProviderBalance {
  balance: string;
  checkedAt?: string;
}

export interface AuditLog {
  id: number;
  actorType: string;
  actorId: number;
  action: string;
  resourceType?: string;
  resourceId?: string;
  ip?: string;
  userAgent?: string;
  metadata?: Record<string, unknown> | string;
  createdAt: string;
}

export interface DashboardRank {
  key: string;
  name: string;
  count: number;
}

export interface DashboardStats {
  totalCompletedOrders: number;
  todayCompletedOrders: number;
  activeOrders: number;
  todayOrders: number;
  todayVisits: number;
  totalVisits: number;
  availableCards: number;
  providerRanking: DashboardRank[];
  serviceRanking: DashboardRank[];
  statusSummary: DashboardRank[];
}
