<?php

namespace App\Commands;

use App\Commands\Concerns\RemovesLocalInstall;
use App\Support\AuthToken;
use App\Support\ConfigFile;
use App\Support\RevokeResult;
use LaravelZero\Framework\Commands\Command;

class UninstallCommand extends Command
{
    use RemovesLocalInstall;

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
        $revoked = $this->revokeToken();
        $removed = $this->removeLocalInstall();

        $this->line('Run `brew uninstall memry` to remove the CLI.');

        return $revoked && $removed ? self::SUCCESS : self::FAILURE;
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
}
