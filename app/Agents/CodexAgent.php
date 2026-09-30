<?php

namespace App\Agents;

use App\Support\McpConfig;
use App\Support\RulesFile;
use App\Support\TomlMcpConfig;

/**
 * OpenAI Codex CLI: the memry MCP server in $CODEX_HOME/config.toml and
 * the memry instructions in $CODEX_HOME/AGENTS.md.
 */
class CodexAgent extends ConfigFileAgent
{
    public function key(): string
    {
        return 'codex';
    }

    public function name(): string
    {
        return 'Codex';
    }

    public function isInstalled(): bool
    {
        return $this->onPath('codex') || is_dir($this->home());
    }

    protected function mcpConfig(): McpConfig
    {
        return new TomlMcpConfig($this->home().'/config.toml');
    }

    /**
     * $CODEX_HOME when it is an absolute path, as Codex requires, or ~/.codex.
     */
    private function home(): string
    {
        $home = getenv('CODEX_HOME');

        return is_string($home) && str_starts_with($home, '/') ? rtrim($home, '/') : getenv('HOME').'/.codex';
    }

    protected function rulesFile(): RulesFile
    {
        return new RulesFile($this->home().'/AGENTS.md');
    }

    protected function server(): array
    {
        return $this->commandServer();
    }
}
