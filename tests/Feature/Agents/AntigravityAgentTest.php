<?php

use App\Agents\AntigravityAgent;
use App\Agents\Protocol;

beforeEach(function () {
    $this->home = sys_get_temp_dir().'/memry-home-'.bin2hex(random_bytes(6));
    mkdir($this->home, 0700, true);
    $this->originalHome = getenv('HOME');
    putenv('HOME='.$this->home);
    config(['memry.executable' => '/opt/homebrew/opt/memry/bin/memry']);
    $this->config = $this->home.'/.gemini/config/mcp_config.json';
    $this->rules = $this->home.'/.gemini/config/GEMINI.md';
    $this->server = ['command' => '/opt/homebrew/opt/memry/bin/memry', 'args' => ['mcp']];
});

afterEach(function () {
    putenv('HOME='.$this->originalHome);
    putenv('MEMRY_CONFIG');
    exec('rm -rf '.escapeshellarg($this->home));
});

function antigravityFile(string $path, string $contents): void
{
    if (! is_dir(dirname($path))) {
        mkdir(dirname($path), 0700, true);
    }

    file_put_contents($path, $contents);
}

it('registers memry as a stdio MCP server in a new ~/.gemini/config/mcp_config.json', function () {
    $result = (new AntigravityAgent)->install('https://memry.test');

    expect($result->successful)->toBeTrue()
        ->and(json_decode(file_get_contents($this->config), true))->toBe(['mcpServers' => ['memry' => $this->server]])
        ->and($result->lines)->toContain(['info', "Registered the memry MCP server in {$this->config}."]);
});

it('is installed when its directory in ~/.gemini exists, not when only Gemini CLI is there', function (?string $directory, bool $installed) {
    mkdir($this->home.'/.gemini');

    if ($directory !== null) {
        mkdir($this->home.'/.gemini/'.$directory);
    }

    expect((new AntigravityAgent)->isInstalled())->toBe($installed);
})->with([
    'only Gemini CLI' => [null, false],
    'antigravity' => ['antigravity', true],
    'config' => ['config', true],
]);

it('passes a custom MEMRY_CONFIG to the server through its environment', function () {
    putenv('MEMRY_CONFIG=/tmp/memry/config.json');

    (new AntigravityAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->config), true)['mcpServers']['memry'])
        ->toBe([...$this->server, 'env' => ['MEMRY_CONFIG' => '/tmp/memry/config.json']]);
});

it('keeps every other setting and server, and non-ASCII text', function () {
    antigravityFile($this->config, '{"mcpServers": {"other": {"serverUrl": "https://x.test/mcp", "headers": {}}}, "note": "Diseño ✓"}');

    (new AntigravityAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->config), true))->toBe([
        'mcpServers' => ['other' => ['serverUrl' => 'https://x.test/mcp', 'headers' => []], 'memry' => $this->server],
        'note' => 'Diseño ✓',
    ])->and(file_get_contents($this->config))->toContain('Diseño ✓')->toContain('"headers": {}');
});

it('leaves the same files after installing twice', function () {
    (new AntigravityAgent)->install('https://memry.test');
    $config = file_get_contents($this->config);
    $rules = file_get_contents($this->rules);

    (new AntigravityAgent)->install('https://memry.test');

    expect(file_get_contents($this->config))->toBe($config)
        ->and(file_get_contents($this->rules))->toBe($rules);
});

it('fails with manual instructions and leaves a malformed mcp_config.json untouched', function () {
    antigravityFile($this->config, $contents = '{"mcpServers": ');

    $result = (new AntigravityAgent)->install('https://memry.test');

    expect($result->successful)->toBeFalse()
        ->and(file_get_contents($this->config))->toBe($contents)
        ->and($result->lines)->toContain(['error', "Could not register the memry MCP server: memry cannot edit {$this->config} safely."])
        ->and($result->lines)->toContain(['line', json_encode(['mcpServers' => ['memry' => $this->server]], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES)]);
});

it('adds the memry instructions to its own GEMINI.md, leaving the one Gemini CLI reads alone', function () {
    antigravityFile($this->rules, "# My rules\n");
    antigravityFile($this->home.'/.gemini/GEMINI.md', $gemini = "# Gemini CLI rules\n");

    (new AntigravityAgent)->install('https://memry.test');

    expect(file_get_contents($this->rules))->toBe("# My rules\n\n<!-- memry:start -->\n".Protocol::text()."\n<!-- memry:end -->\n")
        ->and(file_get_contents($this->home.'/.gemini/GEMINI.md'))->toBe($gemini);
});

it('removes only the memry server and instructions on uninstall', function () {
    antigravityFile($this->config, '{"mcpServers": {"other": {"command": "x"}}}');
    antigravityFile($this->rules, "# My rules\n");
    (new AntigravityAgent)->install('https://memry.test');

    $result = (new AntigravityAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(json_decode(file_get_contents($this->config), true))->toBe(['mcpServers' => ['other' => ['command' => 'x']]])
        ->and(file_get_contents($this->rules))->toBe("# My rules\n");
});

it('succeeds without creating anything when there is nothing to uninstall', function () {
    $result = (new AntigravityAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(file_exists($this->home.'/.gemini'))->toBeFalse();
});
