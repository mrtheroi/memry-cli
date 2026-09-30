<?php

namespace App\Agents;

/**
 * The agents memry supports, in display order.
 */
class AgentRegistry
{
    /**
     * @param  list<Agent>  $agents
     */
    public function __construct(private array $agents = [new ClaudeCodeAgent]) {}

    /**
     * @return list<Agent>
     */
    public function all(): array
    {
        return $this->agents;
    }

    /**
     * The agents with the given keys, in display order.
     *
     * @param  list<string>  $keys
     * @return list<Agent>
     */
    public function only(array $keys): array
    {
        return array_values(array_filter($this->agents, fn (Agent $agent) => in_array($agent->key(), $keys, true)));
    }

    /**
     * @return list<string>
     */
    public function keys(): array
    {
        return array_map(fn (Agent $agent) => $agent->key(), $this->agents);
    }
}
