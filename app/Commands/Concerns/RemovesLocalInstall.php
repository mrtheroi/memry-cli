<?php

namespace App\Commands\Concerns;

use App\Commands\SetupCommand;
use App\Support\ClaudeSettings;
use App\Support\ConfigFile;
use Illuminate\Support\Facades\Process;

/**
 * Remove what `memry setup` left on this machine: the MCP servers, the
 * SessionStart hook and the config file with the login.
 */
trait RemovesLocalInstall
{
    /**
     * Run every step, even when an earlier one fails, and return whether
     * all of them succeeded.
     */
    private function removeLocalInstall(): bool
    {
        $results = [
            $this->removeMcpServers(),
            $this->removeSessionStartHook(),
            $this->deleteConfig(),
        ];

        return ! in_array(false, $results, true);
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
