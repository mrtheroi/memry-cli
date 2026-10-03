<?php

use App\Agents\Agent;
use App\Agents\AgentRegistry;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;
use Tests\TestCase;

/*
|--------------------------------------------------------------------------
| Test Case
|--------------------------------------------------------------------------
|
| The closure you provide to your test functions is always bound to a specific PHPUnit test
| case class. By default, that class is "PHPUnit\Framework\TestCase". Of course, you may
| need to change it using the "uses()" function to bind a different classes or traits.
|
*/

uses(TestCase::class)->in('Feature');

/*
|--------------------------------------------------------------------------
| Expectations
|--------------------------------------------------------------------------
|
| When you're writing tests, you often need to check that values meet certain conditions. The
| "expect()" function gives you access to a set of "expectations" methods that you can use
| to assert different things. Of course, you may extend the Expectation API at any time.
|
*/

expect()->extend('toBeOne', function () {
    return $this->toBe(1);
});

/*
|--------------------------------------------------------------------------
| Functions
|--------------------------------------------------------------------------
|
| While Pest is very powerful out-of-the-box, you may have some testing code specific to your
| project that you don't want to repeat in every file. Here you can also expose helpers as
| global functions to help you to reduce the number of lines of code in your test files.
|
*/

/**
 * Fake the auth endpoints. POST /api/auth/token logs in; DELETE revokes the
 * token used, answering with $revoke (or failing to connect when it is null).
 * GET /api/context answers with $context (or fails to connect when it is null).
 */
function fakeServer(array $code = [202, ['message' => 'If the email is valid, a login code has been sent.']], array $token = [200, ['token' => 'secret-token']], ?array $revoke = [204, ''], ?array $context = [200, '']): void
{
    Http::fake([
        '*/api/context*' => fn ($request) => $context === null
            ? Http::failedConnection()($request)
            : Http::response($context[1], $context[0]),
        '*/api/auth/code' => Http::response($code[1], $code[0]),
        '*/api/auth/token' => fn ($request) => match (true) {
            $request->method() !== 'DELETE' => Http::response($token[1], $token[0]),
            $revoke === null => Http::failedConnection()($request),
            default => Http::response($revoke[1], $revoke[0]),
        },
    ]);
}

/**
 * Write a config file as a previous `memry setup` would have left it.
 */
function previousConfig(array $values = ['url' => 'https://memry.test', 'token' => 'old-token']): void
{
    mkdir(dirname(getenv('MEMRY_CONFIG')), 0700, true);
    file_put_contents(getenv('MEMRY_CONFIG'), json_encode($values));
}

/**
 * Fake the `claude` lookup and `claude mcp` subcommands, succeeding unless a
 * result is given. Calling it again replaces the given results.
 */
function fakeClaude(array $results = [], bool $installed = true): void
{
    Process::fake(['command -v claude' => Process::result(exitCode: $installed ? 0 : 1)]);

    $results += ['remove' => Process::result(), 'add-json' => Process::result(), 'get' => Process::result()];

    foreach ($results as $subcommand => $result) {
        Process::fake(["'claude' 'mcp' '{$subcommand}' *" => $result]);
    }
}

/**
 * Fake DELETE /api/account, answering with $response (or failing to connect
 * when it is null).
 */
function fakeAccountDeletion(?array $response = [204, '']): void
{
    Http::fake([
        '*/api/account' => fn ($request) => $response === null
            ? Http::failedConnection()($request)
            : Http::response($response[1], $response[0]),
    ]);
}

/**
 * Replace the supported agents with the given ones.
 */
function fakeAgents(Agent ...$agents): void
{
    app()->instance(AgentRegistry::class, new AgentRegistry($agents));
}
