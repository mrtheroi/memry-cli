<?php

namespace App\Agents;

use App\Support\Executable;
use App\Support\JsonMcpConfig;
use App\Support\McpConfig;
use App\Support\RulesFile;

/**
 * OpenCode: the memry MCP server in ~/.config/opencode/opencode.json and
 * the memry instructions in ~/.config/opencode/AGENTS.md.
 */
class OpenCodeAgent extends ConfigFileAgent
{
    public function key(): string
    {
        return 'opencode';
    }

    public function name(): string
    {
        return 'OpenCode';
    }

    public function isInstalled(): bool
    {
        return $this->onPath('opencode') || is_dir($this->directory());
    }

    /**
     * opencode.json, or else an existing opencode.jsonc, like `opencode mcp
     * add` picks it. A .jsonc file with comments cannot be parsed, so it is
     * left untouched with manual instructions instead.
     */
    protected function mcpConfig(): McpConfig
    {
        $json = $this->directory().'/opencode.json';
        $jsonc = $this->directory().'/opencode.jsonc';

        return new JsonMcpConfig(! is_file($json) && is_file($jsonc) ? $jsonc : $json, 'mcp');
    }

    protected function rulesFile(): RulesFile
    {
        return new RulesFile($this->directory().'/AGENTS.md');
    }

    protected function server(): array
    {
        return $this->withEnvironment(['type' => 'local', 'command' => Executable::arguments('mcp'), 'enabled' => true], 'environment');
    }

    /**
     * $XDG_CONFIG_HOME/opencode, or ~/.config/opencode, as OpenCode resolves it.
     */
    private function directory(): string
    {
        return (getenv('XDG_CONFIG_HOME') ?: getenv('HOME').'/.config').'/opencode';
    }
}
