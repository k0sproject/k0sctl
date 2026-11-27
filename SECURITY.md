# Security policy

## Supported versions

k0sctl is released from the `main` branch. Only the latest minor release
receives security fixes. Upgrade to the latest release before you report an
issue, if you can.

| Version   | Supported |
|-----------|-----------|
| v0.33.x   | Yes       |
| < v0.33.x | No        |

k0sctl installs and manages k0s. Report vulnerabilities in k0s itself to the
[k0s project](https://github.com/k0sproject/k0s/security/policy).

## Reporting a vulnerability

Do not report security issues in public GitHub issues, pull requests or
discussions.

Report a vulnerability in one of these ways:

1. Use GitHub [private vulnerability reporting]:
   * Go to the [Security tab] of the repository.
   * Click "Advisories".
   * Click "Report a vulnerability".
   * Fill in the details. See below for what to include.

2. Send an email to cncf-k0s-maintainers@lists.cncf.io.

We prefer GitHub private vulnerability reporting. It gives a private channel
where the reporter and the maintainers can coordinate the fix and the
disclosure.

We try to acknowledge reports within a few working days. If you get no
response, contact the maintainers by email to confirm that we received the
report.

[private vulnerability reporting]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability
[Security tab]: https://github.com/k0sproject/k0sctl/security

### What to include

Give as much detail as you can. For example:

* The k0sctl version (`k0sctl version`) and the k0s version.
* The operating system of the machine that runs k0sctl and of the target hosts.
* The connection type to the hosts (SSH, WinRM, OpenSSH client, localhost).
* The configuration you used (`k0sctl.yaml`, with secrets removed) and the
  command line flags.
* The steps to reproduce the issue.
* The impact of the issue, and the severity you think it has.
* Logs or other output. Remove credentials, keys and tokens first.

## Review process

When a maintainer confirms the report, they create a draft security advisory on
GitHub. The maintainers and the reporter discuss the issue in the advisory. Give
your GitHub username if you want an invitation. If you do not want to take part,
you can ask for updates by email.

If we accept the vulnerability, the maintainers and the reporter agree on a
timeline for the fix, the release and the public disclosure. We expect the
reporter to keep to the agreed dates.

## Public disclosure

When the fix is released, we publish a GitHub [security advisory] and mention
the issue in the release notes of the fixed version.

[security advisory]: https://docs.github.com/en/code-security/security-advisories/about-security-advisories
