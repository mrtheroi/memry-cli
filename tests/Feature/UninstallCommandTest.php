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

it('revokes the token on the server it belongs to', function () {
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token']);
    fakeServer();

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('Revoked the memry token.')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/auth/token'
        && $request->method() === 'DELETE'
        && $request->hasHeader('Authorization', 'Bearer old-token'));
});

it('warns and fails when the token cannot be revoked', function (?array $revoke) {
    previousConfig();
    fakeServer(revoke: $revoke);

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('Could not revoke the memry token.')
        ->assertExitCode(1);
})->with([
    'server error' => [[500, 'Server Error']],
    'unreachable' => [null],
]);

it('counts a token the server no longer accepts as already revoked', function () {
    previousConfig();
    fakeServer(revoke: [401, ['message' => 'Unauthenticated.']]);

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('The memry token was already revoked.')
        ->doesntExpectOutputToContain('Could not revoke')
        ->assertExitCode(0);
});

it('skips the revoke when not logged in', function (?array $config) {
    if ($config !== null) {
        previousConfig($config);
    }
    fakeServer();

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('Not logged in; no token to revoke.')
        ->assertExitCode(0);

    Http::assertNothingSent();
})->with([
    'no config file' => [null],
    'config without a token' => [['url' => 'https://memry.test']],
]);

it('removes the memry and legacy db-memory MCP servers from Claude Code', function () {
    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('Removed the memry MCP server from Claude Code.')
        ->assertExitCode(0);

    Process::assertRan(['claude', 'mcp', 'remove', '--scope', 'user', 'memry']);
    Process::assertRan(['claude', 'mcp', 'remove', '--scope', 'user', 'db-memory']);
});

it('ignores MCP servers that are not registered', function () {
    fakeClaude(['remove' => Process::result(exitCode: 1, errorOutput: 'No MCP server found')]);

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('The memry MCP server was not registered in Claude Code.')
        ->doesntExpectOutputToContain('Removed the memry MCP server')
        ->assertExitCode(0);
});

it('removes only the memry SessionStart hook from the Claude Code settings', function () {
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, json_encode(['hooks' => ['SessionStart' => [
        ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => "'/opt/memry/memry' hook:session-start", 'timeout' => 10]]],
        ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'other-tool']]],
    ]]]));

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain("Removed the memry SessionStart hook from {$this->settingsPath}.")
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->settingsPath), true))->toBe(['hooks' => ['SessionStart' => [
        ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'other-tool']]],
    ]]]);
});

it('warns, fails and leaves malformed Claude Code settings untouched', function () {
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, '{"hooks": ');

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain("Could not remove the memry SessionStart hook: {$this->settingsPath} is not valid JSON.")
        ->assertExitCode(1);

    expect(file_get_contents($this->settingsPath))->toBe('{"hooks": ');
});

it('reports when there is no memry SessionStart hook to remove', function () {
    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('No memry SessionStart hook to remove.')
        ->assertExitCode(0);

    expect(file_exists($this->settingsPath))->toBeFalse();
});

it('deletes the config file', function () {
    previousConfig();
    fakeServer();

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain("Deleted {$this->configPath}.")
        ->assertExitCode(0);

    expect(file_exists($this->configPath))->toBeFalse();
});

it('warns and fails when the config file cannot be deleted', function () {
    previousConfig();
    fakeServer();
    chmod($this->tmpDir, 0500);

    try {
        $this->artisan('uninstall', ['--force' => true])
            ->expectsOutputToContain("Could not delete {$this->configPath}.")
            ->assertExitCode(1);
    } finally {
        chmod($this->tmpDir, 0700);
    }

    expect(file_exists($this->configPath))->toBeTrue();
});

it('warns, keeps going and fails when the Claude Code CLI is not found', function () {
    fakeClaude(installed: false);
    previousConfig();
    fakeServer();

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('Claude Code CLI not found; skipped removing the memry MCP server.')
        ->expectsOutputToContain("Deleted {$this->configPath}.")
        ->assertExitCode(1);

    Process::assertDidntRun(fn ($process) => ($process->command[0] ?? null) === 'claude');
});

it('removes nothing when the confirmation is declined', function () {
    previousConfig();
    fakeServer();

    $this->artisan('uninstall')
        ->expectsConfirmation('Remove memry from Claude Code and delete your login?', 'no')
        ->expectsOutputToContain('Aborted; nothing was removed.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    Process::assertNothingRan();
    expect(file_exists($this->configPath))->toBeTrue();
});

it('removes everything after the confirmation and ends with the Homebrew hint', function () {
    previousConfig();
    fakeServer();

    $this->artisan('uninstall')
        ->expectsConfirmation('Remove memry from Claude Code and delete your login?', 'yes')
        ->expectsOutputToContain('Run `brew uninstall memry` to remove the CLI.')
        ->assertExitCode(0);

    expect(file_exists($this->configPath))->toBeFalse();
});

it('removes memry from the agents saved by setup', function () {
    fakeAgents($claude = new FakeAgent('claude-code', 'Claude Code'), $codex = new FakeAgent('codex', 'Codex'));
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['codex']]);
    fakeServer();

    $this->artisan('uninstall', ['--force' => true])
        ->expectsOutputToContain('Unwired Codex.')
        ->assertExitCode(0);

    expect($codex->calls)->toBe(['uninstall'])
        ->and($claude->calls)->toBe([]);
});

it('removes memry from Claude Code when setup saved no agents, as setup did up to 0.4.0', function (?array $config) {
    fakeAgents($claude = new FakeAgent('claude-code', 'Claude Code'), $codex = new FakeAgent('codex', 'Codex'));
    if ($config !== null) {
        previousConfig($config);
    }
    fakeServer();

    $this->artisan('uninstall', ['--force' => true])
        ->assertExitCode(0);

    expect($claude->calls)->toBe(['uninstall'])
        ->and($codex->calls)->toBe([]);
})->with([
    'config without agents' => [['url' => 'https://memry.test', 'token' => 'old-token']],
    'no config file' => [null],
]);
