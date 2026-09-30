<?php

namespace App\Support;

/**
 * A JSON config file of an agent that lists its MCP servers by name under
 * one top-level key, like "mcpServers" or "mcp".
 */
class JsonMcpConfig implements McpConfig
{
    public function __construct(private string $path, private string $key) {}

    public function path(): string
    {
        return $this->path;
    }

    /**
     * Set the server $name, replacing any previous one of that name.
     */
    public function put(string $name, array $server): bool
    {
        if (! is_dir(dirname($this->path))) {
            mkdir(dirname($this->path), 0700, true);
        }

        // Objects, not arrays, so empty objects like "env": {} are written back as they were.
        $config = is_file($this->path) ? json_decode(file_get_contents($this->path)) : new \stdClass;

        if (! $this->isWellFormed($config)) {
            return false;
        }

        $config->{$this->key} ??= new \stdClass;
        $config->{$this->key}->{$name} = $server;

        $this->write($config);

        return true;
    }

    public function snippet(string $name, array $server): string
    {
        return $this->encode((object) [$this->key => (object) [$name => $server]]);
    }

    /**
     * Remove the server $name, and the server list when that leaves it
     * empty, as put() creates it when missing. Returns whether there was
     * such a server, or null, leaving the file untouched, when it cannot
     * be parsed.
     */
    public function remove(string $name): ?bool
    {
        if (! is_file($this->path)) {
            return false;
        }

        $config = json_decode(file_get_contents($this->path));

        if (! $this->isWellFormed($config)) {
            return null;
        }

        if (! isset($config->{$this->key}->{$name})) {
            return false;
        }

        unset($config->{$this->key}->{$name});

        if (get_object_vars($config->{$this->key}) === []) {
            unset($config->{$this->key});
        }

        $this->write($config);

        return true;
    }

    /**
     * A JSON object whose servers, if present, are an object.
     */
    private function isWellFormed(mixed $config): bool
    {
        return $config instanceof \stdClass
            && (! isset($config->{$this->key}) || $config->{$this->key} instanceof \stdClass);
    }

    private function write(\stdClass $config): void
    {
        AtomicFile::write($this->path, $this->encode($config).PHP_EOL);
    }

    private function encode(\stdClass $config): string
    {
        return json_encode($config, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    }
}
