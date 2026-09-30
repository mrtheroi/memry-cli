<?php

namespace App\Commands\Concerns;

use App\Agents\AgentRegistry;
use App\Support\ConfigFile;

/**
 * Remove what `memry setup` left on this machine: memry in every agent it
 * wired, and the config file with the login.
 */
trait RemovesLocalInstall
{
    use ReportsAgentResults;

    /**
     * Run every step, even when an earlier one fails, and return whether
     * all of them succeeded.
     */
    private function removeLocalInstall(AgentRegistry $agents): bool
    {
        $saved = ConfigFile::resolve()->read()['agents'] ?? null;
        $removed = true;

        // Setup saves no agents up to 0.4.0, when it only wired Claude Code.
        foreach ($agents->only(is_array($saved) ? $saved : ['claude-code']) as $agent) {
            $removed = $this->report($agent->uninstall()) && $removed;
        }

        return $this->deleteConfig() && $removed;
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
