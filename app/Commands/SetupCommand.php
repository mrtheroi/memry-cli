<?php

namespace App\Commands;

use App\Agents\AgentRegistry;
use App\Commands\Concerns\ReportsAgentResults;
use App\Support\AuthToken;
use App\Support\ConfigFile;
use App\Support\Email;
use App\Support\RevokeResult;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;
use LaravelZero\Framework\Commands\Command;

use function Laravel\Prompts\multiselect;

class SetupCommand extends Command
{
    use ReportsAgentResults;

    protected $signature = 'setup
        {--url= : The memry server URL}
        {--email= : The email to log in with}
        {--token= : Log in with a token created by the server admin, asked with a hidden prompt (or given as --token=<value>)}
        {--agents= : Comma-separated keys of the agents to wire memry into}';

    protected $description = 'Log in to memry with an email one-time code or a token';

    public function handle(AgentRegistry $agents): int
    {
        if ($this->option('agents') !== null) {
            $unknown = array_diff($this->splitKeys($this->option('agents')), $agents->keys());

            if ($unknown !== []) {
                $this->error('Unknown agent "'.reset($unknown).'". Valid agents: '.implode(', ', $agents->keys()).'.');

                return self::FAILURE;
            }
        }

        if ($this->option('email') !== null && $this->input->hasParameterOption('--token')) {
            $this->error('Use either --email or --token, not both.');

            return self::FAILURE;
        }

        $url = rtrim($this->option('url') ?? config('memry.url'), '/');

        if ($this->input->hasParameterOption('--token')) {
            if ($this->option('token') === null && ! $this->input->isInteractive()) {
                $this->error('Pass --token=<value> when running without interaction.');

                return self::FAILURE;
            }

            $token = trim((string) ($this->option('token') ?? $this->secret('Token')));

            if ($token === '') {
                $this->error('The token is empty.');

                return self::FAILURE;
            }

            if (! $this->acceptsToken($url, $token)) {
                return self::FAILURE;
            }

            $loggedIn = "Connected to {$url}.";
        } else {
            [$token, $loggedIn] = $this->loginWithEmail($url);

            if ($token === null) {
                return self::FAILURE;
            }
        }

        $config = ConfigFile::resolve();
        $previous = $config->read();
        $saved = is_array($previous['agents'] ?? null) ? $previous['agents'] : null;

        $selected = $this->option('agents') !== null ? $this->splitKeys($this->option('agents')) : $this->askAgents($agents, $saved);
        $config->merge(['url' => $url, 'token' => $token, 'agents' => $selected]);

        $this->info("{$loggedIn} Credentials saved to {$config->path()}.");

        if (is_string($previous['token'] ?? null) && is_string($previous['url'] ?? null) && $previous['token'] !== $token) {
            $this->revokePreviousToken($previous['url'], $previous['token']);
        }

        if ($selected === []) {
            $this->warn('No agents selected; memry is not wired into any agent. Run `memry setup` again to choose some.');
        }

        // Every agent is handled, even when an earlier one fails.
        $summary = [];

        foreach ($agents->only($selected) as $agent) {
            $summary[] = $this->report($agent->install($url))
                ? ['info', "{$agent->name()}: memry is set up."]
                : ['error', "{$agent->name()}: failed; see the messages above."];
        }

        foreach ($agents->only(array_diff($saved ?? [], $selected)) as $agent) {
            $summary[] = $this->report($agent->uninstall())
                ? ['info', "{$agent->name()}: memry was removed."]
                : ['error', "{$agent->name()}: failed; see the messages above."];
        }

        $this->newLine();

        foreach ($summary as [$style, $text]) {
            $this->{$style}($text);
        }

        return in_array('error', array_column($summary, 0), true) ? self::FAILURE : self::SUCCESS;
    }

    /**
     * Whether the server accepts the token, telling why when it does not.
     * Any authenticated endpoint would do; the context is a cheap read.
     */
    private function acceptsToken(string $url, string $token): bool
    {
        try {
            $response = Http::acceptJson()->timeout(10)->withToken($token)->get($url.'/api/context', ['project' => 'memry']);
        } catch (ConnectionException) {
            $this->error("Could not reach the memry server at {$url}.");

            return false;
        }

        if ($response->unauthorized()) {
            $this->error("The token was rejected by {$url}.");

            return false;
        }

        if ($response->failed()) {
            $this->failWith($response);

            return false;
        }

        return true;
    }

