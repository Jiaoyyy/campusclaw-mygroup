# Design

- `POST /api/login` validates the existing provisioned account and returns `access_token`, `token_type: Bearer`, and `expires_at` without setting a Cookie. The token is a 256-bit random value; the database stores only its keyed hash.
- Protected APIs require `Authorization: Bearer <token>`. Cookie-only requests and malformed or unknown Bearer tokens return 401.
- The browser stores the token in tab-scoped `sessionStorage`, sends it only in an Authorization header, and uses authenticated `fetch` for file downloads. No token is added to URLs or logs.
- `GET /api/me` proves the token resolves to the expected account and class. Logout requires the existing same-origin and CSRF check, then deletes the session record. Expiration and role/class lookup remain server-controlled.
- Demo evidence should show the logged-in CampusClaw page and one successful protected API request in Network; the token itself should be hidden or redacted in any submitted screenshot.
