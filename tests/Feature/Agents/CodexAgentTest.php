<?php

use App\Agents\CodexAgent;
use App\Agents\Protocol;

beforeEach(function () {
    $this->home = sys_get_temp_dir().'/memry-home-'.bin2hex(random_bytes(6));
    mkdir($this->home, 0700, true);
    $this->originalHome = getenv('HOME');
    putenv('HOME='.$this->home);
    putenv('CODEX_HOME');
    config(['memry.executable' => '/opt/homebrew/opt/memry/bin/memry']);
    $this->config = $this->home.'/.codex/config.toml';
    $this->rules = $this->home.'/.codex/AGENTS.md';
    $this->table = "[mcp_servers.memry]\ncommand = \"/opt/homebrew/opt/memry/bin/memry\"\nargs = [\"mcp\"]\n";
});

afterEach(function () {
    putenv('HOME='.$this->originalHome);
    putenv('CODEX_HOME');
    putenv('MEMRY_CONFIG');
    exec('rm -rf '.escapeshellarg($this->home));
});

it('registers memry as a stdio MCP server in a new ~/.codex/config.toml', function () {
    $result = (new CodexAgent)->install('https://memry.test');

    expect($result->successful)->toBeTrue()
        ->and(file_get_contents($this->config))->toBe($this->table)
        ->and($result->lines)->toContain(['info', "Registered the memry MCP server in {$this->config}."]);
});

it('passes a custom MEMRY_CONFIG to the server through its environment', function () {
    putenv('MEMRY_CONFIG=/tmp/memry/config.json');

    (new CodexAgent)->install('https://memry.test');

    expect(file_get_contents($this->config))->toBe($this->table."env = { MEMRY_CONFIG = \"/tmp/memry/config.json\" }\n");
});

it('uses CODEX_HOME when it is an absolute path', function (string $codexHome, string $expected) {
    putenv('CODEX_HOME='.str_replace('{home}', $this->home, $codexHome));

    (new CodexAgent)->install('https://memry.test');

    expect(file_exists(str_replace('{home}', $this->home, $expected)))->toBeTrue();
})->with([
    'absolute' => ['{home}/custom-codex', '{home}/custom-codex/config.toml'],
    'relative' => ['custom-codex', '{home}/.codex/config.toml'],
]);

it('adds the memry instructions to ~/.codex/AGENTS.md, keeping the other instructions', function () {
    mkdir(dirname($this->rules), 0700, true);
    file_put_contents($this->rules, "# My rules\n\nBe concise. Diseño ✓\n");

    $result = (new CodexAgent)->install('https://memry.test');

    expect(file_get_contents($this->rules))
        ->toBe("# My rules\n\nBe concise. Diseño ✓\n\n<!-- memry:start -->\n".Protocol::text()."\n<!-- memry:end -->\n")
        ->and($result->lines)->toContain(['info', "Added the memry instructions to {$this->rules}."]);
});

it('fails with manual instructions and leaves a config.toml it cannot edit untouched', function () {
    mkdir(dirname($this->config), 0700, true);
    file_put_contents($this->config, $contents = "model = \"o3\n");

    $result = (new CodexAgent)->install('https://memry.test');

    expect($result->successful)->toBeFalse()
        ->and(file_get_contents($this->config))->toBe($contents)
        ->and($result->lines)->toContain(['error', "Could not register the memry MCP server: memry cannot edit {$this->config} safely."])
        ->and($result->lines)->toContain(['line', 'Fix the file, then add this to it by hand:'])
        ->and($result->lines)->toContain(['line', rtrim($this->table)])
        ->and(file_exists($this->rules))->toBeTrue();
});

