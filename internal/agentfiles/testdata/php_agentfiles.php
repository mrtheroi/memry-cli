<?php
// Generates php_agentfiles.jsonl from the PHP CLI's own classes: run
// `php php_agentfiles.php > php_agentfiles.jsonl` from this directory.
// Each line is one case: the file kind, its contents before (null for no
// file), and what put and remove return and leave in the file, each run on
// a fresh copy in a temporary directory.
//
// It needs the PHP CLI's classes, removed in 1.0.0: check out its last PHP
// commit, f819d83d1fd8447ae1d6023ac7f2ce27becd06b0, run `composer install`, then run
// this script from go/internal/agentfiles/testdata, where it lived then, and
// copy the output here.
require __DIR__.'/../../../../vendor/autoload.php';

use App\Support\JsonMcpConfig;
use App\Support\RulesFile;
use App\Support\TomlMcpConfig;

$server = ['command' => '/opt/memry', 'args' => ['mcp'], 'env' => ['MEMRY_CONFIG' => '/tmp/a "b"/é.json']];

$cases = [
    'toml' => [
        null, '', "\n", 'model = "o3"', "model = \"o3\"\r\n",
        "[mcp_servers.memry] # ours\ncommand = \"old\"\n",
        "[mcp_servers.\"memry\"]\ncommand = \"old\"\n",
        "[mcp_servers.'memry']\ncommand = \"old\"\n",
        "[ mcp_servers . memry ]\ncommand = \"old\"\n",
        "[mcp_servers.\"\\u006demry\"]\ncommand = \"old\"\n",
        "[mcp_servers.\"me\\U0000006Dry\"]\ncommand = \"old\"\n",
        "[[mcp_servers.memry]]\ncommand = \"old\"\n",
        "[mcp_servers.memry]\ncommand = \"old\"\n\n[other]\nx = 1\n\n[mcp_servers.memry.env]\nA = \"b\"\n# trailing\n\n",
        "[mcp_servers.memry]\ncommand = \"old\"\n# a comment\n\n# another\n",
        "a = 1\n[mcp_servers.memry]\n",
        "notes = \"\"\"\nquote \\\"\"\" still\n\"\"\"\n[mcp_servers.memry]\nx = 1\n",
        "notes = '''\nback\\'''\n",
        "s = 'a # b'\nt = \"c # d\" # e\n",
        "arr = [ [1, 2], { a = \"]\" } ]\n",
        "arr = [\n  { a = [\n 1 ] },\n]\n[mcp_servers.memry]\ncommand = \"x\"\n",
        "\"é\" = 1\n",
        "a\x0B= 1\n",
        "a.b = 1\n",
        "[mcp_servers]\nother.command = \"x\"\n",
        "[mcp_servers]\n[mcp_servers.memry]\ncommand = \"x\"\n",
        "x = ]\n",
        "x = \"unterminated\n",
        "[a]]\n",
        "\t[mcp_servers.memry]\t\n\tcommand = \"x\"\n",
        "[mcp_servers.memry.env]\nA = \"b\"\n",
        "model = \"o3\"\n\n\n",
        "# only a comment",
    ],
    'json' => [
        null, '', ' ', 'null', '"str"', '1', '{}', '[]', '{"mcpServers": null}', '{"mcpServers": {"memry": null}}',
        '{"mcpServers": {"memry": {"command": "old"}, "other": {}}}', '{"x":1,"x":2,"mcpServers":{}}',
        '{"n": [1.0, 1e2, 0.1, -0, 12345678901234567890]}', '{"s": "\u2028 é \/ \\\\ \u0001"}', '{"": {"": []}}',
        '{"\u0000": 1}', '{"mcpServers": {"0": {}, "1": {}}}', '{"mcpServers": 1}', '{"mcpServers": true}',
        "\xEF\xBB\xBF{}", '{"a": {}}  ', '{"a": {}} {}', '{"mcpServers": {"memry": {}}}',
    ],
    'rules' => [
        null, '', "\n", "# Rules", "# Rules\r\n", "<!-- memry:start -->\nold\n<!-- memry:end -->",
        "<!-- memry:start -->\nold\n<!-- memry:end -->\n", "a\n<!-- memry:start -->old<!-- memry:end -->b\n",
        "a\n\n<!-- memry:start -->\nold\n<!-- memry:end -->\n\n\nb\n", "  \n<!-- memry:start -->\nx\n<!-- memry:end -->\n \t\n",
        "x <!-- memry:end --> <!-- memry:start -->\n",
    ],
];

function attempt(string $kind, ?string $contents, string $method, array $server): array
{
    $dir = sys_get_temp_dir().'/memry-golden-'.bin2hex(random_bytes(6));
    mkdir($dir, 0700);
    $path = $dir.'/file';
    if ($contents !== null) {
        file_put_contents($path, $contents);
    }
    $file = match ($kind) {
        'toml' => new TomlMcpConfig($path),
        'json' => new JsonMcpConfig($path, 'mcpServers'),
        'rules' => new RulesFile($path),
    };
    $result = match (true) {
        $method === 'put' && $kind === 'rules' => $file->put("Use memry.\nÉ"),
        $method === 'put' => $file->put('memry', $server),
        $kind === 'rules' => $file->remove(),
        default => $file->remove('memry'),
    };
    $after = is_file($path) ? file_get_contents($path) : null;
    exec('rm -rf '.escapeshellarg($dir));

    return [$result, $after];
}

foreach ($cases as $kind => $inputs) {
    foreach ($inputs as $contents) {
        echo json_encode([
            'kind' => $kind,
            'contents' => $contents,
            'put' => attempt($kind, $contents, 'put', $server),
            'remove' => attempt($kind, $contents, 'remove', $server),
        ], JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE), "\n";
    }
}
