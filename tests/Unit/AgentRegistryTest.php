<?php

use App\Agents\Agent;
use App\Agents\AgentRegistry;

it('supports Claude Code, Codex, OpenCode, Antigravity and Windsurf, in that order', function () {
    $agents = (new AgentRegistry)->all();

    expect((new AgentRegistry)->keys())->toBe(['claude-code', 'codex', 'opencode', 'antigravity', 'windsurf'])
        ->and(array_map(fn (Agent $agent) => $agent->name(), $agents))->toBe(['Claude Code', 'Codex', 'OpenCode', 'Antigravity', 'Windsurf']);
});
