export function formatPhone(input: { phoneNumber?: string; phoneCountryCode?: string; phoneNationalNumber?: string }) {
  const countryCode = input.phoneCountryCode;
  const nationalNumber = input.phoneNationalNumber;
  if (countryCode && nationalNumber) {
    return { display: `+${countryCode} ${nationalNumber}`, segment: nationalNumber };
  }
  return { display: input.phoneNumber || '-', segment: input.phoneNumber || '' };
}
