<?php

namespace App\Commands;

use App\Support\ConfigFile;
use App\Support\StdioStreams;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;
use LaravelZero\Framework\Commands\Command;

class McpCommand extends Command
{
    protected $signature = 'mcp';

    protected $description = 'Proxy MCP messages between stdio and the memry server';

    protected $hidden = true;

    public function handle(StdioStreams $stdio): int
    {
        while (($line = fgets($stdio->input)) !== false) {
            $message = trim($line);
            $reply = $message !== '' ? $this->forward($message) : null;

            if ($reply !== null) {
                fwrite($stdio->output, $reply."\n");
                fflush($stdio->output);
            }
        }

        return self::SUCCESS;
    }

    /**
     * Send one message to the server and return the line to print, or null
     * when there is nothing to print.
     */
    private function forward(string $message): ?string
    {
        if (! json_validate($message)) {
            return '{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}';
        }

        $config = ConfigFile::resolve()->read();

        if (! is_string($config['url'] ?? null) || $config['url'] === ''
            || ! is_string($config['token'] ?? null) || $config['token'] === '') {
            return $this->errorReply($message, -32000, 'Not logged in to memry. Run `memry setup`.');
        }

        try {
            $response = Http::withToken($config['token'])
                ->accept('application/json, text/event-stream')
                ->timeout(30)
                ->withoutRedirecting()
                ->withHeaders($this->protocolHeaders($message))
                ->withBody($message, 'application/json')
                ->post($config['url'].'/mcp/memory');
        } catch (ConnectionException) {
            return $this->errorReply($message, -32603, 'Could not reach the memry server.');
        }

        if ($response->status() === 401) {
            return $this->errorReply($message, -32000, 'Your memry login is no longer valid. Run `memry setup`.');
        }

        // Neither a success nor an error, such as a redirect: whatever its body says.
        if (! $response->successful() && ! $response->failed()) {
            return $this->errorReply($message, -32603, "The memry server returned HTTP {$response->status()}.");
        }

        // laravel/mcp answers JSON-RPC errors with a 4xx or 5xx status.
        if ($response->failed() && ! isset($response->json()['jsonrpc'])) {
            return $this->errorReply($message, -32603, "The memry server returned HTTP {$response->status()}.");
        }

        if ($response->body() === '') {
            return null;
        }

        if (! json_validate($response->body())) {
            return $this->errorReply($message, -32603, 'The memry server returned an invalid response.');
        }

        // Line breaks in valid JSON can only be whitespace between tokens.
        return str_replace(["\r", "\n"], '', $response->body());
    }

    /**
     * The headers the stateless MCP protocol (2026-07-28) requires to mirror
     * the request body. Requests of the older initialize handshake protocols
     * carry no protocol version in their _meta and need none.
     */
    private function protocolHeaders(string $message): array
    {
        $request = json_decode($message, true);
        $version = $request['params']['_meta']['io.modelcontextprotocol/protocolVersion'] ?? null;

        if (! is_string($version) || ! is_string($request['method'] ?? null)) {
            return [];
        }

        $name = match ($request['method']) {
            'tools/call', 'prompts/get' => $request['params']['name'] ?? null,
            'resources/read' => $request['params']['uri'] ?? null,
            default => null,
        };

        return array_filter([
            'MCP-Protocol-Version' => $version,
            'Mcp-Method' => $request['method'],
            'Mcp-Name' => is_string($name) ? $name : null,
        ]);
    }

    /**
     * A JSON-RPC error answering the given message, or null when it is a
     * notification, which gets no answer.
     */
    private function errorReply(string $message, int $code, string $text): ?string
    {
        $id = json_decode($message, true)['id'] ?? null;

        if ($id === null) {
            return null;
        }

        return json_encode(['jsonrpc' => '2.0', 'id' => $id, 'error' => ['code' => $code, 'message' => $text]], JSON_UNESCAPED_SLASHES);
    }
}
