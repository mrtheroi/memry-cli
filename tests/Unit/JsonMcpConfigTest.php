<?php

use App\Support\JsonMcpConfig;

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/memry-json-'.bin2hex(random_bytes(6));
    $this->path = $this->dir.'/agent/mcp_config.json';
    $this->server = ['command' => '/opt/memry', 'args' => ['mcp']];
});

afterEach(function () {
    exec('rm -rf '.escapeshellarg($this->dir));
});

function writeJson(string $path, string $contents): void
{
    if (! is_dir(dirname($path))) {
        mkdir(dirname($path), 0700, true);
    }

    file_put_contents($path, $contents);
}

it('creates the file and its directories with only the memry server', function () {
    expect((new JsonMcpConfig($this->path, 'mcpServers'))->put('memry', $this->server))->toBeTrue()
        ->and(file_get_contents($this->path))->toBe(<<<'JSON'
            {
                "mcpServers": {
                    "memry": {
                        "command": "/opt/memry",
                        "args": [
                            "mcp"
                        ]
                    }
                }
            }

            JSON);
});

it('keeps every other server and setting as it was, replacing only memry', function () {
    writeJson($this->path, '{"theme": "Diseño ✓", "empty": {}, "mcpServers": {"other": {"command": "npx", "args": ["-y", "other"]}, "memry": {"command": "old"}}, "url": "https://x.test/a"}');

    (new JsonMcpConfig($this->path, 'mcpServers'))->put('memry', $this->server);

    expect(json_decode(file_get_contents($this->path), true))->toBe([
        'theme' => 'Diseño ✓',
        'empty' => [],
        'mcpServers' => [
            'other' => ['command' => 'npx', 'args' => ['-y', 'other']],
            'memry' => $this->server,
        ],
        'url' => 'https://x.test/a',
    ])->and(file_get_contents($this->path))
        ->toContain('"empty": {}')
        ->toContain('Diseño ✓')
        ->toContain('https://x.test/a');
});

it('leaves a file it cannot parse untouched', function (string $contents) {
    writeJson($this->path, $contents);

    expect((new JsonMcpConfig($this->path, 'mcpServers'))->put('memry', $this->server))->toBeFalse()
        ->and(file_get_contents($this->path))->toBe($contents);
})->with([
    'invalid JSON' => ['{"mcpServers": {'],
    'JSON with comments' => ["{\n  // my servers\n  \"mcpServers\": {}\n}"],
    'not an object' => ['[]'],
    'servers not an object' => ['{"mcpServers": []}'],
    'server list is a string' => ['{"mcpServers": "nope"}'],
]);

it('removes only the memry server, keeping the others', function () {
    writeJson($this->path, '{"theme": "Diseño ✓", "mcpServers": {"other": {"command": "npx"}, "memry": {"command": "old"}}}');

    expect((new JsonMcpConfig($this->path, 'mcpServers'))->remove('memry'))->toBeTrue()
        ->and(json_decode(file_get_contents($this->path), true))
        ->toBe(['theme' => 'Diseño ✓', 'mcpServers' => ['other' => ['command' => 'npx']]]);
});

it('drops the server list when memry was the only server in it', function () {
    writeJson($this->path, '{"theme": "dark", "mcpServers": {"memry": {"command": "old"}}}');

    (new JsonMcpConfig($this->path, 'mcpServers'))->remove('memry');

    expect(json_decode(file_get_contents($this->path), true))->toBe(['theme' => 'dark']);
});

it('removes nothing when there is no memry server or no file', function (?string $contents) {
    if ($contents !== null) {
        writeJson($this->path, $contents);
    }

    expect((new JsonMcpConfig($this->path, 'mcpServers'))->remove('memry'))->toBeFalse()
        ->and(is_file($this->path) ? file_get_contents($this->path) : null)->toBe($contents);
})->with([
    'no file' => [null],
    'no servers' => ['{"theme":"dark"}'],
    'other servers' => ['{"mcpServers":{"other":{}}}'],
]);

it('leaves a file it cannot parse untouched when removing', function () {
    writeJson($this->path, $contents = '{"mcpServers": {');

    expect((new JsonMcpConfig($this->path, 'mcpServers'))->remove('memry'))->toBeNull()
        ->and(file_get_contents($this->path))->toBe($contents);
});

it('replaces the file atomically, keeping its permissions', function () {
    writeJson($this->path, '{}');
    chmod($this->path, 0640);
    clearstatcache();
    $inode = fileinode($this->path);

    (new JsonMcpConfig($this->path, 'mcpServers'))->put('memry', $this->server);
    clearstatcache();

    expect(fileinode($this->path))->not->toBe($inode)
        ->and(fileperms($this->path) & 0777)->toBe(0640);
});
