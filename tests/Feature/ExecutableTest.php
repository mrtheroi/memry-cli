<?php

use App\Support\Executable;

afterEach(function () {
    putenv('MEMRY_EXECUTABLE');
    putenv('MEMRY_CONFIG');
});

/**
 * Re-evaluate config/memry.php so the env → config mapping runs against the
 * current process environment, as it does when the PHAR boots.
 */
function reloadMemryConfig(): void
{
    config(['memry' => require config_path('memry.php')]);
}

it('runs the executable given in MEMRY_EXECUTABLE', function () {
    putenv('MEMRY_EXECUTABLE=/opt/homebrew/opt/memry/bin/memry');
    reloadMemryConfig();

    expect(Executable::command('mcp-headers'))->toBe('/opt/homebrew/opt/memry/bin/memry mcp-headers');
});

it('prefixes a custom MEMRY_CONFIG to the configured executable', function () {
    putenv('MEMRY_CONFIG=/tmp/memry config.json');
    config(['memry.executable' => "'/opt/memry/memry'"]);

    expect(Executable::command('mcp-headers'))
        ->toBe("MEMRY_CONFIG='/tmp/memry config.json' '/opt/memry/memry' mcp-headers");
});
