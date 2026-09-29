<?php

use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    $this->configPath = $this->tmpDir.'/config.json';
    Http::preventStrayRequests();
    Process::preventStrayProcesses();
    fakeClaude();
    $this->originalHome = getenv('HOME');
    putenv('MEMRY_CONFIG='.$this->configPath);
    $this->settingsPath = $this->tmpDir.'/claude/settings.json';
    putenv('CLAUDE_CONFIG_DIR='.dirname($this->settingsPath));
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
    putenv('CLAUDE_CONFIG_DIR');
    putenv('HOME='.$this->originalHome);
    if (is_dir($this->tmpDir)) {
        exec('rm -rf '.escapeshellarg($this->tmpDir));
    }
});

function fakeServer(array $code = [202, ['message' => 'If the email is valid, a login code has been sent.']], array $token = [200, ['token' => 'secret-token']]): void
{
    Http::fake([
        '*/api/auth/code' => Http::response($code[1], $code[0]),
        '*/api/auth/token' => Http::response($token[1], $token[0]),
    ]);
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

it('writes the url and token to the config file on success', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->configPath), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'secret-token']);
});

it('makes the config file readable by the owner only', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(fileperms($this->configPath) & 0777)->toBe(0600);
});

it('confirms the login without ever printing the token', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Logged in as ana@example.com')
        ->doesntExpectOutputToContain('secret-token')
        ->assertExitCode(0);
});

it('tells the user where the login code was sent', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsOutputToContain('We sent a login code to ana@example.com.')
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);
});

it('sends JSON requests that accept JSON responses', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/auth/code'
        && $request->method() === 'POST'
        && $request->hasHeader('Accept', 'application/json')
        && $request->isJson()
        && $request->data() === ['email' => 'ana@example.com']);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/auth/token'
        && $request->method() === 'POST'
        && $request->hasHeader('Accept', 'application/json')
        && $request->isJson()
        && $request->data() === ['email' => 'ana@example.com', 'code' => '123456']);
});

it('asks for the email when no --email option is given', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test'])
        ->expectsQuestion('Email', 'ana@example.com')
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => str_ends_with($request->url(), '/api/auth/code')
        && $request->data() === ['email' => 'ana@example.com']);
});

it('trims and lowercases the email before sending it', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test'])
        ->expectsQuestion('Email', '  Ana@Example.COM ')
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => str_ends_with($request->url(), '/api/auth/code')
        && $request->data() === ['email' => 'ana@example.com']);
    Http::assertSent(fn ($request) => str_ends_with($request->url(), '/api/auth/token')
        && $request->data()['email'] === 'ana@example.com');
});

it('strips a trailing slash from the server url', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test/', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/auth/code');
    expect(json_decode(file_get_contents($this->configPath), true)['url'])->toBe('https://memry.test');
});

it('uses the configured server url when no --url option is given', function () {
    config(['memry.url' => 'https://configured.memry.test']);
    fakeServer();

    $this->artisan('setup', ['--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Http::assertSent(fn ($request) => $request->url() === 'https://configured.memry.test/api/auth/code');
});

it('overwrites url and token but keeps other keys of an existing config file', function () {
    mkdir($this->tmpDir, 0700, true);
    file_put_contents($this->configPath, json_encode(['url' => 'https://old.test', 'token' => 'old-token', 'project' => 'kept']));
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->configPath), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'secret-token', 'project' => 'kept']);
});

it('creates the missing parent directory readable by the owner only', function () {
    $path = $this->tmpDir.'/nested/memry/config.json';
    putenv('MEMRY_CONFIG='.$path);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(is_file($path))->toBeTrue()
        ->and(fileperms(dirname($path)) & 0777)->toBe(0700);
});

it('writes to ~/.config/memry/config.json when MEMRY_CONFIG is not set', function () {
    putenv('MEMRY_CONFIG');
    putenv('HOME='.$this->tmpDir);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->tmpDir.'/.config/memry/config.json'), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'secret-token']);
});

