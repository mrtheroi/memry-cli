<?php

namespace App\Commands;

use App\Support\AuthToken;
use App\Support\ClaudeSettings;
use App\Support\ConfigFile;
use App\Support\RevokeResult;
use Illuminate\Support\Facades\Process;
use LaravelZero\Framework\Commands\Command;

class UninstallCommand extends Command
{
    protected $signature = 'uninstall
        {--force : Do not ask for confirmation}';

    protected $description = 'Remove memry from Claude Code and delete your login';

    public function handle(): int
    {
        if (! $this->option('force') && ! $this->confirm('Remove memry from Claude Code and delete your login?')) {
            $this->line('Aborted; nothing was removed.');

            return self::FAILURE;
        }

        // Each step runs even when an earlier one fails.
        $results = [
            $this->revokeToken(),
            $this->removeMcpServers(),
            $this->removeSessionStartHook(),
            $this->deleteConfig(),
        ];

        $this->line('Run `brew uninstall memry` to remove the CLI.');

        return in_array(false, $results, true) ? self::FAILURE : self::SUCCESS;
    }

    private function revokeToken(): bool
    {
        $config = ConfigFile::resolve()->read();

        if (! is_string($config['url'] ?? null) || ! is_string($config['token'] ?? null)) {
            $this->line('Not logged in; no token to revoke.');

            return true;
        }

        // A token the server no longer accepts is as good as revoked.
        $result = AuthToken::revoke($config['url'], $config['token']);

        match ($result) {
            RevokeResult::Revoked => $this->info('Revoked the memry token.'),
            RevokeResult::AlreadyRevoked => $this->line('The memry token was already revoked.'),
            RevokeResult::Failed => $this->warn('Could not revoke the memry token.'),
        };

        return $result !== RevokeResult::Failed;
    }

    /**
     * Remove the memry MCP server and the legacy one of earlier versions.
     * Failing to remove one only means it was not registered.
     */
    private function removeMcpServers(): bool
    {
        if (Process::run('command -v claude')->failed()) {
            $this->warn('Claude Code CLI not found; skipped removing the '.SetupCommand::MCP_SERVER.' MCP server.');

            return false;
        }

        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', SetupCommand::LEGACY_MCP_SERVER]);
        Process::run(['claude', 'mcp', 'remove', '--scope', 'user', SetupCommand::MCP_SERVER])->successful()
            ? $this->info('Removed the '.SetupCommand::MCP_SERVER.' MCP server from Claude Code.')
            : $this->line('The '.SetupCommand::MCP_SERVER.' MCP server was not registered in Claude Code.');

        return true;
    }

    /**
     * Remove the hook `memry setup` installed, identified like setup does
     * by the hook:session-start command.
     */
    private function removeSessionStartHook(): bool
    {
        $settings = ClaudeSettings::resolve();
        $removed = $settings->removeSessionStartHook('hook:session-start');

        if ($removed === null) {
            $this->warn("Could not remove the memry SessionStart hook: {$settings->path()} is not valid JSON.");

            return false;
        }

        $removed
            ? $this->info("Removed the memry SessionStart hook from {$settings->path()}.")
            : $this->line('No memry SessionStart hook to remove.');

        return true;
    }

    /**
     * Delete the config file, and with it the login.
     */
    private function deleteConfig(): bool
    {
        $path = ConfigFile::resolve()->path();

        if (! is_file($path)) {
            $this->line('No config file to delete.');

            return true;
        }

        if (! @unlink($path)) {
            $this->warn("Could not delete {$path}.");

            return false;
        }

        $this->info("Deleted {$path}.");

        return true;
    }
}