    /**
     * Log in with an email one-time code, returning the token and the
     * confirmation to show, or nulls (after telling why) when it fails.
     *
     * @return array{0: ?string, 1: ?string}
     */
    private function loginWithEmail(string $url): array
    {
        if ($this->option('email') === null && ! $this->input->isInteractive()) {
            $this->error('Pass --email when running without interaction.');

            return [null, null];
        }

        $email = $this->option('email') !== null ? Email::normalize($this->option('email')) : $this->askEmail();

        if ($email === null) {
            $this->error('Invalid email address given with --email.');

            return [null, null];
        }

        try {
            $response = $this->post($url.'/api/auth/code', ['email' => $email]);

            if ($response->failed()) {
                $this->failWith($response);

                return [null, null];
            }

            $this->line("We sent a login code to {$email}.");

            $code = $this->askCode();

            if ($code === null) {
                $this->error('No login code given. Run `memry setup` interactively to enter the code from the email.');

                return [null, null];
            }

            $response = $this->post($url.'/api/auth/token', ['email' => $email, 'code' => $code]);

            if ($response->failed()) {
                $this->failWith($response);

                return [null, null];
            }
        } catch (ConnectionException) {
            $this->error("Could not reach the memry server at {$url}.");

            return [null, null];
        }

        $token = $response->json('token');

        if (! is_string($token) || $token === '') {
            $this->error('The memry server did not return a token.');

            return [null, null];
        }

        return [$token, "Logged in as {$email}."];
    }

    /**
     * Revoke the token of the previous login on the server it belongs to.
     * A failure never fails setup: the new login is already saved.
     */
    private function revokePreviousToken(string $url, string $token): void
    {
        AuthToken::revoke($url, $token) === RevokeResult::Revoked
            ? $this->info('Revoked the previous memry token.')
            : $this->warn('Could not revoke the previous memry token.');
    }

    /**
     * Ask for the email until it is a valid address.
     */
    private function askEmail(): string
    {
        while (($email = Email::normalize((string) $this->ask('Email'))) === null) {
            $this->error('Enter a valid email address.');
        }

        return $email;
    }

    /**
     * Ask for the login code until it has 6 digits, or return null when
     * no answer can be read at all (as when not interactive).
     */
    private function askCode(): ?string
    {
        while (($answer = $this->ask('Login code')) !== null) {
            if (preg_match('/^\d{6}$/', $code = trim((string) $answer)) === 1) {
                return $code;
            }

            $this->error('Enter the 6-digit code from the email.');
        }

        return null;
    }

    /**
     * Ask which of the supported agents to wire memry into, defaulting to
     * the saved selection, or else the installed ones. The default is also
     * the answer when not interactive.
     *
     * @param  list<string>|null  $saved
     * @return list<string>
     */
    private function askAgents(AgentRegistry $agents, ?array $saved): array
    {
        $options = [];
        $installed = [];

        foreach ($agents->all() as $agent) {
            if ($agent->isInstalled()) {
                $options[$agent->key()] = $agent->name();
                $installed[] = $agent->key();
            } else {
                $options[$agent->key()] = "{$agent->name()} (not installed)";
            }
        }

        $default = $saved !== null ? array_values(array_intersect($saved, $agents->keys())) : $installed;

        if (! $this->input->isInteractive()) {
            return $default;
        }

        return multiselect('Which agents do you use?', $options, $default, hint: 'Space to select, enter to confirm.');
    }

    /**
     * The agent keys of a comma-separated list, trimmed, without empty ones.
     *
     * @return list<string>
     */
    private function splitKeys(string $list): array
    {
        return array_values(array_filter(array_map('trim', explode(',', $list)), fn ($key) => $key !== ''));
    }

    private function post(string $url, array $data): Response
    {
        return Http::acceptJson()->timeout(10)->post($url, $data);
    }

    private function failWith(Response $response): void
    {
        $this->error($response->tooManyRequests()
            ? 'Too many attempts, try again later.'
            : $response->json('message') ?? "The memry server returned an unexpected error (HTTP {$response->status()}).");
    }
}
