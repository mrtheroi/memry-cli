<?php

namespace App\Commands\Concerns;

use App\Agents\AgentResult;

trait ReportsAgentResults
{
    /**
     * Print the lines of an agent result and return whether it succeeded.
     */
    private function report(AgentResult $result): bool
    {
        foreach ($result->lines as [$style, $text]) {
            $this->{$style}($text);
        }

        return $result->successful;
    }
}
