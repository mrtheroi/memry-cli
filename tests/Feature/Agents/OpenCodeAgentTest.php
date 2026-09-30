<?php

use App\Agents\OpenCodeAgent;
use App\Agents\Protocol;

beforeEach(function () {
    $this->home = sys_get_temp_dir().'/memry-home-'.bin2hex(random_bytes(6));
    mkdir($this->home, 0700, true);
    $this->originalHome = getenv('HOME');
    putenv('HOME='.$this->home);
    putenv('XDG_CONFIG_HOME');
    config(['memry.executable' => '/opt/homebrew/opt/memry/bin/memry']);
    $this->dir = $this->home.'/.config/opencode';
    $this->config = $this->dir.'/opencode.json';
    $this->rules = $this->dir.'/AGENTS.md';
    $this->server = ['type' => 'local', 'command' => ['/opt/homebrew/opt/memry/bin/memry', 'mcp'], 'enabled' => true];
});

afterEach(function () {
    putenv('HOME='.$this->originalHome);
    putenv('XDG_CONFIG_HOME');
    putenv('MEMRY_CONFIG');
    exec('rm -rf '.escapeshellarg($this->home));
});

function openCodeFile(string $path, string $contents): void
{
    if (! is_dir(dirname($path))) {
        mkdir(dirname($path), 0700, true);
    }

    file_put_contents($path, $contents);
}

it('registers memry as a local MCP server in a new ~/.config/opencode/opencode.json', function () {
    $result = (new OpenCodeAgent)->install('https://memry.test');

    expect($result->successful)->toBeTrue()
        ->and(json_decode(file_get_contents($this->config), true))->toBe(['mcp' => ['memry' => $this->server]])
        ->and($result->lines)->toContain(['info', "Registered the memry MCP server in {$this->config}."]);
});

it('passes a custom MEMRY_CONFIG to the server through its environment', function () {
    putenv('MEMRY_CONFIG=/tmp/memry/config.json');

    (new OpenCodeAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->config), true)['mcp']['memry'])
        ->toBe([...$this->server, 'environment' => ['MEMRY_CONFIG' => '/tmp/memry/config.json']]);
});

it('uses $XDG_CONFIG_HOME/opencode when XDG_CONFIG_HOME is set', function () {
    putenv('XDG_CONFIG_HOME='.$this->home.'/xdg');

    (new OpenCodeAgent)->install('https://memry.test');

    expect(file_exists($this->home.'/xdg/opencode/opencode.json'))->toBeTrue()
        ->and(file_exists($this->home.'/xdg/opencode/AGENTS.md'))->toBeTrue()
        ->and(file_exists($this->config))->toBeFalse();
});

it('edits an existing opencode.jsonc when there is no opencode.json, as OpenCode does', function () {
    openCodeFile($this->dir.'/opencode.jsonc', '{"theme": "dark"}');

    (new OpenCodeAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->dir.'/opencode.jsonc'), true))->toBe(['theme' => 'dark', 'mcp' => ['memry' => $this->server]])
        ->and(file_exists($this->config))->toBeFalse();
});

it('keeps every other setting and server, and non-ASCII text', function () {
    openCodeFile($this->config, '{"$schema": "https://opencode.ai/config.json", "theme": "Diseño ✓", "mcp": {"other": {"type": "remote", "url": "https://x.test/mcp", "headers": {}}}}');

    (new OpenCodeAgent)->install('https://memry.test');

    expect(json_decode(file_get_contents($this->config), true))->toBe([
        '$schema' => 'https://opencode.ai/config.json',
        'theme' => 'Diseño ✓',
        'mcp' => ['other' => ['type' => 'remote', 'url' => 'https://x.test/mcp', 'headers' => []], 'memry' => $this->server],
    ])->and(file_get_contents($this->config))->toContain('Diseño ✓')->toContain('"headers": {}');
});

it('leaves the same files after installing twice', function () {
    (new OpenCodeAgent)->install('https://memry.test');
    $config = file_get_contents($this->config);
    $rules = file_get_contents($this->rules);

    (new OpenCodeAgent)->install('https://memry.test');

    expect(file_get_contents($this->config))->toBe($config)
        ->and(file_get_contents($this->rules))->toBe($rules);
});

it('fails with manual instructions and leaves an opencode.jsonc with comments untouched', function () {
    openCodeFile($this->dir.'/opencode.jsonc', $contents = "{\n  // my theme\n  \"theme\": \"dark\"\n}\n");

    $result = (new OpenCodeAgent)->install('https://memry.test');

    expect($result->successful)->toBeFalse()
        ->and(file_get_contents($this->dir.'/opencode.jsonc'))->toBe($contents)
        ->and($result->lines)->toContain(['error', "Could not register the memry MCP server: memry cannot edit {$this->dir}/opencode.jsonc safely."])
        ->and($result->lines)->toContain(['line', json_encode(['mcp' => ['memry' => $this->server]], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES)]);
});

it('adds the memry instructions to AGENTS.md, keeping the other instructions', function () {
    openCodeFile($this->rules, "# My rules\n");

    (new OpenCodeAgent)->install('https://memry.test');

    expect(file_get_contents($this->rules))->toBe("# My rules\n\n<!-- memry:start -->\n".Protocol::text()."\n<!-- memry:end -->\n");
});

it('removes only the memry server and instructions on uninstall', function () {
    openCodeFile($this->config, '{"theme": "dark", "mcp": {"other": {"type": "local", "command": ["x"]}}}');
    openCodeFile($this->rules, "# My rules\n");
    (new OpenCodeAgent)->install('https://memry.test');

    $result = (new OpenCodeAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(json_decode(file_get_contents($this->config), true))->toBe(['theme' => 'dark', 'mcp' => ['other' => ['type' => 'local', 'command' => ['x']]]])
        ->and(file_get_contents($this->rules))->toBe("# My rules\n");
});

it('succeeds without creating anything when there is nothing to uninstall', function () {
    $result = (new OpenCodeAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(file_exists($this->dir))->toBeFalse();
});

it('is installed when the opencode CLI is on the PATH or its config directory exists', function () {
    $originalPath = getenv('PATH');
    $bin = $this->home.'/bin';
    mkdir($bin);
    putenv('PATH='.$bin);

    try {
        expect((new OpenCodeAgent)->isInstalled())->toBeFalse();

        touch($bin.'/opencode');
        chmod($bin.'/opencode', 0755);
        expect((new OpenCodeAgent)->isInstalled())->toBeTrue();

        unlink($bin.'/opencode');
        mkdir($this->dir, 0700, true);
        expect((new OpenCodeAgent)->isInstalled())->toBeTrue();
    } finally {
        putenv('PATH='.$originalPath);
    }
});
