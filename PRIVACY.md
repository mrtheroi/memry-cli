# memry Privacy Policy

Last updated: 2026-10-15 · [Versión en español](PRIVACY.es.md)

memry is a persistent memory service for AI coding agents. It is operated by Cesar Valero, based in
Mexico ("we"). This policy explains what memry stores, why, who else handles it, and how you can
delete it. memry is in public beta.

Questions and requests: **mrtheroi@gmail.com**

## What we store

| Data | Why |
| --- | --- |
| Your email address | To identify your account and send you login codes. |
| The memories your agents save: title, content, type, project name, topic key and session id | This is the service: your agents read them back in later sessions. |
| The prompts your agents save with the `save-prompt` tool, with their project and session id | Same as above. |
| Login codes, stored only as a hash | To log you in. They expire after 5 minutes and are deleted a day later. |
| Access tokens, stored only as a hash | To let your machine use the service without logging in again. |
| Your IP address | To rate-limit login attempts, and in the server's operational logs, which the hosting provider keeps for a limited time. |
| **Only if you run `memry usage enable`:** Claude Code's token and cost data points: the token count or cost, the model, the token type, the query source and the time window of each one, and a keyed hash of its attributes. Everything else Claude Code sends is discarded on receipt | To chart your Claude Code usage in the memry dashboard. |

What your agents save is up to you and them. **Do not let them save passwords, API keys or other
secrets** in memry.

On your machine, `memry setup` keeps the server URL and your access token in
`~/.config/memry/config.json`, readable only by your user. If you turn on usage metrics,
`memry usage enable` also writes a usage token, which can only send usage metrics, to Claude Code's
`~/.claude/settings.json`, and leaves that file readable only by your user.

## What we do not do

- We do not sell your data or share it with advertisers.
- We do not use your memories to train AI models.
- We do not read your memories, except when you ask us to help with a problem or when the law
  requires it.
- memry has no website tracking, analytics or cookies.
- The memry CLI sends no telemetry. Usage metrics are off by default: memry configures Claude Code
  to send them only after you run `memry usage enable`, and only to the memry server you logged in
  to (see the limits of what memry can check below).

## Usage metrics, in detail

Usage metrics are off unless you run `memry usage enable`. Then Claude Code, not memry, decides
what it sends, as documented in its
[monitoring guide](https://code.claude.com/docs/en/monitoring-usage): its OpenTelemetry metrics,
such as token usage, cost, sessions and lines of code, with identity attributes such as your email
and organization id, and metric-specific attributes. That list is not exhaustive and may grow with
new Claude Code versions. memry turns off the optional attribute gates it can, and does not turn
on Claude Code's logs or traces, so no prompts or code are sent through it.

**What the memry server keeps is the guarantee.** It keeps only the token and cost data points,
tied to your memry account: the token count or cost, the model, the token type (input, output,
cache read or cache creation), the query source, the time window and the time it was received. It
discards everything else on receipt. It does not store your email, organization id or any other
attribute: it only computes a keyed hash (HMAC) of the metric name and the attributes of a data
point, to tell series apart and to ignore retried exports. The ingest endpoint does not log
requests or their headers.

**What memry can check.** Before turning usage metrics on, memry checks the environment of the
shell `memry usage enable` runs in, your Claude Code `settings.json` and Claude Code's managed
settings files, and refuses if other telemetry is configured there. It cannot see per-session
`--settings` files, managed policy helpers, MDM profiles, the Windows registry, server-managed
settings or the environment Claude Code is launched from; any of them can change what Claude Code
sends or where. If you use mTLS, Claude Code may also present its client certificate to your memry
server.

## Who else handles your data

- **Laravel Cloud** hosts the memry server and its database.
- **Resend** delivers the login code emails, so it receives your email address and the code.

All traffic between your machine and memry uses HTTPS.

## How long we keep it

We keep your account, memories, prompts and usage metrics until you delete them. Turning usage
metrics off stops new ones; the ones already sent are deleted with your account. Login codes are deleted a day
after they expire. Hosting and email providers may keep operational logs for a limited time under
their own policies.

## Your choices

- **Stop using memry on a machine:** `memry uninstall` revokes that machine's token and removes memry
  from Claude Code.
- **Stop sending usage metrics:** `memry usage disable` (or `memry uninstall`) removes the setting
  from Claude Code. `memry usage status` shows whether they are on.
- **Delete everything:** `memry delete-account` permanently deletes your account, every memory,
  prompt and usage metric, all your tokens and login codes. It cannot be undone.
- **Access, correct or delete your data, or object to its use:** write to
  mrtheroi@gmail.com. We will answer within 20 business days.

## Changes

If we change this policy, we will update the date above and describe the change in the
[CHANGELOG](CHANGELOG.md). If a change affects how we use your data, we will tell you by email before
it applies.
