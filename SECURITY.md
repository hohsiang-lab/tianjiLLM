# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **security@berri.ai**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix or mitigation**: Depends on severity, targeting 30 days for critical issues

## Scope

This policy covers the TianjiLLM codebase, including:
- Authentication and authorization (master key, virtual keys)
- Request routing and proxy behavior
- Configuration parsing (env var interpolation, secret resolution)
- Web UI session management

## Supported Versions

Security updates are applied to the latest release on the `main` branch.
