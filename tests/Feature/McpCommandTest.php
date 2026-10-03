<?php

use App\Support\StdioStreams;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Http;
use Symfony\Component\Process\InputStream;
use Symfony\Component\Process\Process;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    mkdir($this->tmpDir, 0700, true);
    $this->configPath = $this->tmpDir.'/config.json';
    file_put_contents($this->configPath, json_encode(['url' => 'https://memry.test', 'token' => 'secret-token']));
    putenv('MEMRY_CONFIG='.$this->configPath);
    Http::preventStrayRequests();
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
    exec('rm -rf '.escapeshellarg($this->tmpDir));
});

/**
 * Run `memry mcp` with the given lines on stdin and return its exit code and
 * exact stdout.
 */
function runMcp(array $lines): array
{
    $input = fopen('php://memory', 'r+');
    fwrite($input, implode("\n", $lines)."\n");
    rewind($input);
    $output = fopen('php://memory', 'r+');
    app()->instance(StdioStreams::class, new StdioStreams($input, $output));

    $exitCode = Artisan::call('mcp');
    rewind($output);

    return [$exitCode, stream_get_contents($output)];
}

it('forwards a request line and prints the server response as one line', function () {
    Http::fake(['*/mcp/memory' => Http::response(['jsonrpc' => '2.0', 'id' => 1, 'result' => ['tools' => []]])]);

    [$exitCode, $output] = runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/list"}']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}'."\n");
});

it('posts the line as is to the MCP endpoint with the bearer token and JSON headers', function () {
    Http::fake(['*/mcp/memory' => Http::response(['jsonrpc' => '2.0', 'id' => 1, 'result' => []])]);

    runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/list"}']);

    Http::assertSent(fn ($request) => $request->method() === 'POST'
        && $request->url() === 'https://memry.test/mcp/memory'
        && $request->header('Authorization') === ['Bearer secret-token']
        && $request->header('Accept') === ['application/json, text/event-stream']
        && $request->header('Content-Type') === ['application/json']
        && $request->body() === '{"jsonrpc":"2.0","id":1,"method":"tools/list"}');
});

it('prints nothing for a notification the server accepts with an empty 202', function () {
    Http::fake(['*/mcp/memory' => Http::response('', 202)]);

    [$exitCode, $output] = runMcp(['{"jsonrpc":"2.0","method":"notifications/initialized"}']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('');
    Http::assertSentCount(1);
});

it('processes every line in order until stdin ends', function () {
    Http::fake(['*/mcp/memory' => fn ($request) => Http::response(
        json_decode($request->body(), true)['method'] === 'notifications/initialized'
            ? ''
            : ['jsonrpc' => '2.0', 'id' => json_decode($request->body(), true)['id'], 'result' => []],
        200,
    )]);

    [$exitCode, $output] = runMcp([
        '{"jsonrpc":"2.0","id":1,"method":"initialize"}',
        '{"jsonrpc":"2.0","method":"notifications/initialized"}',
        '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
    ]);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('{"jsonrpc":"2.0","id":1,"result":[]}'."\n".'{"jsonrpc":"2.0","id":2,"result":[]}'."\n");
    Http::assertSentCount(3);
});

it('answers every request with a not logged in error without contacting the server', function (?array $config) {
    $config === null ? unlink($this->configPath) : file_put_contents($this->configPath, json_encode($config));
    Http::fake();

    [$exitCode, $output] = runMcp([
        '{"jsonrpc":"2.0","id":1,"method":"initialize"}',
        '{"jsonrpc":"2.0","method":"notifications/initialized"}',
        '{"jsonrpc":"2.0","id":"two","method":"tools/list"}',
    ]);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe(
            '{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Not logged in to memry. Run `memry setup`."}}'."\n"
            .'{"jsonrpc":"2.0","id":"two","error":{"code":-32000,"message":"Not logged in to memry. Run `memry setup`."}}'."\n"
        );
    Http::assertNothingSent();
})->with([
    'missing config' => [null],
    'missing url' => [['token' => 'secret-token']],
    'empty token' => [['url' => 'https://memry.test', 'token' => '']],
]);

it('answers a request with a login error when the server rejects the token', function () {
    Http::fake(['*/mcp/memory' => Http::response(['message' => 'Unauthenticated.'], 401)]);

    [$exitCode, $output] = runMcp(['{"jsonrpc":"2.0","id":7,"method":"tools/list"}']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('{"jsonrpc":"2.0","id":7,"error":{"code":-32000,"message":"Your memry login is no longer valid. Run `memry setup`."}}'."\n");
});

it('answers a request with an internal error when the server fails without a JSON-RPC reply and keeps going', function (int $status, array|string $body) {
    Http::fake(['*/mcp/memory' => Http::sequence()
        ->push($body, $status)
        ->push(['jsonrpc' => '2.0', 'id' => 2, 'result' => []])]);

    [$exitCode, $output] = runMcp([
        '{"jsonrpc":"2.0","id":1,"method":"tools/list"}',
        '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
    ]);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe(
            '{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned HTTP '.$status.'."}}'."\n"
            .'{"jsonrpc":"2.0","id":2,"result":[]}'."\n"
        );
})->with([
    'server error page' => [500, '<html>Server Error</html>'],
    'rate limited' => [429, ['message' => 'Too Many Attempts.']],
    'unavailable, empty body' => [503, ''],
]);

it('answers a request with an internal error instead of following a redirect', function () {
    Http::fake([
        'https://memry.test/mcp/memory' => Http::response('', 302, ['Location' => 'https://memry.test/elsewhere']),
        'https://memry.test/elsewhere' => Http::response(['jsonrpc' => '2.0', 'id' => 1, 'result' => []]),
    ]);

    [$exitCode, $output] = runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/list"}']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned HTTP 302."}}'."\n");
    Http::assertNotSent(fn ($request) => $request->url() === 'https://memry.test/elsewhere');
});

