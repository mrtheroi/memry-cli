<?php

use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;
use Tests\Fakes\FakeAgent;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    $this->configPath = $this->tmpDir.'/config.json';
    Http::preventStrayRequests();
    Process::preventStrayProcesses();
    fakeClaude();
    putenv('MEMRY_CONFIG='.$this->configPath);
    $this->settingsPath = $this->tmpDir.'/claude/settings.json';
    putenv('CLAUDE_CONFIG_DIR='.dirname($this->settingsPath));
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
    putenv('CLAUDE_CONFIG_DIR');
    if (is_dir($this->tmpDir)) {
        exec('rm -rf '.escapeshellarg($this->tmpDir));
    }
});

it('fails when not logged in', function (?array $config) {
    if ($config !== null) {
        previousConfig($config);
    }

    $this->artisan('delete-account')
        ->expectsOutputToContain('You are not logged in to memry.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    Process::assertNothingRan();
})->with([
    'no config file' => [null],
    'config without a token' => [['url' => 'https://memry.test']],
]);

it('warns and deletes nothing when the email confirmation is left empty', function () {
    previousConfig();

    $this->artisan('delete-account')
        ->expectsOutputToContain('This permanently deletes your memry account and ALL its memories on the server. It cannot be undone.')
        ->expectsQuestion('Type your account email to confirm', '')
        ->expectsOutputToContain('Aborted; nothing was deleted.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    Process::assertNothingRan();
    expect(file_exists($this->configPath))->toBeTrue();
});

it('asks again when the confirmation is not a valid email address', function () {
    previousConfig();

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'not-an-email')
        ->expectsOutputToContain('Enter a valid email address.')
        ->expectsQuestion('Type your account email to confirm', '')
        ->expectsOutputToContain('Aborted; nothing was deleted.')
        ->assertExitCode(1);

    Http::assertNothingSent();
});

it('deletes the account on the server it is logged in to', function () {
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token']);
    fakeAccountDeletion();

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', '  Ana@Example.com ')
        ->expectsOutputToContain('Deleted your memry account and all its memories.')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/account'
        && $request->method() === 'DELETE'
        && $request->hasHeader('Authorization', 'Bearer old-token')
        && $request->hasHeader('Accept', 'application/json')
        && $request->data() === ['email' => 'ana@example.com']);
});

it('fails and removes nothing locally when the email does not match the account', function () {
    previousConfig();
    fakeAccountDeletion([422, ['message' => 'The email does not match this account.']]);

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'bob@example.com')
        ->expectsOutputToContain('The email does not match your memry account.')
        ->doesntExpectOutputToContain('Deleted your memry account')
        ->assertExitCode(1);

    Process::assertNothingRan();
    expect(file_exists($this->configPath))->toBeTrue();
});

it('fails and removes nothing locally when the login is no longer valid', function () {
    previousConfig();
    fakeAccountDeletion([401, ['message' => 'Unauthenticated.']]);

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'ana@example.com')
        ->expectsOutputToContain('Your memry login is no longer valid. Run `memry setup` and try again.')
        ->doesntExpectOutputToContain('Deleted your memry account')
        ->assertExitCode(1);

    Process::assertNothingRan();
    expect(file_exists($this->configPath))->toBeTrue();
});

it('fails and removes nothing locally when the account cannot be deleted', function (?array $response) {
    previousConfig();
    fakeAccountDeletion($response);

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'ana@example.com')
        ->expectsOutputToContain('Could not delete your memry account. Try again later.')
        ->doesntExpectOutputToContain('Deleted your memry account')
        ->assertExitCode(1);

    Process::assertNothingRan();
    expect(file_exists($this->configPath))->toBeTrue();
})->with([
    'server error' => [[500, 'Server Error']],
    'unreachable' => [null],
]);

it('does not follow a redirect from the deletion to a page that answers 200', function () {
    previousConfig();
    Http::fake([
        'https://memry.test/api/account' => Http::response('', 302, ['Location' => 'https://memry.test/login']),
        'https://memry.test/login' => Http::response('<html>Log in</html>', 200),
    ]);

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'ana@example.com')
        ->expectsOutputToContain('Could not delete your memry account. Try again later.')
        ->doesntExpectOutputToContain('Deleted your memry account')
        ->assertExitCode(1);

    Http::assertNotSent(fn ($request) => $request->url() === 'https://memry.test/login');
    Process::assertNothingRan();
    expect(file_exists($this->configPath))->toBeTrue();
});

it('removes memry from this machine after deleting the account', function () {
    previousConfig();
    fakeAccountDeletion();
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, json_encode(['hooks' => ['SessionStart' => [
        ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => "'/opt/memry/memry' hook:session-start", 'timeout' => 10]]],
    ]]]));

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'ana@example.com')
        ->expectsOutputToContain('Removed the memry MCP server from Claude Code.')
        ->expectsOutputToContain("Removed the memry SessionStart hook from {$this->settingsPath}.")
        ->expectsOutputToContain("Deleted {$this->configPath}.")
        ->expectsOutputToContain('Run `brew uninstall memry` to remove the CLI.')
        ->assertExitCode(0);

    Process::assertRan(['claude', 'mcp', 'remove', '--scope', 'user', 'memry']);
    Process::assertRan(['claude', 'mcp', 'remove', '--scope', 'user', 'db-memory']);
    expect(file_exists($this->configPath))->toBeFalse()
        ->and(json_decode(file_get_contents($this->settingsPath), true))->toBe([]);
    Http::assertSentCount(1);
});

it('keeps cleaning up and fails when a local step fails after deleting the account', function () {
    fakeClaude(installed: false);
    previousConfig();
    fakeAccountDeletion();

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'ana@example.com')
        ->expectsOutputToContain('Deleted your memry account and all its memories.')
        ->expectsOutputToContain('Claude Code CLI not found; skipped removing the memry MCP server.')
        ->expectsOutputToContain("Deleted {$this->configPath}.")
        ->assertExitCode(1);
});

it('removes memry from the agents saved by setup after deleting the account', function () {
    fakeAgents($claude = new FakeAgent('claude-code', 'Claude Code'), $codex = new FakeAgent('codex', 'Codex'));
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['codex']]);
    fakeAccountDeletion();

    $this->artisan('delete-account')
        ->expectsQuestion('Type your account email to confirm', 'ana@example.com')
        ->expectsOutputToContain('Unwired Codex.')
        ->assertExitCode(0);

    expect($codex->calls)->toBe(['uninstall'])
        ->and($claude->calls)->toBe([]);
});
