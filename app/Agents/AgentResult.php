<?php

namespace App\Agents;

/**
 * The outcome of installing or uninstalling memry in an agent: whether it
 * succeeded, and the lines to show the user, each as [style, text] with
 * style one of info, warn, error or line.
 */
class AgentResult
{
    /**
     * @param  list<array{string, string}>  $lines
     */
    public function __construct(
        public readonly bool $successful,
        public readonly array $lines = [],
    ) {}
}