it('fails without writing the config when the email is rejected', function () {
    fakeServer(code: [422, ['message' => 'The email field must be a valid email address.', 'errors' => ['email' => ['The email field must be a valid email address.']]]]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'not-an-email'])
        ->expectsOutputToContain('The email field must be a valid email address.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
    Process::assertNothingRan();
    Http::assertSentCount(1);
});

it('fails without writing the config when the code is invalid or expired', function () {
    fakeServer(token: [422, ['message' => 'Invalid or expired code.']]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '000000')
        ->expectsOutputToContain('Invalid or expired code.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
    Process::assertNothingRan();
});

it('fails without writing the config when rate limited', function (string $endpoint) {
    $tooMany = [429, ['message' => 'Too Many Attempts.']];
    $endpoint === 'code' ? fakeServer(code: $tooMany) : fakeServer(token: $tooMany);

    $command = $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com']);
    if ($endpoint === 'token') {
        $command->expectsQuestion('Login code', '123456');
    }

    $command->expectsOutputToContain('Too many attempts, try again later.')
        ->assertExitCode(1)
        ->run();

    expect(file_exists($this->configPath))->toBeFalse();
    Process::assertNothingRan();
})->with(['code', 'token']);

it('fails without writing the config when the server cannot be reached', function (string $endpoint) {
    Http::fake([
        '*/api/auth/code' => $endpoint === 'code'
            ? Http::failedConnection()
            : Http::response(['message' => 'If the email is valid, a login code has been sent.'], 202),
        '*/api/auth/token' => Http::failedConnection(),
    ]);

    $command = $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com']);
    if ($endpoint === 'token') {
        $command->expectsQuestion('Login code', '123456');
    }

    $command->expectsOutputToContain('Could not reach the memry server at https://memry.test.')
        ->assertExitCode(1)
        ->run();

    expect(file_exists($this->configPath))->toBeFalse();
    Process::assertNothingRan();
})->with(['code', 'token']);

it('fails with a generic message when the server errors without a message', function (string $endpoint) {
    $serverError = [500, 'Server Error'];
    $endpoint === 'code' ? fakeServer(code: $serverError) : fakeServer(token: $serverError);

    $command = $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com']);
    if ($endpoint === 'token') {
        $command->expectsQuestion('Login code', '123456');
    }

    $command->expectsOutputToContain('The memry server returned an unexpected error (HTTP 500).')
        ->assertExitCode(1)
        ->run();

    expect(file_exists($this->configPath))->toBeFalse();
    Process::assertNothingRan();
})->with(['code', 'token']);

it('fails without writing the config when the server answers without a token', function () {
    fakeServer(token: [200, []]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('The memry server did not return a token.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
    Process::assertNothingRan();
});

/**
 * The command `memry setup` writes for the configured '/opt/memry/memry'
 * executable, prefixed with the custom MEMRY_CONFIG every test sets.
 */
function memryCommand(string $subcommand): string
{
    return 'MEMRY_CONFIG='.escapeshellarg(getenv('MEMRY_CONFIG'))." '/opt/memry/memry' {$subcommand}";
}

it('registers the db-memory MCP server in Claude Code with a headers helper', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Process::assertRan(fn ($process) => $process->command === [
        'claude', 'mcp', 'add-json', '--scope', 'user', 'db-memory',
        json_encode(['type' => 'http', 'url' => 'https://memry.test/mcp/memory', 'headersHelper' => memryCommand('mcp-headers')], JSON_UNESCAPED_SLASHES),
    ]);
});

it('points the headers helper at the running memry script when no helper command is configured', function () {
    putenv('MEMRY_CONFIG');
    putenv('HOME='.$this->tmpDir);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    $helper = escapeshellarg(PHP_BINARY).' '.escapeshellarg(base_path('memry')).' mcp-headers';

    Process::assertRan(fn ($process) => ($process->command[2] ?? null) === 'add-json'
        && json_decode($process->command[6], true)['headersHelper'] === $helper);
});

it('looks up claude, removes any existing user-scope db-memory entry, adds it and verifies it, in that order', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    Process::assertRanInOrder([
        'command -v claude',
        ['claude', 'mcp', 'remove', '--scope', 'user', 'db-memory'],
        fn ($process) => ($process->command[2] ?? null) === 'add-json',
        ['claude', 'mcp', 'get', 'db-memory'],
    ]);
});

it('fails with manual instructions but keeps the config when adding the MCP server fails', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    fakeServer();
    fakeClaude(['add-json' => Process::result(exitCode: 1, errorOutput: 'Invalid configuration')]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Could not register the db-memory MCP server in Claude Code.')
        ->expectsOutputToContain('claude mcp add-json --scope user db-memory '.escapeshellarg(json_encode(['type' => 'http', 'url' => 'https://memry.test/mcp/memory', 'headersHelper' => memryCommand('mcp-headers')], JSON_UNESCAPED_SLASHES)))
        ->assertExitCode(1);

    expect(json_decode(file_get_contents($this->configPath), true))
        ->toBe(['url' => 'https://memry.test', 'token' => 'secret-token']);
});

it('fails with manual instructions when the MCP server cannot be verified', function () {
    fakeServer();
    fakeClaude(['get' => Process::result(exitCode: 1)]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Could not register the db-memory MCP server in Claude Code.')
        ->expectsOutputToContain('claude mcp add-json --scope user db-memory')
        ->assertExitCode(1);

    Process::assertRan(['claude', 'mcp', 'get', 'db-memory']);
});

it('confirms the MCP server registration', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Registered the db-memory MCP server in Claude Code (user scope).')
        ->assertExitCode(0);
});

it('warns with manual instructions and skips registration when the Claude Code CLI is missing', function () {
    fakeServer();
    fakeClaude(installed: false);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Claude Code CLI not found; skipped MCP registration.')
        ->expectsOutputToContain('claude mcp add-json --scope user db-memory')
        ->assertExitCode(1);

    Process::assertDidntRun(fn ($process) => ($process->command[0] ?? null) === 'claude');
    expect(json_decode(file_get_contents($this->configPath), true)['token'])->toBe('secret-token');
});

it('ignores a failure to remove a db-memory entry that does not exist', function () {
    fakeServer();
    fakeClaude(['remove' => Process::result(exitCode: 1, errorOutput: 'No MCP server found with name: db-memory')]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('Registered the db-memory MCP server in Claude Code (user scope).')
        ->assertExitCode(0);
});

it('never passes the token to Claude Code or prints it in the manual instructions', function () {
    fakeServer();
    fakeClaude(['add-json' => Process::result(exitCode: 1)]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->doesntExpectOutputToContain('secret-token')
        ->assertExitCode(1);

    Process::assertRan(fn ($process) => ($process->command[2] ?? null) === 'add-json');
    Process::assertDidntRun(fn ($process) => str_contains(implode(' ', (array) $process->command), 'secret-token'));
});

it('passes a custom MEMRY_CONFIG on to the headers helper', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    $helper = 'MEMRY_CONFIG='.escapeshellarg($this->configPath).' '
        .escapeshellarg(PHP_BINARY).' '.escapeshellarg(base_path('memry')).' mcp-headers';

    Process::assertRan(fn ($process) => ($process->command[2] ?? null) === 'add-json'
        && json_decode($process->command[6], true)['headersHelper'] === $helper);
});

/**
 * The SessionStart matcher group that `memry setup` installs.
 */
function memryHookGroup(?string $command = null): array
{
    return [
        'matcher' => 'startup|resume|clear|compact',
        'hooks' => [['type' => 'command', 'command' => $command ?? memryCommand('hook:session-start'), 'timeout' => 10]],
    ];
}

it('installs the memry SessionStart hook in a new Claude Code settings file', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain("Installed the memry SessionStart hook in {$this->settingsPath}.")
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->settingsPath), true))
        ->toBe(['hooks' => ['SessionStart' => [memryHookGroup()]]]);
});

