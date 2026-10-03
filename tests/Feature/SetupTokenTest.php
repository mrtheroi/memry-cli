<?php

use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;
use Tests\Fakes\FakeAgent;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    $this->configPath = $this->tmpDir.'/config.json';
    Http::preventStrayRequests();
    Process::preventStrayProcesses();
    putenv('MEMRY_CONFIG='.$this->configPath);
    $this->claude = new FakeAgent('claude-code', 'Claude Code');
    fakeAgents($this->claude);
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
    if (is_dir($this->tmpDir)) {
        exec('rm -rf '.escapeshellarg($this->tmpDir));
    }
});

it('skips the email login when a --token option is given', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->assertExitCode(0);

    Http::assertNotSent(fn ($request) => str_contains($request->url(), '/api/auth/'));
});

it('checks the token against the server before saving it', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => str_starts_with($request->url(), 'https://memry.test/api/context?project=')
        && $request->method() === 'GET'
        && $request->hasHeader('Authorization', 'Bearer admin-token')
        && $request->hasHeader('Accept', 'application/json'));
});

it('fails without saving anything when the server rejects the token', function () {
    fakeServer(context: [401, ['message' => 'Unauthenticated.']]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('The token was rejected by https://memry.test.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse()
        ->and($this->claude->calls)->toBe([]);
});

it('fails without saving anything when the server cannot be reached', function () {
    fakeServer(context: null);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('Could not reach the memry server at https://memry.test.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse()
        ->and($this->claude->calls)->toBe([]);
});

it('fails without saving anything when the server cannot check the token', function () {
    fakeServer(context: [500, 'Server Error']);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('The memry server returned an unexpected error (HTTP 500).')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse()
        ->and($this->claude->calls)->toBe([]);
});

it('saves the url and token and wires the agents without printing the token', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('Connected to https://memry.test.')
        ->expectsOutputToContain('Claude Code: memry is set up.')
        ->doesntExpectOutputToContain('admin-token')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->configPath), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'admin-token', 'agents' => ['claude-code']])
        ->and(fileperms($this->configPath) & 0777)->toBe(0600)
        ->and($this->claude->calls)->toBe(['install https://memry.test']);
});

it('revokes the previous token only when it differs from the given one', function (string $previous, bool $revoked) {
    previousConfig(['url' => 'https://memry.test', 'token' => $previous]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->assertExitCode(0);

    $revoke = fn ($request) => $request->method() === 'DELETE'
        && $request->url() === 'https://memry.test/api/auth/token'
        && $request->hasHeader('Authorization', "Bearer {$previous}");
    $revoked ? Http::assertSent($revoke) : Http::assertNotSent($revoke);
})->with([
    'different token' => ['old-token', true],
    'same token' => ['admin-token', false],
]);

it('fails without sending anything when both --email and --token are given', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('Use either --email or --token, not both.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
});

it('fails without sending anything when the --token option is empty', function (string $token) {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => $token, '--agents' => 'claude-code'])
        ->expectsOutputToContain('The token is empty.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
})->with(['empty' => [''], 'blank' => ['   ']]);

it('trims the token before checking and saving it', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => "  admin-token \n", '--agents' => 'claude-code'])
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->hasHeader('Authorization', 'Bearer admin-token'));
    expect(json_decode(file_get_contents($this->configPath), true)['token'])->toBe('admin-token');
});

it('fails without sending anything when --email is given with a valueless --token', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--token' => null, '--agents' => 'claude-code'])
        ->expectsOutputToContain('Use either --email or --token, not both.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
});

it('asks for the token when --token is given without a value', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => null, '--agents' => 'claude-code'])
        ->expectsQuestion('Token', 'admin-token')
        ->doesntExpectOutputToContain('admin-token')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->hasHeader('Authorization', 'Bearer admin-token'));
    Http::assertNotSent(fn ($request) => str_contains($request->url(), '/api/auth/'));
    expect(json_decode(file_get_contents($this->configPath), true)['token'])->toBe('admin-token');
});

it('fails without sending anything when the entered token is empty', function (string $token) {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => null, '--agents' => 'claude-code'])
        ->expectsQuestion('Token', $token)
        ->expectsOutputToContain('The token is empty.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
})->with(['empty' => [''], 'blank' => ['   ']]);

it('fails without sending anything when --token has no value and there is no interaction', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => null, '--agents' => 'claude-code', '--no-interaction' => true])
        ->expectsOutputToContain('Pass --token=<value> when running without interaction.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
});

it('fails without sending anything when a valueless --email is given with --token', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => null, '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('Use either --email or --token, not both.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
});

it('fails without saving anything when the server answers the token check with a redirect', function () {
    fakeServer(context: [302, '']);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('The memry server returned an unexpected error (HTTP 302).')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse()
        ->and($this->claude->calls)->toBe([]);
});

it('fails without asking or sending anything when --token is given without --url', function (?string $token) {
    fakeServer();

    $this->artisan('setup', ['--token' => $token, '--agents' => 'claude-code'])
        ->expectsOutputToContain('Pass --url with --token, the address of your memry server.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect(file_exists($this->configPath))->toBeFalse();
})->with(['with a value' => ['admin-token'], 'without a value' => [null]]);

it('does not follow a redirect from the token check to a page that answers 200', function () {
    Http::fake([
        'https://memry.test/api/context*' => Http::response('', 302, ['Location' => 'https://memry.test/login']),
        'https://memry.test/login' => Http::response('<html>Log in</html>', 200),
    ]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->expectsOutputToContain('The memry server returned an unexpected error (HTTP 302).')
        ->assertExitCode(1);

    Http::assertNotSent(fn ($request) => $request->url() === 'https://memry.test/login');
    expect(file_exists($this->configPath))->toBeFalse()
        ->and($this->claude->calls)->toBe([]);
});

it('accepts a plain http server url for self-hosted servers', function (string $url) {
    fakeServer();

    $this->artisan('setup', ['--url' => $url, '--token' => 'admin-token', '--agents' => 'claude-code'])
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => str_starts_with($request->url(), rtrim($url, '/').'/api/context?'));
    expect(json_decode(file_get_contents($this->configPath), true)['url'])->toBe(rtrim($url, '/'));
})->with([
    'localhost with a port' => 'http://localhost:8000',
    'a LAN address' => 'http://192.168.1.10/',
    'a service name' => 'http://memry_server',
    'a path' => 'https://tools.company.internal/memry',
]);
