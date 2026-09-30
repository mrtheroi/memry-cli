<?php

namespace App\Support;

/**
 * An agent's config file that lists its MCP servers by name.
 */
interface McpConfig
{
    public function path(): string;

    /**
     * Set the server $name, replacing any previous one of that name.
     * Returns false, leaving the file untouched, when it cannot be edited
     * safely.
     */
    public function put(string $name, array $server): bool;

    /**
     * Remove the server $name. Returns whether there was one, or null,
     * leaving the file untouched, when it cannot be edited safely.
     */
    public function remove(string $name): ?bool;

    /**
     * The server $name as it would appear in the file, for manual setup.
     */
    public function snippet(string $name, array $server): string;
}
