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
  status: string;
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
  name: string;
  shortName: string;
  region: string;
}

export interface ProviderService {
  id: number;
  name: string;
  favourite: number;
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

export interface ProviderBalance {
  balance: string;
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