it('fails and leaves the instructions untouched when their memry markers are broken', function () {
    mkdir(dirname($this->rules), 0700, true);
    file_put_contents($this->rules, $contents = "# My rules\n<!-- memry:start -->\nOld.\n");

    $result = (new CodexAgent)->install('https://memry.test');

    expect($result->successful)->toBeFalse()
        ->and(file_get_contents($this->rules))->toBe($contents)
        ->and($result->lines)->toContain(['error', "Could not add the memry instructions: the <!-- memry:start --> and <!-- memry:end --> markers in {$this->rules} do not enclose one block."])
        ->and($result->lines)->toContain(['line', 'Fix or remove them, then run `memry setup` again.'])
        ->and(file_get_contents($this->config))->toBe($this->table);
});

it('keeps every other setting and server, byte for byte', function () {
    mkdir(dirname($this->config), 0700, true);
    file_put_contents($this->config, $existing = "# Diseño ✓\nmodel = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\nargs = [\"-y\", \"other\"]\n");

    (new CodexAgent)->install('https://memry.test');

    expect(file_get_contents($this->config))->toBe($existing."\n".$this->table);
});

it('leaves the same files after installing twice', function () {
    (new CodexAgent)->install('https://memry.test');
    $config = file_get_contents($this->config);
    $rules = file_get_contents($this->rules);

    $result = (new CodexAgent)->install('https://memry.test');

    expect($result->successful)->toBeTrue()
        ->and(file_get_contents($this->config))->toBe($config)
        ->and(file_get_contents($this->rules))->toBe($rules);
});

it('removes only the memry server and instructions on uninstall', function () {
    mkdir(dirname($this->config), 0700, true);
    file_put_contents($this->config, $config = "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n");
    file_put_contents($this->rules, $rules = "# My rules\n");
    (new CodexAgent)->install('https://memry.test');

    $result = (new CodexAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(file_get_contents($this->config))->toBe($config)
        ->and(file_get_contents($this->rules))->toBe($rules)
        ->and($result->lines)->toBe([
            ['info', "Removed the memry MCP server from {$this->config}."],
            ['info', "Removed the memry instructions from {$this->rules}."],
        ]);
});

it('succeeds without creating anything when there is nothing to uninstall', function () {
    $result = (new CodexAgent)->uninstall();

    expect($result->successful)->toBeTrue()
        ->and(file_exists($this->home.'/.codex'))->toBeFalse()
        ->and($result->lines)->toBe([
            ['line', 'No memry MCP server to remove from '.$this->config.'.'],
            ['line', 'No memry instructions to remove from '.$this->rules.'.'],
        ]);
});

it('warns, fails and leaves files it cannot edit untouched on uninstall', function () {
    mkdir(dirname($this->config), 0700, true);
    file_put_contents($this->config, $config = "model = \"o3\n");
    file_put_contents($this->rules, $rules = "<!-- memry:start -->\nOld.\n");

    $result = (new CodexAgent)->uninstall();

    expect($result->successful)->toBeFalse()
        ->and(file_get_contents($this->config))->toBe($config)
        ->and(file_get_contents($this->rules))->toBe($rules)
        ->and($result->lines)->toBe([
            ['warn', "Could not remove the memry MCP server: memry cannot edit {$this->config} safely."],
            ['warn', "Could not remove the memry instructions: the <!-- memry:start --> and <!-- memry:end --> markers in {$this->rules} do not enclose one block."],
        ]);
});

it('is installed when the codex CLI is on the PATH or its home directory exists', function () {
    $originalPath = getenv('PATH');
    $bin = $this->home.'/bin';
    mkdir($bin);
    putenv('PATH='.$bin);

    try {
        expect((new CodexAgent)->isInstalled())->toBeFalse();

        touch($bin.'/codex');
        chmod($bin.'/codex', 0755);
        expect((new CodexAgent)->isInstalled())->toBeTrue();

        unlink($bin.'/codex');
        mkdir($this->home.'/.codex');
        expect((new CodexAgent)->isInstalled())->toBeTrue();
    } finally {
        putenv('PATH='.$originalPath);
    }
});