it('keeps every other setting, event and hook when installing the SessionStart hook', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    $engram = ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'engram context']]];
    $existing = [
        'model' => 'opus',
        'permissions' => ['allow' => ['Bash(ls)'], 'deny' => []],
        'env' => new stdClass,
        'hooks' => [
            'PreToolUse' => [['matcher' => 'Bash', 'hooks' => [['type' => 'command', 'command' => 'echo pre']]]],
            'SessionStart' => [$engram],
        ],
    ];
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, json_encode($existing));
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    $expected = $existing;
    $expected['hooks']['SessionStart'][] = memryHookGroup();

    expect(json_decode(file_get_contents($this->settingsPath)))
        ->toEqual(json_decode(json_encode($expected)));
});

it('replaces a previously installed memry SessionStart hook', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    $engram = ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'engram context']]];
    $mixed = ['matcher' => 'resume', 'hooks' => [
        ['type' => 'command', 'command' => "'/old/memry' hook:session-start"],
        ['type' => 'command', 'command' => 'echo resumed'],
    ]];
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, json_encode(['hooks' => ['SessionStart' => [
        $engram, memryHookGroup("'/old/memry' hook:session-start"), $mixed,
    ]]]));
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->settingsPath), true)['hooks']['SessionStart'])->toBe([
        $engram,
        ['matcher' => 'resume', 'hooks' => [['type' => 'command', 'command' => 'echo resumed']]],
        memryHookGroup(),
    ]);
});

