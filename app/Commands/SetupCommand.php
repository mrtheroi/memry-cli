<?php

namespace App\Commands;

use App\Support\ConfigFile;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;
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

        return self::SUCCESS;
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