it('answers a request with an internal error for a redirect whose body is a JSON-RPC reply', function () {
    Http::fake(['*/mcp/memory' => Http::response(['jsonrpc' => '2.0', 'id' => 1, 'result' => []], 302, ['Location' => 'https://memry.test/elsewhere'])]);

    [, $output] = runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/list"}']);

    expect($output)->toBe('{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned HTTP 302."}}'."\n");
});

it('forwards a JSON-RPC error the server sends with an error status', function (int $status) {
    $error = '{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"The method [foo] was not found."}}';
    Http::fake(['*/mcp/memory' => Http::response($error, $status)]);

    [, $output] = runMcp(['{"jsonrpc":"2.0","id":1,"method":"foo"}']);

    expect($output)->toBe($error."\n");
})->with([400, 404, 500]);

it('answers a request with an internal error when the server is unreachable and keeps going', function () {
    Http::fake(['*/mcp/memory' => Http::sequence()
        ->pushFailedConnection()
        ->push(['jsonrpc' => '2.0', 'id' => 2, 'result' => []])]);

    [$exitCode, $output] = runMcp([
        '{"jsonrpc":"2.0","id":1,"method":"tools/list"}',
        '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
    ]);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe(
            '{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Could not reach the memry server."}}'."\n"
            .'{"jsonrpc":"2.0","id":2,"result":[]}'."\n"
        );
});

it('answers a request with an internal error when the server reply is not JSON', function () {
    Http::fake(['*/mcp/memory' => Http::response('<html>Maintenance</html>', 200)]);

    [, $output] = runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/list"}']);

    expect($output)->toBe('{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned an invalid response."}}'."\n");
});

it('prints a multi-line server reply as a single line', function () {
    Http::fake(['*/mcp/memory' => Http::response(json_encode(['jsonrpc' => '2.0', 'id' => 1, 'result' => ['text' => "a\nb"]], JSON_PRETTY_PRINT)."\r\n")]);

    [, $output] = runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/list"}']);

    expect($output)->toBe('{    "jsonrpc": "2.0",    "id": 1,    "result": {        "text": "a\nb"    }}'."\n")
        ->and(json_decode($output, true)['result']['text'])->toBe("a\nb");
});

it('answers a line that is not JSON with a parse error without contacting the server and keeps going', function () {
    Http::fake(['*/mcp/memory' => Http::response(['jsonrpc' => '2.0', 'id' => 2, 'result' => []])]);

    [$exitCode, $output] = runMcp([
        '{not json',
        '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
    ]);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe(
            '{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}'."\n"
            .'{"jsonrpc":"2.0","id":2,"result":[]}'."\n"
        );
    Http::assertSentCount(1);
});

