<?php

namespace App\Agents;

use App\Support\Executable;
use App\Support\McpConfig;
use App\Support\RulesFile;
use Symfony\Component\Process\ExecutableFinder;

/**
 * An agent memry is wired into by editing its files: the memry MCP server,
 * run over stdio with `memry mcp`, in its config file, and the memry
 * Protocol in its global instructions, as it has no SessionStart hook.
 */
abstract class ConfigFileAgent implements Agent
{
    public const MCP_SERVER = 'memry';

    /** @var list<array{string, string}> */
    private array $lines = [];

    /**
     * The config file that lists the agent's MCP servers.
     */
    abstract protected function mcpConfig(): McpConfig;

    /**
     * The memry server entry in the agent's format.
     */
    abstract protected function server(): array;

    /**
     * The agent's global instructions file, read in every session.
     */
    abstract protected function rulesFile(): RulesFile;

    /**
     * The server URL is not written anywhere: `memry mcp` reads it, with
     * the token, from the memry config file.
     */
    public function install(string $url): AgentResult
    {
        $this->lines = [];

        // The instructions only need their own file, so write them even when registration fails.
        $registered = $this->registerMcpServer();
        $instructed = $this->addInstructions();

        return new AgentResult($registered && $instructed, $this->lines);
    }

    public function uninstall(): AgentResult
    {
        $this->lines = [];

        // Each step runs even when the other one fails.
        $removed = $this->removeMcpServer();
        $uninstructed = $this->removeInstructions();

        return new AgentResult($removed && $uninstructed, $this->lines);
    }

    /**
     * The server as the common {command, args, env} entry.
     */
    protected function commandServer(): array
    {
        $arguments = Executable::arguments('mcp');

        return $this->withEnvironment(['command' => array_shift($arguments), 'args' => $arguments], 'env');
    }

    /**
     * The server with the environment `memry mcp` needs under $key, if any.
     */
    protected function withEnvironment(array $server, string $key): array
    {
        return Executable::environment() === [] ? $server : [...$server, $key => Executable::environment()];
    }

    /**
     * Whether the command $name is on the PATH.
     */
    protected function onPath(string $name): bool
    {
        return (new ExecutableFinder)->find($name) !== null;
    }

    private function registerMcpServer(): bool
    {
        $config = $this->mcpConfig();

        if (! $config->put(self::MCP_SERVER, $this->server())) {
            $this->say('error', 'Could not register the '.self::MCP_SERVER." MCP server: memry cannot edit {$config->path()} safely.");
            $this->say('line', 'Fix the file, then add this to it by hand:');
            $this->say('line', rtrim($config->snippet(self::MCP_SERVER, $this->server())));

            return false;
        }

        $this->say('info', 'Registered the '.self::MCP_SERVER." MCP server in {$config->path()}.");

        return true;
    }

    private function addInstructions(): bool
    {
        $rules = $this->rulesFile();

        if (! $rules->put(Protocol::text())) {
            $this->say('error', $this->brokenMarkers('Could not add the memry instructions', $rules));
            $this->say('line', 'Fix or remove them, then run `memry setup` again.');

            return false;
        }

        $this->say('info', "Added the memry instructions to {$rules->path()}.");

        return true;
    }

    private function removeMcpServer(): bool
    {
        $config = $this->mcpConfig();
        $removed = $config->remove(self::MCP_SERVER);

        if ($removed === null) {
            $this->say('warn', 'Could not remove the '.self::MCP_SERVER." MCP server: memry cannot edit {$config->path()} safely.");

            return false;
        }

        $removed
            ? $this->say('info', 'Removed the '.self::MCP_SERVER." MCP server from {$config->path()}.")
            : $this->say('line', 'No '.self::MCP_SERVER." MCP server to remove from {$config->path()}.");

        return true;
    }

    private function removeInstructions(): bool
    {
        $rules = $this->rulesFile();
        $removed = $rules->remove();

        if ($removed === null) {
            $this->say('warn', $this->brokenMarkers('Could not remove the memry instructions', $rules));

            return false;
        }

        $removed
            ? $this->say('info', "Removed the memry instructions from {$rules->path()}.")
            : $this->say('line', "No memry instructions to remove from {$rules->path()}.");

        return true;
    }

    private function brokenMarkers(string $failure, RulesFile $rules): string
    {
        return "{$failure}: the ".RulesFile::START.' and '.RulesFile::END." markers in {$rules->path()} do not enclose one block.";
    }

    private function say(string $style, string $text): void
    {
        $this->lines[] = [$style, $text];
    }
}
