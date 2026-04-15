# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in bino-plugin-sdk, please report it responsibly.

**Do not open a public issue for security vulnerabilities.**

Instead, please email **sven@bino.bi** with:

- A description of the vulnerability
- Steps to reproduce the issue
- The potential impact
- Any suggested fixes (if applicable)

You should receive an acknowledgement within 72 hours. We will work with you to understand the issue and coordinate a fix before any public disclosure.

## Supported Versions

Security fixes are applied to the latest minor release only. We recommend always running the most recent version of the SDK.

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |
| Older   | No        |

## Scope

The following are in scope for security reports:

- The `github.com/bino-bi/bino-plugin-sdk` Go module source code
- The gRPC contract under `proto/v1/` (`plugin.proto` and generated stubs)

The following are out of scope:

- Third-party dependencies (report these to their respective maintainers)
- The bino CLI itself (see https://github.com/bino-bi/bino-cli)
- Plugins built with this SDK (report to the plugin author)
