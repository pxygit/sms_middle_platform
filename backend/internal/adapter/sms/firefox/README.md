# firefox adapter

Provider code: `firefox`

Base URL defaults to `http://www.firefox.fun`.

Credential format in provider settings:
- `ApiName:PassWord`, the adapter logs in with `act=login` and uses the returned token.
- Or a raw token, if you want to provide an already issued token directly.

Mapped API actions:
- `login`: obtain token.
- `myInfo`: account balance.
- `getItem`: country/service/price/stock metadata.
- `getPhone`: request a phone number.
- `getPhoneCode`: poll SMS code.
- `setRel`: release/cancel a number.

The adapter implements both `SMSProvider` and `MetadataProvider`, so the existing service configuration and order flow can use it without provider-specific business branches.
