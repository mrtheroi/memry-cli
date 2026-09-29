<?php

namespace App\Commands;

use App\Support\ClaudeSettings;
use App\Support\ConfigFile;
use App\Support\Executable;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;
use LaravelZero\Framework\Commands\Command;

class SetupCommand extends Command
{
    protected $signature = 'setup
        {--url= : The memry server URL}
        {--email= : The email to log in with}';

    protected $description = 'Log in to memry with an email one-time code';

    public function handle(): int
    {
        $url = rtrim($this->option('url') ?? config('memry.url'), '/');
        $email = strtolower(trim($this->option('email') ?? $this->ask('Email')));

        try {
            $response = $this->post($url.'/api/auth/code', ['email' => $email]);

            if ($response->failed()) {
                return $this->failWith($response);
            }

            $this->line("We sent a login code to {$email}.");

            $code = $this->ask('Login code');

            $response = $this->post($url.'/api/auth/token', ['email' => $email, 'code' => $code]);

            if ($response->failed()) {
                return $this->failWith($response);
            }
        } catch (ConnectionException) {
            $this->error("Could not reach the memry server at {$url}.");

            return self::FAILURE;
        }

        $token = $response->json('token');

        if (! is_string($token) || $token === '') {
            $this->error('The memry server did not return a token.');

            return self::FAILURE;
        }

        $config = ConfigFile::resolve();
        $config->merge(['url' => $url, 'token' => $token]);

        $this->info("Logged in as {$email}. Credentials saved to {$config->path()}.");

        // The hook only needs the config file, so install it even when MCP registration fails.
        $registered = $this->registerMcpServer($url);
        $installed = $this->installSessionStartHook();

        return $registered === self::SUCCESS && $installed === self::SUCCESS ? self::SUCCESS : self::FAILURE;
    }

    /**
     * Register memry as the user-scope db-memory MCP server in Claude Code.
     * The token stays in the config file; Claude Code gets it from the
     * headers helper.
     */
    private function registerMcpServer(string $url): int
    {
        $server = json_encode([
            'type' => 'http',
            'url' => $url.'/mcp/memory',
            'headersHelper' => Executable::command('mcp-headers'),
        ], JSON_UNESCAPED_SLASHES);

        if (Process::run('command -v claude')->failed()) {
            $this->warn('Claude Code CLI not found; skipped MCP registration.');

            return $this->failWithManualRegistration($server);
        }

        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', 'db-memory']);

        if (Process::run(['claude', 'mcp', 'add-json', '--scope', 'user', 'db-memory', $server])->failed()
            || Process::run(['claude', 'mcp', 'get', 'db-memory'])->failed()) {
            $this->error('Could not register the db-memory MCP server in Claude Code.');

            return $this->failWithManualRegistration($server);
        }

        $this->info('Registered the db-memory MCP server in Claude Code (user scope).');

        return self::SUCCESS;
    }

    /**
     * Install the Claude Code SessionStart hook that prints the memry
     * context of the current project.
     */
    private function installSessionStartHook(): int
    {
        $settings = ClaudeSettings::resolve();

        $group = [
            'matcher' => 'startup|resume|clear|compact',
            'hooks' => [['type' => 'command', 'command' => Executable::command('hook:session-start'), 'timeout' => 10]],
        ];

        if (! $settings->replaceSessionStartHook($group, 'hook:session-start')) {
            $this->error("Could not install the memry SessionStart hook: {$settings->path()} is not valid JSON.");
            $this->line('Fix the file, then add this group to the "hooks.SessionStart" array by hand:');
            $this->line(json_encode($group, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES));

            return self::FAILURE;
        }

        $this->info("Installed the memry SessionStart hook in {$settings->path()}.");

        return self::SUCCESS;
    }

    private function failWithManualRegistration(string $server): int
    {
        $this->line('Your login was saved. Register the server manually with:');
        $this->line('  claude mcp add-json --scope user db-memory '.escapeshellarg($server));

        return self::FAILURE;
    }

    private function post(string $url, array $data): Response
    {
        return Http::acceptJson()->timeout(10)->post($url, $data);
    }

    private function failWith(Response $response): int
    {
        $this->error($response->tooManyRequests()
            ? 'Too many attempts, try again later.'
            : $response->json('message') ?? "The memry server returned an unexpected error (HTTP {$response->status()}).");

        return self::FAILURE;
    }
}
