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
| **Only if you run `memry usage enable`:** Claude Code's token counts and cost per model, with the token type, model, query source, session id and the time window of each data point | To chart your Claude Code usage in the memry dashboard. No prompts, code or file contents are sent. |

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
