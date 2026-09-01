# Security Policy

Please do not disclose exploitable vulnerabilities in public issues. Report them privately to the repository maintainers with a reproduction, affected version, impact, and proposed mitigation.

## Security Model

Nexss Kernel provides **typed context guards** (`RequireRole`, `RequirePermission`, `RequireFeature`, `RequireTenant`, `RateLimit`) that evaluate authentication claims and metadata attached to `context.Context`.

External authentication (validating JWT signatures, OAuth exchanges, password hashing, session lookups) and secret storage belong to application and transport boundary adapters.
