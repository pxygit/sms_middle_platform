type Translator = (key: string) => string;

const messageMappings: Array<[RegExp, string]> = [
  [/invalid username or password/i, 'error_INVALID_CREDENTIALS'],
  [/admin account is disabled/i, 'error_ADMIN_DISABLED'],
  [/new password must be at least 8 characters/i, 'error_PASSWORD_TOO_SHORT'],
  [/admin account not found/i, 'error_ADMIN_NOT_FOUND'],
  [/old password is incorrect/i, 'error_OLD_PASSWORD_INCORRECT'],
  [/oldpassword and newpassword are required/i, 'error_PASSWORD_REQUIRED'],
  [/quantity must be between 1 and 10000/i, 'error_QUANTITY_RANGE'],
  [/uses per code must be greater than 0/i, 'error_USES_PER_CODE'],
  [/invalid card status/i, 'error_INVALID_CARD_STATUS'],
  [/plain card code is unavailable/i, 'error_CARD_CODE_UNAVAILABLE'],
  [/card code not found/i, 'error_CARD_NOT_FOUND'],
  [/card code is not enabled/i, 'error_CARD_DISABLED'],
  [/card code has expired/i, 'error_CARD_EXPIRED'],
  [/card code has no remaining uses/i, 'error_CARD_NO_USES'],
  [/service is disabled/i, 'error_SERVICE_DISABLED'],
  [/invalid service config metadata/i, 'error_INVALID_SERVICE_METADATA'],
  [/simtype must be 1 or 2|unsupported simtype/i, 'error_INVALID_SIM_TYPE'],
  [/order not found/i, 'error_ORDER_NOT_FOUND'],
  [/order already received sms/i, 'error_ORDER_ALREADY_RECEIVED'],
  [/order cannot be cancelled in current status/i, 'error_ORDER_STATUS'],
  [/manual check orders cannot be cancelled/i, 'error_MANUAL_ORDER_CANCEL'],
  [/cancel is allowed after two minutes|wait two minutes/i, 'error_CANCEL_WAIT'],
  [/cannot be cancelled/i, 'error_CANNOT_CANCEL'],
  [/provider does not support validity options/i, 'error_VALIDITY_UNSUPPORTED'],
  [/country is required when syncing provider services/i, 'error_COUNTRY_REQUIRED'],
  [/provider does not support metadata queries/i, 'error_METADATA_UNSUPPORTED'],
  [/announcement title is required/i, 'error_ANNOUNCEMENT_TITLE_REQUIRED'],
  [/announcement content is required/i, 'error_ANNOUNCEMENT_CONTENT_REQUIRED'],
  [/invalid announcement status/i, 'error_INVALID_ANNOUNCEMENT_STATUS'],
  [/invalid announcement notify mode/i, 'error_INVALID_NOTIFY_MODE'],
  [/announcement end time must be after start time/i, 'error_ANNOUNCEMENT_TIME'],
  [/invalid reader payload|reader id is required/i, 'error_INVALID_REQUEST'],
  [/invalid id|status is required/i, 'error_INVALID_REQUEST'],
  [/auth_error|unauthorized|api key|login credential|invalid token/i, 'error_AUTH_ERROR'],
  [/balance/i, 'error_BALANCE_ERROR'],
  [/out of stock|stock is empty|stock is insufficient/i, 'error_OUT_OF_STOCK'],
  [/price not found|price exceeds|max price/i, 'error_PRICE_NOT_FOUND'],
  [/too many requests|rate limit|limited to once/i, 'error_RATE_LIMITED'],
  [/network error|failed to fetch|timeout|request failed with status code/i, 'error_NETWORK'],
];

export function localizedError(message: string, t: Translator) {
  const raw = String(message || '').trim();
  const codeMatch = raw.match(/^([A-Z][A-Z0-9_]*)(?::|$)/);
  const candidates = [
    codeMatch ? `error_${codeMatch[1]}` : '',
    ...messageMappings.filter(([pattern]) => pattern.test(raw)).map(([, key]) => key),
  ].filter(Boolean);

  for (const key of candidates) {
    const translated = t(key);
    if (translated !== key) return translated;
  }
  return t('error_REQUEST_FAILED');
}