it('ignores blank lines', function () {
    Http::fake();

    [$exitCode, $output] = runMcp(['', '   ']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('');
    Http::assertNothingSent();
});

it('mirrors the protocol version, method and name into headers for stateless protocol requests', function (string $method, array $params, ?string $name) {
    Http::fake(['*/mcp/memory' => Http::response(['jsonrpc' => '2.0', 'id' => 1, 'result' => []])]);
    $meta = ['io.modelcontextprotocol/protocolVersion' => '2026-07-28', 'io.modelcontextprotocol/clientCapabilities' => (object) []];

    runMcp([json_encode(['jsonrpc' => '2.0', 'id' => 1, 'method' => $method, 'params' => [...$params, '_meta' => $meta]])]);

    Http::assertSent(fn ($request) => $request->header('MCP-Protocol-Version') === ['2026-07-28']
        && $request->header('Mcp-Method') === [$method]
        && $request->header('Mcp-Name') === ($name === null ? [] : [$name]));
})->with([
    'tools/list' => ['tools/list', [], null],
    'tools/call' => ['tools/call', ['name' => 'search-memory', 'arguments' => []], 'search-memory'],
    'prompts/get' => ['prompts/get', ['name' => 'a-prompt'], 'a-prompt'],
    'resources/read' => ['resources/read', ['uri' => 'memry://context'], 'memry://context'],
]);

it('sends no protocol headers for requests of the initialize handshake protocols', function () {
    Http::fake(['*/mcp/memory' => Http::response(['jsonrpc' => '2.0', 'id' => 1, 'result' => []])]);

    runMcp(['{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search-memory"}}']);

    Http::assertSent(fn ($request) => ! $request->hasHeader('MCP-Protocol-Version')
        && ! $request->hasHeader('Mcp-Method')
        && ! $request->hasHeader('Mcp-Name'));
});

it('uses a new login without being restarted', function () {
    Http::fake(['*/mcp/memory' => function ($request) {
        if ($request->header('Authorization') === ['Bearer secret-token']) {
            file_put_contents(getenv('MEMRY_CONFIG'), json_encode(['url' => 'https://memry.test', 'token' => 'new-token']));

            return Http::response(['message' => 'Unauthenticated.'], 401);
        }

        return Http::response(['jsonrpc' => '2.0', 'id' => 2, 'result' => []]);
    }]);

    [, $output] = runMcp([
        '{"jsonrpc":"2.0","id":1,"method":"tools/list"}',
        '{"jsonrpc":"2.0","id":2,"method":"tools/list"}',
    ]);

    expect($output)->toEndWith('{"jsonrpc":"2.0","id":2,"result":[]}'."\n");
    Http::assertSent(fn ($request) => $request->header('Authorization') === ['Bearer new-token']);
});

it('is hidden from the command list', function () {
    $this->artisan('list')
        ->doesntExpectOutputToContain('Proxy MCP messages')
        ->assertExitCode(0);
});

it('writes nothing but JSON-RPC lines to the real stdout', function () {
    unlink($this->configPath);
    $process = new Process([PHP_BINARY, base_path('memry'), 'mcp'], env: ['MEMRY_CONFIG' => $this->configPath]);
    $process->setInput(implode("\n", [
        '{"jsonrpc":"2.0","id":1,"method":"initialize"}',
        '{"jsonrpc":"2.0","method":"notifications/initialized"}',
        'garbage',
    ])."\n");

    $process->run();

    expect($process->getExitCode())->toBe(0)
        ->and($process->getOutput())->toBe(
            '{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Not logged in to memry. Run `memry setup`."}}'."\n"
            .'{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}'."\n"
        );
});

it('answers each line before stdin is closed', function () {
    unlink($this->configPath);
    $input = new InputStream;
    $process = new Process([PHP_BINARY, base_path('memry'), 'mcp'], env: ['MEMRY_CONFIG' => $this->configPath]);
    $process->setInput($input);
    $process->setTimeout(10);
    $process->start();

    $input->write('{"jsonrpc":"2.0","id":1,"method":"initialize"}'."\n");
    $process->waitUntil(fn ($type, $output) => str_contains($output, "\n"));
    $answeredBeforeEof = $process->getOutput();
    $input->close();
    $process->wait();

    expect($answeredBeforeEof)->toBe('{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Not logged in to memry. Run `memry setup`."}}'."\n")
        ->and($process->getExitCode())->toBe(0);
});
