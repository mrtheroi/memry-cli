<?php

namespace App\Commands;

use App\Agents\AgentRegistry;
use App\Commands\Concerns\RemovesLocalInstall;
use App\Support\ConfigFile;
use App\Support\Email;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;
use LaravelZero\Framework\Commands\Command;

class DeleteAccountCommand extends Command
{
    use RemovesLocalInstall;

    protected $signature = 'delete-account';

    protected $description = 'Permanently delete your memry account and all its memories';

    public function handle(AgentRegistry $agents): int
    {
        $config = ConfigFile::resolve()->read();

        if (! is_string($config['url'] ?? null) || ! is_string($config['token'] ?? null)) {
            $this->error('You are not logged in to memry.');

            return self::FAILURE;
        }

        $this->warn('This permanently deletes your memry account and ALL its memories on the server. It cannot be undone.');

        if (($email = $this->askEmail()) === null) {
            $this->line('Aborted; nothing was deleted.');

            return self::FAILURE;
        }

        $error = $this->deleteAccount($config['url'], $config['token'], $email);

        if ($error !== null) {
            $this->error($error);

            return self::FAILURE;
        }

        $this->info('Deleted your memry account and all its memories.');

        // The token was deleted with the account, so there is nothing to revoke.
        $removed = $this->removeLocalInstall($agents);

        $this->line('Run `brew uninstall memry` to remove the CLI.');

        return $removed ? self::SUCCESS : self::FAILURE;
    }

    /**
     * Delete the account on the server, returning why it failed, or null
     * once it is deleted.
     */
    private function deleteAccount(string $url, string $token, string $email): ?string
    {
        try {
            $response = Http::acceptJson()->timeout(10)->withoutRedirecting()->withToken($token)->delete($url.'/api/account', ['email' => $email]);
        } catch (ConnectionException) {
            return 'Could not delete your memry account. Try again later.';
        }

        return match (true) {
            $response->successful() => null,
            $response->unprocessableEntity() => 'The email does not match your memry account.',
            $response->unauthorized() => 'Your memry login is no longer valid. Run `memry setup` and try again.',
            default => 'Could not delete your memry account. Try again later.',
        };
    }

    /**
     * Ask for the account email until it is a valid address, or return
     * null when the answer is left empty.
     */
    private function askEmail(): ?string
    {
        while (($answer = trim((string) $this->ask('Type your account email to confirm'))) !== '') {
            if (($email = Email::normalize($answer)) !== null) {
                return $email;
            }

            $this->error('Enter a valid email address.');
        }

        return null;
    }
}
