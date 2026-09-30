# Proposal: Bearer token login for the September 30 classroom task

The classroom task asks for a token-based login demonstration and a screenshot of the result. The published third lesson uses a Cookie server-side session and explicitly does not mandate JWT. This change adds a Bearer transport for the existing cryptographically random session token, while retaining the prior Cookie transport for older clients.

The browser requests token mode at login, stores the returned token for the current tab, and sends it in the Authorization header on protected API calls. The server continues to resolve role and class from its session and user tables on every request. Logout removes the session row, making the Bearer token invalid immediately.
