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
    $this->codex = new FakeAgent('codex', 'Codex');
    fakeAgents($this->claude, $this->codex);
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
    if (is_dir($this->tmpDir)) {
        exec('rm -rf '.escapeshellarg($this->tmpDir));
    }
});

it('fails before logging in when --agents has an unknown key', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--agents' => 'claude-code,cursor'])
        ->expectsOutputToContain('Unknown agent "cursor". Valid agents: claude-code, codex.')
        ->assertExitCode(1);

    Http::assertNothingSent();
    expect($this->claude->calls)->toBe([])
        ->and($this->codex->calls)->toBe([])
        ->and(file_exists($this->configPath))->toBeFalse();
});

it('wires only the agents given with --agents and saves them', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--agents' => 'codex'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Wired Codex.')
        ->assertExitCode(0);

    expect($this->codex->calls)->toBe(['install https://memry.test'])
        ->and($this->claude->calls)->toBe([])
        ->and(json_decode(file_get_contents($this->configPath), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'secret-token', 'agents' => ['codex']]);
});

it('asks which agents to wire, labelling the ones not installed', function () {
    fakeAgents($this->claude, $codex = new FakeAgent('codex', 'Codex', installed: false));
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsChoice('Which agents do you use?', ['codex'], ['claude-code' => 'Claude Code', 'codex' => 'Codex (not installed)'])
        ->assertExitCode(0);

    expect($codex->calls)->toBe(['install https://memry.test'])
        ->and($this->claude->calls)->toBe([])
        ->and(json_decode(file_get_contents($this->configPath), true)['agents'])->toBe(['codex']);
});

it('wires the installed agents without asking when not interactive', function () {
    fakeAgents($this->claude, $codex = new FakeAgent('codex', 'Codex', installed: false));
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--no-interaction' => true])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect($this->claude->calls)->toBe(['install https://memry.test'])
        ->and($codex->calls)->toBe([])
        ->and(json_decode(file_get_contents($this->configPath), true)['agents'])->toBe(['claude-code']);
});

it('wires the previously saved agents without asking when not interactive', function () {
    fakeAgents($this->claude, $codex = new FakeAgent('codex', 'Codex', installed: false));
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['codex']]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--no-interaction' => true])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect($codex->calls)->toBe(['install https://memry.test'])
        ->and($this->claude->calls)->toBe([])
        ->and(json_decode(file_get_contents($this->configPath), true)['agents'])->toBe(['codex']);
});

it('drops saved agents this version does not support', function () {
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['cursor', 'codex']]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--no-interaction' => true])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->configPath), true)['agents'])->toBe(['codex']);
});

it('removes memry from previously saved agents that are no longer selected', function () {
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['claude-code', 'codex']]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--agents' => 'codex'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Unwired Claude Code.')
        ->assertExitCode(0);

    expect($this->claude->calls)->toBe(['uninstall'])
        ->and($this->codex->calls)->toBe(['install https://memry.test']);
});

it('keeps wiring the other agents and fails when one agent fails', function () {
    fakeAgents($claude = new FakeAgent('claude-code', 'Claude Code', succeeds: false), $this->codex);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--agents' => 'claude-code,codex'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Could not wire Claude Code.')
        ->expectsOutputToContain('Wired Codex.')
        ->assertExitCode(1);

    expect($claude->calls)->toBe(['install https://memry.test'])
        ->and($this->codex->calls)->toBe(['install https://memry.test']);
});

it('fails when memry cannot be removed from a deselected agent', function () {
    fakeAgents($claude = new FakeAgent('claude-code', 'Claude Code', succeeds: false), $this->codex);
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['claude-code']]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--agents' => 'codex'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(1);

    expect($claude->calls)->toBe(['uninstall']);
});

it('warns but keeps the login when no agent is selected', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsChoice('Which agents do you use?', [], ['claude-code' => 'Claude Code', 'codex' => 'Codex'])
        ->expectsOutputToContain('No agents selected; memry is not wired into any agent. Run `memry setup` again to choose some.')
        ->assertExitCode(0);

    expect($this->claude->calls)->toBe([])
        ->and($this->codex->calls)->toBe([])
        ->and(json_decode(file_get_contents($this->configPath), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'secret-token', 'agents' => []]);
});

it('ends with a summary line per agent', function () {
    fakeAgents($claude = new FakeAgent('claude-code', 'Claude Code', succeeds: false), $this->codex, $gemini = new FakeAgent('gemini', 'Gemini CLI'));
    previousConfig(['url' => 'https://memry.test', 'token' => 'old-token', 'agents' => ['gemini']]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com', '--agents' => 'claude-code,codex'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Claude Code: failed; see the messages above.')
        ->expectsOutputToContain('Codex: memry is set up.')
        ->expectsOutputToContain('Gemini CLI: memry was removed.')
        ->assertExitCode(1);
});
