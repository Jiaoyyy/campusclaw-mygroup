# Proposal: Bearer token login for the September 30 classroom task

The classroom task asks for a token-based login demonstration and a screenshot of the result. The published third lesson uses a Cookie server-side session and explicitly does not mandate JWT. For this assignment the application uses only the Bearer transport for its cryptographically random session token; Cookie login is removed.

The browser requests token mode at login, stores the returned token for the current tab, and sends it in the Authorization header on protected API calls. The server continues to resolve role and class from its session and user tables on every request. Logout removes the session row, making the Bearer token invalid immediately.
