<?php

use Illuminate\Support\Facades\Http;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    $this->configPath = $this->tmpDir.'/config.json';
    Http::preventStrayRequests();
    $this->originalHome = getenv('HOME');
    putenv('MEMRY_CONFIG='.$this->configPath);
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
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
    Http::assertSentCount(1);
});

it('fails without writing the config when the code is invalid or expired', function () {
    fakeServer(token: [422, ['message' => 'Invalid or expired code.']]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '000000')
        ->expectsOutputToContain('Invalid or expired code.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
});

it('fails without writing the config when rate limited', function (string $endpoint) {
    $tooMany = [429, ['message' => 'Too Many Attempts.']];
    $endpoint === 'code' ? fakeServer(code: $tooMany) : fakeServer(token: $tooMany);

    $command = $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com']);
    if ($endpoint === 'token') {
        $command->expectsQuestion('Login code', '123456');
    }

    $command->expectsOutputToContain('Too many attempts, try again later.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
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
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
})->with(['code', 'token']);

it('fails with a generic message when the server errors without a message', function (string $endpoint) {
    $serverError = [500, 'Server Error'];
    $endpoint === 'code' ? fakeServer(code: $serverError) : fakeServer(token: $serverError);

    $command = $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com']);
    if ($endpoint === 'token') {
        $command->expectsQuestion('Login code', '123456');
    }

    $command->expectsOutputToContain('The memry server returned an unexpected error (HTTP 500).')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
})->with(['code', 'token']);

it('fails without writing the config when the server answers without a token', function () {
    fakeServer(token: [200, []]);

    $this->artisan('setup', ['--url' => 'https://memry.test', '--email' => 'ana@example.com'])
        ->expectsQuestion('Login code', '123456')
        ->expectsOutputToContain('The memry server did not return a token.')
        ->assertExitCode(1);

    expect(file_exists($this->configPath))->toBeFalse();
});
