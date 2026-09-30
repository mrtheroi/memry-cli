<?php

namespace App\Agents;

/**
 * The memry rules written into the global instructions of agents that have
 * no SessionStart hook, so the agent loads the context itself. Claude Code
 * gets the same guidance from its hook instead.
 */
class Protocol
{
    public static function text(): string
    {
        return <<<'MD'
            ## memry memory

            memry gives you a persistent memory through the `memry` MCP tools.

            - At the start of every session, call get-context with the project name: the `project` in `.memry.json` at the repository root if there is one, otherwise the repository (or folder) name.
            - Use search-memory to find older memories and get-memory to read one in full when they are relevant to the task.
            - Save decisions, bug fixes and discoveries with save-memory for that project, with a topic_key for evolving topics.
            - Before ending the session, save a summary with session-summary (project and repository name).
            MD;
    }
}
