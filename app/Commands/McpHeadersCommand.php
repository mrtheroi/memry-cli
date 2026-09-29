<?php

namespace App\Commands;

use App\Support\ConfigFile;
use LaravelZero\Framework\Commands\Command;

class McpHeadersCommand extends Command
{
    protected $signature = 'mcp-headers';

    protected $description = 'Print the memry MCP authorization header as JSON (Claude Code headersHelper)';

    protected $hidden = true;

    public function handle(): int
    {
        $token = ConfigFile::resolve()->read()['token'] ?? null;

        if (! is_string($token) || $token === '') {
            $this->output->getErrorStyle()->writeln('memry is not logged in. Run memry setup.');

            return self::FAILURE;
        }

        $this->line(json_encode(['Authorization' => "Bearer {$token}"]));

        return self::SUCCESS;
    }
}
