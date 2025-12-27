# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < latest| :x:                |

We currently support only the latest release with security updates. We recommend always using the most recent version.

## Reporting a Vulnerability

We take the security of skopeo-tag-finder seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please do the following:

- **DO NOT** open a public GitHub issue for security vulnerabilities
- Use GitHub's Security Advisory feature to privately report vulnerabilities
- Go to the [Security Advisories](https://github.com/Gankarloo/skopeo-tag-finder/security/advisories) page
- Click "Report a vulnerability"
- Provide a detailed description of the vulnerability

### What to include in your report:

- Type of vulnerability (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the vulnerability
- The location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the vulnerability, including how an attacker might exploit it

### What to expect:

- We will acknowledge your report within 48 hours
- We will provide a more detailed response within 7 days indicating the next steps
- We will work on a fix and coordinate disclosure timing with you
- We will publicly acknowledge your responsible disclosure (unless you prefer to remain anonymous)

## Security Best Practices for Users

When using skopeo-tag-finder:

1. **Keep Updated**: Always use the latest version to ensure you have the most recent security patches
2. **Verify Downloads**: Check SHA256 checksums of downloaded binaries against the checksums.txt file in releases
3. **Registry Credentials**: If using authentication, ensure credentials are stored securely and not exposed in logs or screenshots
4. **Network Security**: Be cautious when using with untrusted container registries
5. **Input Validation**: Ensure image references and digests come from trusted sources

## Vulnerability Disclosure Policy

When we receive a security bug report, we will:

1. Confirm the problem and determine affected versions
2. Audit code to find any similar problems
3. Prepare fixes for all supported releases
4. Release new versions as soon as possible
5. Publish a security advisory on GitHub

Thank you for helping keep skopeo-tag-finder and its users safe!
