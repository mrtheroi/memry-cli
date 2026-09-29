<?php

use Symfony\Component\Process\Process;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    $this->configPath = $this->tmpDir.'/config.json';
    mkdir($this->tmpDir, 0700, true);
});

afterEach(function () {
    exec('rm -rf '.escapeshellarg($this->tmpDir));
});

/**
 * Run `memry mcp-headers` in a real subprocess, the way Claude Code runs a
 * headersHelper, so stdout and stderr can be told apart.
 */
function runMcpHeaders(string $configPath): Process
{
    $process = new Process([PHP_BINARY, base_path('memry'), 'mcp-headers'], env: ['MEMRY_CONFIG' => $configPath]);
    $process->run();

    return $process;
}

it('prints the authorization header as JSON from the config file', function () {
    file_put_contents($this->configPath, json_encode(['url' => 'https://memry.test', 'token' => 'secret-token']));

    $process = runMcpHeaders($this->configPath);

    expect($process->getExitCode())->toBe(0)
        ->and($process->getOutput())->toBe('{"Authorization":"Bearer secret-token"}'.PHP_EOL);
});

it('fails with nothing on stdout when the config file is missing', function () {
    $process = runMcpHeaders($this->configPath);

    expect($process->getExitCode())->toBe(1)
        ->and($process->getOutput())->toBe('')
        ->and($process->getErrorOutput())->toContain('memry is not logged in. Run memry setup.');
});

it('fails with nothing on stdout when the config has no token', function (array $config) {
    file_put_contents($this->configPath, json_encode($config));

    $process = runMcpHeaders($this->configPath);

    expect($process->getExitCode())->toBe(1)
        ->and($process->getOutput())->toBe('')
        ->and($process->getErrorOutput())->toContain('memry is not logged in. Run memry setup.');
})->with([
    'missing token' => [['url' => 'https://memry.test']],
    'empty token' => [['url' => 'https://memry.test', 'token' => '']],
]);

it('fails with nothing on stdout when the config file is not valid JSON', function () {
    file_put_contents($this->configPath, '{not json');

    $process = runMcpHeaders($this->configPath);

    expect($process->getExitCode())->toBe(1)
        ->and($process->getOutput())->toBe('')
        ->and($process->getErrorOutput())->toContain('memry is not logged in. Run memry setup.');
});

it('is hidden from the command list', function () {
    $this->artisan('list')
        ->doesntExpectOutputToContain('mcp-headers')
        ->assertExitCode(0);
});
