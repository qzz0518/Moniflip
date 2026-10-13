# Security Policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub:
[Report a vulnerability](https://github.com/qzz0518/Moniflip/security/advisories/new)
(Security tab → "Report a vulnerability"). Do not open a public issue.

Include the Moniflip version, OS and steps to reproduce. You will usually get a reply
within a week; a fix ships in a new release, with credit if you want it.

Only the latest release is supported.

## What Moniflip talks to

- **Your monitors**, over DDC/CI. The only VCP code Moniflip writes is `0x60`
  (input source).
- **Your local network**, only when Apple Home (HomeKit) is turned on: a
  HomeKit accessory server advertised over Bonjour, which only paired Home
  devices can control.
- **GitHub**, to check for and download updates from this repository's
  releases over HTTPS. Updates are signed, and the app installs only those
  that verify against the public key built into it.

There is no other network access, no telemetry and no account.

## In scope

For example: controlling Moniflip through HomeKit without pairing, getting
an update installed that was not signed with the release key, writing VCP
codes other than `0x60`, or privilege escalation through the app, its
updater or the Windows installer.

DDC/CI itself has no authentication: any program on a computer connected to
a monitor can change its settings. That is not a Moniflip vulnerability.
