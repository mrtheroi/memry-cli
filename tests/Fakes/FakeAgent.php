<?php

namespace Tests\Fakes;

use App\Agents\Agent;
use App\Agents\AgentResult;

/**
 * An agent that records the calls it gets and succeeds unless told to fail.
 */
class FakeAgent implements Agent
{
    /** @var list<string> */
    public array $calls = [];

    public function __construct(
        private string $key,
        private string $name,
        private bool $installed = true,
        private bool $succeeds = true,
    ) {}

    public function key(): string
    {
        return $this->key;
    }

    public function name(): string
    {
        return $this->name;
    }

    public function isInstalled(): bool
    {
        return $this->installed;
    }

    public function install(string $url): AgentResult
    {
        $this->calls[] = "install {$url}";

        return new AgentResult($this->succeeds, [$this->succeeds ? ['info', "Wired {$this->name}."] : ['error', "Could not wire {$this->name}."]]);
    }

    public function uninstall(): AgentResult
    {
        $this->calls[] = 'uninstall';

        return new AgentResult($this->succeeds, [['info', "Unwired {$this->name}."]]);
    }
}