it('leaves exactly one memry SessionStart hook after running setup twice', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    fakeServer();

    foreach ([1, 2] as $run) {
        $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
            ->expectsQuestion('Login code', '123456')
            ->assertExitCode(0);
    }

    expect(json_decode(file_get_contents($this->settingsPath), true)['hooks']['SessionStart'])
        ->toBe([memryHookGroup()]);
});

it('fails with manual instructions and leaves an invalid settings file untouched', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, '{not json');
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain("Could not install the memry SessionStart hook: {$this->settingsPath} is not valid JSON.")
        ->expectsOutputToContain('"command": '.json_encode(memryCommand('hook:session-start'), JSON_UNESCAPED_SLASHES))
        ->expectsOutputToContain('Registered the db-memory MCP server in Claude Code (user scope).')
        ->assertExitCode(1);

    expect(file_get_contents($this->settingsPath))->toBe('{not json')
        ->and(json_decode(file_get_contents($this->configPath), true)['token'])->toBe('secret-token');
});

it('runs the hook with the running memry script and a custom MEMRY_CONFIG', function () {
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    $command = 'MEMRY_CONFIG='.escapeshellarg($this->configPath).' '
        .escapeshellarg(PHP_BINARY).' '.escapeshellarg(base_path('memry')).' hook:session-start';

    expect(json_decode(file_get_contents($this->settingsPath), true)['hooks']['SessionStart'])
        ->toBe([memryHookGroup($command)]);
});

it('installs the hook in ~/.claude/settings.json when CLAUDE_CONFIG_DIR is not set', function () {
    putenv('CLAUDE_CONFIG_DIR');
    putenv('HOME='.$this->tmpDir);
    config(['memry.executable' => "'/opt/memry/memry'"]);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(json_decode(file_get_contents($this->tmpDir.'/.claude/settings.json'), true))
        ->toBe(['hooks' => ['SessionStart' => [memryHookGroup()]]]);
});

it('keeps the permissions of an existing settings file', function () {
    mkdir(dirname($this->settingsPath), 0700, true);
    file_put_contents($this->settingsPath, '{}');
    chmod($this->settingsPath, 0640);
    fakeServer();

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->assertExitCode(0);

    expect(fileperms($this->settingsPath) & 0777)->toBe(0640);
});

it('still installs the hook but fails when the MCP server cannot be registered', function () {
    config(['memry.executable' => "'/opt/memry/memry'"]);
    fakeServer();
    fakeClaude(installed: false);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain("Installed the memry SessionStart hook in {$this->settingsPath}.")
        ->assertExitCode(1);

    expect(json_decode(file_get_contents($this->settingsPath), true)['hooks']['SessionStart'])
        ->toBe([memryHookGroup()]);
});

it('does not touch the Claude Code settings when the login fails', function () {
    fakeServer(token: [422, ['message' => 'Invalid or expired code.']]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '000000')
        ->assertExitCode(1);

    expect(file_exists(dirname($this->settingsPath)))->toBeFalse();
});
