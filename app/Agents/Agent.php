<?php

namespace App\Agents;

/**
 * An AI agent memry can be wired into. Adding an agent means one class
 * implementing this contract, listed in the AgentRegistry.
 */
interface Agent
{
    /**
     * The stable key used in `--agents` and the config file, e.g. 'claude-code'.
     */
    public function key(): string;

    /**
     * The name shown to the user, e.g. 'Claude Code'.
     */
    public function name(): string;

    /**
     * Whether the agent is found on this machine.
     */
    public function isInstalled(): bool;

    /**
     * Wire memry into the agent, for the memry server at $url.
     */
    public function install(string $url): AgentResult;

    /**
     * Remove memry from the agent.
     */
    public function uninstall(): AgentResult;
}
