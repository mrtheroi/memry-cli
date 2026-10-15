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
| **Only if you run `memry usage enable`:** the token and cost data points of Claude Code: the token count or cost, the model, the token type, the query source and the time window of each one, and a hash of all its attributes | To chart your Claude Code usage in the memry dashboard. |

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
- The memry CLI sends no telemetry. Usage metrics are off by default: Claude Code sends them only
  after you run `memry usage enable`, and only to the memry server you logged in to, never to
  anyone else.

## Usage metrics, in detail

Claude Code has no per-metric filter. Once you turn usage metrics on, it sends all 8 of its
OpenTelemetry metrics to the memry server: session count, lines of code, pull requests, commits,
cost, token usage, code edit tool decisions and active time. Each one carries your email, your
organization id, an anonymous user id and the terminal type, which Claude Code cannot leave out.
memry turns off the session id and the account uuid. The OTLP resource block also names the
service and its version, the operating system type and version, and the host architecture; memry
does not turn usage metrics on while `OTEL_RESOURCE_ATTRIBUTES` adds custom attributes to it, a
headers helper is set, or Claude Code's managed settings files set telemetry. Policies memry cannot
see, like server-managed settings or an MDM profile, may still change where metrics go. No prompts or code
are sent: memry does not turn on Claude Code's logs or traces. Before turning usage metrics on,
memry checks the OpenTelemetry settings of the shell `memry usage enable` runs in; if Claude Code
is launched from a different environment that exports OpenTelemetry variables, those take
precedence over what memry set.

The memry server keeps only the token and cost data points, tied to your memry account: the token
count or cost, the model, the token type (input, output, cache read or cache creation), the query
source, the time window and the time it was received. It discards the other 6 metrics on receipt.
It does not store your email, organization id, user id, terminal type or any other attribute: it
only computes a SHA-256 hash of the metric name and all the attributes of a data point, and keeps
that hash to tell series apart and to ignore retried exports.

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
