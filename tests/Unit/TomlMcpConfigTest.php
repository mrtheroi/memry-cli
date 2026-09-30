<?php

use App\Support\TomlMcpConfig;

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/memry-toml-'.bin2hex(random_bytes(6));
    $this->path = $this->dir.'/codex/config.toml';
    $this->server = ['command' => '/opt/memry', 'args' => ['mcp']];
    $this->table = "[mcp_servers.memry]\ncommand = \"/opt/memry\"\nargs = [\"mcp\"]\n";
});

afterEach(function () {
    exec('rm -rf '.escapeshellarg($this->dir));
});

function writeToml(string $path, string $contents): void
{
    if (! is_dir(dirname($path))) {
        mkdir(dirname($path), 0700, true);
    }

    file_put_contents($path, $contents);
}

it('creates the file and its directories with only the memry server table', function () {
    expect((new TomlMcpConfig($this->path))->put('memry', $this->server))->toBeTrue()
        ->and(file_get_contents($this->path))->toBe($this->table);
});

it('writes the server environment as an inline table, escaping strings', function () {
    (new TomlMcpConfig($this->path))->put('memry', [...$this->server, 'env' => ['MEMRY_CONFIG' => '/tmp/a "b"\\c.json']]);

    expect(file_get_contents($this->path))->toBe($this->table."env = { MEMRY_CONFIG = \"/tmp/a \\\"b\\\"\\\\c.json\" }\n");
});

it('appends the memry table after the existing config, leaving it byte for byte as it was', function (string $existing, string $separator) {
    writeToml($this->path, $existing);

    (new TomlMcpConfig($this->path))->put('memry', $this->server);

    expect(file_get_contents($this->path))->toBe($existing.$separator.$this->table);
})->with([
    'comments, tables and arrays of tables' => [<<<'TOML'
        # Codex config — Diseño ✓
        model = "o3"  # inline comment

        [mcp_servers.other]
        command = "npx"
        args = [
          "-y",   # a comment inside an array
          "other",
        ]

        [mcp_servers.other.env]
        TOKEN = "x"

        [[profiles.list]]
        name = "a"

        TOML, "\n"],
    'without a final newline' => ['model = "o3"', "\n\n"],
]);

it('replaces an existing memry table and its subtables in place', function () {
    writeToml($this->path, <<<'TOML'
        model = "o3"

        [mcp_servers.memry]
        command = "old"
        args = [
          "[not a header]",
        ]

        [mcp_servers.memry.env]
        MEMRY_CONFIG = "/old"

        # Other servers
        [mcp_servers."other"]
        command = "npx"

        TOML);

    (new TomlMcpConfig($this->path))->put('memry', $this->server);

    expect(file_get_contents($this->path))->toBe(<<<TOML
        model = "o3"

        {$this->table}
        # Other servers
        [mcp_servers."other"]
        command = "npx"

        TOML);
});

it('leaves a file it cannot edit safely untouched', function (string $contents) {
    writeToml($this->path, $contents);

    expect((new TomlMcpConfig($this->path))->put('memry', $this->server))->toBeFalse()
        ->and(file_get_contents($this->path))->toBe($contents);
})->with([
    'unterminated string' => ["model = \"o3\n"],
    'unterminated multi-line string' => ["notes = \"\"\"\nstill open\n"],
    'unclosed array' => ["args = [\n  \"a\",\n"],
    'broken table header' => ["[mcp_servers.other\ncommand = \"x\"\n"],
    'not a key/value line' => ["just some text\n"],
    'memry as an inline table' => ["[mcp_servers]\nmemry = { command = \"old\" }\n"],
    'memry through dotted keys' => ["[mcp_servers]\nmemry.command = \"old\"\n"],
    'mcp_servers as a root inline table' => ["mcp_servers = { other = { command = \"x\" } }\n"],
    'mcp_servers through root dotted keys' => ["mcp_servers.other.command = \"x\"\n"],
]);

it('leaves exactly the same file after putting the same server twice', function () {
    writeToml($this->path, "model = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n");
    $config = new TomlMcpConfig($this->path);
    $config->put('memry', $this->server);
    $once = file_get_contents($this->path);

    $config->put('memry', $this->server);

    expect(file_get_contents($this->path))->toBe($once);
});

it('does not mistake a header-like line inside a multi-line string for a table', function () {
    writeToml($this->path, $contents = "notes = '''\n[mcp_servers.memry]\n'''\n");

    (new TomlMcpConfig($this->path))->put('memry', $this->server);

    expect(file_get_contents($this->path))->toBe($contents."\n".$this->table);
});

it('removes only the memry table and its subtables, restoring the config it was appended to', function () {
    writeToml($this->path, $existing = "# Diseño ✓\nmodel = \"o3\"\n\n[mcp_servers.other]\ncommand = \"npx\"\n");
    $config = new TomlMcpConfig($this->path);
    $config->put('memry', [...$this->server, 'env' => ['A' => 'b']]);

    expect($config->remove('memry'))->toBeTrue()
        ->and(file_get_contents($this->path))->toBe($existing);
});

it('removes a memry table between other tables without leaving a double blank line', function () {
    writeToml($this->path, "[a]\nx = 1\n\n".$this->table."\n# b\n[b]\ny = 2\n");

    (new TomlMcpConfig($this->path))->remove('memry');

    expect(file_get_contents($this->path))->toBe("[a]\nx = 1\n\n# b\n[b]\ny = 2\n");
});

it('removes nothing when there is no memry server or no file', function (?string $contents) {
    if ($contents !== null) {
        writeToml($this->path, $contents);
    }

    expect((new TomlMcpConfig($this->path))->remove('memry'))->toBeFalse()
        ->and(is_file($this->path) ? file_get_contents($this->path) : null)->toBe($contents);
})->with([
    'no file' => [null],
    'other servers' => ["[mcp_servers.other]\ncommand = \"x\"\n"],
    'a server whose name starts with memry' => ["[mcp_servers.memry-old]\ncommand = \"x\"\n"],
]);

it('leaves a file it cannot edit safely untouched when removing', function () {
    writeToml($this->path, $contents = "[mcp_servers]\nmemry = { command = \"old\" }\n");

    expect((new TomlMcpConfig($this->path))->remove('memry'))->toBeNull()
        ->and(file_get_contents($this->path))->toBe($contents);
});

it('replaces the file atomically, keeping its permissions', function () {
    writeToml($this->path, "model = 1\n");
    chmod($this->path, 0640);
    clearstatcache();
    $inode = fileinode($this->path);

    (new TomlMcpConfig($this->path))->put('memry', $this->server);
    clearstatcache();

    expect(fileinode($this->path))->not->toBe($inode)
        ->and(fileperms($this->path) & 0777)->toBe(0640);
});
