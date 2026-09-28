<?php

namespace App\Support;

class ConfigFile
{
    public function __construct(private string $path) {}

    /**
     * The file at $MEMRY_CONFIG, or ~/.config/memry/config.json by default.
     */
    public static function resolve(): self
    {
        return new self(getenv('MEMRY_CONFIG') ?: getenv('HOME').'/.config/memry/config.json');
    }

    public function path(): string
    {
        return $this->path;
    }

    /**
     * Merge the given values into the file, keeping any other keys, and
     * make it readable by the owner only.
     */
    public function merge(array $values): void
    {
        if (! is_dir(dirname($this->path))) {
            mkdir(dirname($this->path), 0700, true);
        }

        $existing = is_file($this->path)
            ? (json_decode(file_get_contents($this->path), true) ?? [])
            : [];

        touch($this->path);
        chmod($this->path, 0600);
        file_put_contents($this->path, json_encode(array_merge($existing, $values), JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES).PHP_EOL);
    }
}
