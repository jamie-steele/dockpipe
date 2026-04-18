# Security policy

## Supported versions

dockpipe is **pre-1.0**. Security fixes are applied on the **current development line** and shipped in **new releases** (see repo-root **`VERSION`** and [GitHub Releases](https://github.com/jamie-steele/dockpipe/releases)).

| Version        | Supported                                                         |
| -------------- | ----------------------------------------------------------------- |
| **Latest** `0.x` release | :white_check_mark: Yes — install the newest tag / package. |
| Older `0.x`    | :x: No — please upgrade; we do not maintain long-lived backport lines yet. |

After **1.0**, this table will be updated with explicit minor-version support.

## Reporting a vulnerability

**Please do not open a public issue** for undisclosed security bugs.

1. **Preferred:** Use **[GitHub → Security → Report a vulnerability](https://github.com/jamie-steele/dockpipe/security/advisories/new)** (private advisory) if the feature is enabled on the repo.
2. **Alternative:** Email the maintainer with **`[dockpipe-security]`** in the subject (use the contact method you prefer to publish in your profile or org readme if you add one).

### What to include

- Description of the issue and impact  
- Steps to reproduce (or a proof-of-concept), if possible  
- Affected versions / platforms (if known)

### What to expect

- We aim to acknowledge **within a few business days**.  
- We’ll coordinate a fix and release; you’ll be credited in the advisory / release notes if you want.  
- If the report is out of scope or not accepted, we’ll explain briefly.

## Automation

The repository runs **`govulncheck`**, **`gosec`**, and **CodeQL** in CI; that does not replace responsible disclosure for issues you find in application logic or container workflows.
