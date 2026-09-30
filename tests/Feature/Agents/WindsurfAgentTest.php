<?php

use App\Agents\Protocol;
use App\Agents\WindsurfAgent;

beforeEach(function () {
    $this->home = sys_get_temp_dir().'/memry-home-'.bin2hex(random_bytes(6));
    mkdir($this->home, 0700, true);
    $this->originalHome = getenv('HOME');
    putenv('HOME='.$this->home);
    config(['memry.executable' => '/opt/homebrew/opt/memry/bin/memry']);
    $this->config = $this->home.'/.codeium/windsurf/mcp_config.json';
    $this->rules = $this->home.'/.codeium/windsurf/memories/global_rules.md';
    $this->server = ['command' => '/opt/homebrew/opt/memry/bin/memry', 'args' => ['mcp']];
});

afterEach(function () {
    putenv('HOME='.$this->originalHome);
    putenv('MEMRY_CONFIG');
    exec('rm -rf '.escapeshellarg($this->home));
});

function windsurfFile(string $path, string $contents): void
{
    if (! is_dir(dirname($path))) {
        mkdir(dirname($path), 0700, true);
    }

    file_put_contents($path, $contents);
}

it('registers memry as a stdio MCP server in a new ~/.codeium/windsurf/mcp_config.json', function () {
    $result = (new WindsurfAgent)->install('https://memry.test');

    expect($result->successful)->toBeTrue()
        ->and(json_decode(file_get_contents($this->config), true))->toBe(['mcpServers' => ['memry' => $this->server]])
        ->and($result->lines)->toContain(['info', "Registered the memry MCP server in {$this->config}."]);
});

it('is installed when ~/.codeium/windsurf exists', function () {
    expect((new WindsurfAgent)->isInstalled())->toBeFalse();

    mkdir($this->home.'/.codeium/windsurf', 0700, true);

    expect((new WindsurfAgent)->isInstalled())->toBeTrue();
});

it('passes a custom MEMRY_CONFIG to the server through its environment', function () {
    putenv('MEMRY_CONFIG=/tmp/memry/config.json');

    (new WindsurfAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->config), true)['mcpServers']['memry'])
        ->toBe([...$this->server, 'env' => ['MEMRY_CONFIG' => '/tmp/memry/config.json']]);
});

it('keeps every other setting and server, and non-ASCII text', function () {
    windsurfFile($this->config, '{"mcpServers": {"other": {"serverUrl": "https://x.test/mcp", "env": {}}}, "note": "Diseño ✓"}');

    (new WindsurfAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->config), true))->toBe([
        'mcpServers' => ['other' => ['serverUrl' => 'https://x.test/mcp', 'env' => []], 'memry' => $this->server],
        'note' => 'Diseño ✓',
    ])->and(file_get_contents($this->config))->toContain('Diseño ✓')->toContain('"env": {}');
});

it('leaves the same files after installing twice', function () {
    (new WindsurfAgent)->install('https://memry.test');
    $config = file_get_contents($this->config);
    $rules = file_get_contents($this->rules);

    (new WindsurfAgent)->install('https://memry.test');

    expect(file_get_contents($this->config))->toBe($config)
        ->and(file_get_contents($this->rules))->toBe($rules);
});

it('fails with manual instructions and leaves a malformed mcp_config.json untouched', function () {
    windsurfFile($this->config, $contents = '{"mcpServers": []}');

    $result = (new WindsurfAgent)->install('https://memry.test');

    expect($result->successful)->toBeFalse()
        ->and(file_get_contents($this->config))->toBe($contents)
        ->and($result->lines)->toContain(['error', "Could not register the memry MCP server: memry cannot edit {$this->config} safely."]);
});

it('adds the memry instructions to the global rules, keeping the other rules', function () {
    windsurfFile($this->rules, "# My rules\n");

    (new WindsurfAgent)->install('https://memry.test');

    expect(file_get_contents($this->rules))->toBe("# My rules\n\n<!-- memry:start -->\n".Protocol::text()."\n<!-- memry:end -->\n");
});

it('removes only the memry server and instructions on uninstall', function () {
    windsurfFile($this->config, '{"mcpServers": {"other": {"command": "x"}}}');
    windsurfFile($this->rules, "# My rules\n");
    (new WindsurfAgent)->install('https://memry.test');

    $result = (new WindsurfAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(json_decode(file_get_contents($this->config), true))->toBe(['mcpServers' => ['other' => ['command' => 'x']]])
        ->and(file_get_contents($this->rules))->toBe("# My rules\n");
});

it('succeeds without creating anything when there is nothing to uninstall', function () {
    $result = (new WindsurfAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(file_exists($this->home.'/.codeium'))->toBeFalse();
});
