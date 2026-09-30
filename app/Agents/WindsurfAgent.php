<?php

namespace App\Agents;

use App\Support\JsonMcpConfig;
use App\Support\McpConfig;
use App\Support\RulesFile;

/**
 * Windsurf (now Devin Desktop): the memry MCP server in
 * ~/.codeium/windsurf/mcp_config.json and the memry instructions in its
 * global rules, ~/.codeium/windsurf/memories/global_rules.md.
 */
class WindsurfAgent extends ConfigFileAgent
{
    public function key(): string
    {
        return 'windsurf';
    }

    public function name(): string
    {
        return 'Windsurf';
    }

    public function isInstalled(): bool
    {
        return is_dir($this->directory());
    }

    protected function mcpConfig(): McpConfig
    {
        return new JsonMcpConfig($this->directory().'/mcp_config.json', 'mcpServers');
    }

    protected function rulesFile(): RulesFile
    {
        return new RulesFile($this->directory().'/memories/global_rules.md');
    }

    protected function server(): array
    {
        return $this->commandServer();
    }

    private function directory(): string
    {
        return getenv('HOME').'/.codeium/windsurf';
    }
}
