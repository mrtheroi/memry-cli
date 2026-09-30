<?php

namespace App\Agents;

use App\Support\ClaudeSettings;
use App\Support\Executable;
use Illuminate\Support\Facades\Process;

/**
 * Claude Code: the user-scope memry MCP server, registered with the
 * `claude` CLI, and a SessionStart hook in its settings.json.
 */
class ClaudeCodeAgent implements Agent
{
    public const MCP_SERVER = 'memry';

    public const LEGACY_MCP_SERVER = 'db-memory';

    private const HOOK_MARKER = 'hook:session-start';

    /** @var list<array{string, string}> */
    private array $lines = [];

    public function key(): string
    {
        return 'claude-code';
    }

    public function name(): string
    {
        return 'Claude Code';
    }

    public function isInstalled(): bool
    {
        return Process::run('command -v claude')->successful();
    }

    public function install(string $url): AgentResult
    {
        $this->lines = [];

        // The hook only needs the config file, so install it even when MCP registration fails.
        $registered = $this->registerMcpServer($url);
        $installed = $this->installSessionStartHook();

        return new AgentResult($registered && $installed, $this->lines);
    }

    public function uninstall(): AgentResult
    {
        $this->lines = [];

        // Each step runs even when the other one fails.
        $removed = $this->removeMcpServers();
        $unhooked = $this->removeSessionStartHook();

        return new AgentResult($removed && $unhooked, $this->lines);
    }

    /**
     * Register memry as the user-scope memry MCP server in Claude Code.
     * The token stays in the config file; Claude Code gets it from the
     * headers helper.
     */
    private function registerMcpServer(string $url): bool
    {
        $server = json_encode([
            'type' => 'http',
            'url' => $url.'/mcp/memory',
            'headersHelper' => Executable::command('mcp-headers'),
        ], JSON_UNESCAPED_SLASHES);

        if (! $this->isInstalled()) {
            $this->say('warn', 'Claude Code CLI not found; skipped MCP registration.');

            return $this->failWithManualRegistration($server);
        }

        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', self::LEGACY_MCP_SERVER]);
        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', self::MCP_SERVER]);

        if (Process::run(['claude', 'mcp', 'add-json', '--scope', 'user', self::MCP_SERVER, $server])->failed()
            || Process::run(['claude', 'mcp', 'get', self::MCP_SERVER])->failed()) {
            $this->say('error', 'Could not register the '.self::MCP_SERVER.' MCP server in Claude Code.');

            return $this->failWithManualRegistration($server);
        }

        $this->say('info', 'Registered the '.self::MCP_SERVER.' MCP server in Claude Code (user scope).');

        return true;
    }

    /**
     * Install the Claude Code SessionStart hook that prints the memry
     * context of the current project.
     */
    private function installSessionStartHook(): bool
    {
        $settings = ClaudeSettings::resolve();

        $group = [
            'matcher' => 'startup|resume|clear|compact',
            'hooks' => [['type' => 'command', 'command' => Executable::command(self::HOOK_MARKER), 'timeout' => 10]],
        ];

        if (! $settings->replaceSessionStartHook($group, self::HOOK_MARKER)) {
            $this->say('error', "Could not install the memry SessionStart hook: {$settings->path()} is not valid JSON.");
            $this->say('line', 'Fix the file, then add this group to the "hooks.SessionStart" array by hand:');
            $this->say('line', json_encode($group, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES));

            return false;
        }

        $this->say('info', "Installed the memry SessionStart hook in {$settings->path()}.");

        return true;
    }

    /**
     * Remove the memry MCP server and the legacy one of earlier versions.
     * Failing to remove one only means it was not registered.
     */
    private function removeMcpServers(): bool
    {
        if (! $this->isInstalled()) {
            $this->say('warn', 'Claude Code CLI not found; skipped removing the '.self::MCP_SERVER.' MCP server.');

            return false;
        }

        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', self::LEGACY_MCP_SERVER]);
        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', self::MCP_SERVER])->successful()
            ? $this->say('info', 'Removed the '.self::MCP_SERVER.' MCP server from Claude Code.')
            : $this->say('line', 'The '.self::MCP_SERVER.' MCP server was not registered in Claude Code.');

        return true;
    }

    /**
     * Remove the hook `memry setup` installed, identified like setup does
     * by the hook:session-start command.
     */
    private function removeSessionStartHook(): bool
    {
        $settings = ClaudeSettings::resolve();
        $removed = $settings->removeSessionStartHook(self::HOOK_MARKER);

        if ($removed === null) {
            $this->say('warn', "Could not remove the memry SessionStart hook: {$settings->path()} is not valid JSON.");

            return false;
        }

        $removed
            ? $this->say('info', "Removed the memry SessionStart hook from {$settings->path()}.")
            : $this->say('line', 'No memry SessionStart hook to remove.');

        return true;
    }

    private function failWithManualRegistration(string $server): bool
    {
        $this->say('line', 'Your login was saved. Register the server manually with:');
        $this->say('line', '  claude mcp add-json --scope user '.self::MCP_SERVER.' '.escapeshellarg($server));

        return false;
    }

    private function say(string $style, string $text): void
    {
        $this->lines[] = [$style, $text];
    }
}
