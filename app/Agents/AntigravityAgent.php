<?php

namespace App\Agents;

use App\Support\JsonMcpConfig;
use App\Support\McpConfig;
use App\Support\RulesFile;

/**
 * Google Antigravity: the memry MCP server in ~/.gemini/config/mcp_config.json
 * and the memry instructions in ~/.gemini/config/GEMINI.md, a global rules
 * file only Antigravity reads (Gemini CLI reads ~/.gemini/GEMINI.md instead).
 */
class AntigravityAgent extends ConfigFileAgent
{
    public function key(): string
    {
        return 'antigravity';
    }

    public function name(): string
    {
        return 'Antigravity';
    }

    public function isInstalled(): bool
    {
        return is_dir(getenv('HOME').'/.gemini/antigravity') || is_dir(getenv('HOME').'/.gemini/config');
    }

    protected function mcpConfig(): McpConfig
    {
        return new JsonMcpConfig(getenv('HOME').'/.gemini/config/mcp_config.json', 'mcpServers');
    }

    protected function rulesFile(): RulesFile
    {
        return new RulesFile(getenv('HOME').'/.gemini/config/GEMINI.md');
    }

    protected function server(): array
    {
        return $this->commandServer();
    }
}
