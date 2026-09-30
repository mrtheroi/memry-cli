<?php

namespace App\Support;

/**
 * A TOML config file of an agent that lists its MCP servers as
 * [mcp_servers.<name>] tables, like Codex's config.toml.
 */
class TomlMcpConfig implements McpConfig
{
    /** A bare, "basic" or 'literal' key segment. */
    private const SEGMENT = '[A-Za-z0-9_-]+|"(?:[^"\\\\\n]|\\\\.)*"|\'[^\'\n]*\'';

    /** A dotted key. */
    private const KEY = '(?:'.self::SEGMENT.')(?:\s*\.\s*(?:'.self::SEGMENT.'))*';

    public function __construct(private string $path) {}

    public function path(): string
    {
        return $this->path;
    }

    /**
     * Set the server $name, replacing any previous one of that name.
     *
     * @param  array<string, string|list<string>|array<string, string>>  $server
     */
    public function put(string $name, array $server): bool
    {
        if (! is_dir(dirname($this->path))) {
            mkdir(dirname($this->path), 0700, true);
        }

        $contents = is_file($this->path) ? file_get_contents($this->path) : '';
        $scan = $this->scan($contents, $name);

        if ($scan === null) {
            return false;
        }

        [$lines, $spans] = $scan;

        AtomicFile::write($this->path, $spans === []
            ? BlankLine::after($contents).$this->snippet($name, $server)
            : $this->withoutSpans($lines, $spans, $this->snippet($name, $server)));

        return true;
    }

    /**
     * Remove the server $name and its subtables. Returns whether there was
     * such a server, or null, leaving the file untouched, when the file
     * cannot be edited safely.
     */
    public function remove(string $name): ?bool
    {
        $scan = $this->scan(is_file($this->path) ? file_get_contents($this->path) : '', $name);

        if ($scan === null) {
            return null;
        }

        [$lines, $spans] = $scan;

        if ($spans === []) {
            return false;
        }

        // The lines before the first span are the same with or without the server.
        $before = implode('', array_slice($lines, 0, $spans[0][0]));
        $after = substr($this->withoutSpans($lines, $spans, ''), strlen($before));

        AtomicFile::write($this->path, BlankLine::join($before, $after));

        return true;
    }

    /**
     * The lines without the given spans, putting $replacement in place of
     * the first one.
     *
     * @param  list<string>  $lines
     * @param  list<array{int, int}>  $spans
     */
    private function withoutSpans(array $lines, array $spans, string $replacement): string
    {
        foreach (array_reverse($spans, true) as $index => [$first, $last]) {
            array_splice($lines, $first, $last - $first + 1, $index === 0 ? [$replacement] : []);
        }

        return implode('', $lines);
    }

    /**
     * Split $contents into lines, each with its newline, and find the line
     * spans [first, last] of the [mcp_servers.<name>] table and its
     * subtables, without the blank and comment lines ending them. Returns
     * null when the file cannot be edited safely: it is not valid TOML as
     * far as this scan can tell, or it defines the server (or the
     * mcp_servers table) with dotted keys or an inline table instead.
     *
     * @return array{list<string>, list<array{int, int}>}|null
     */
    private function scan(string $contents, string $name): ?array
    {
        $lines = preg_split('/(?<=\n)/', $contents, -1, PREG_SPLIT_NO_EMPTY);
        $spans = [];
        $open = null;
        $table = [];
        $string = null;
        $depth = 0;

        foreach ($lines as $index => $line) {
            // Inside a multi-line string or array.
            if ($string !== null || $depth > 0) {
                if (! $this->scanValue($line, $string, $depth)) {
                    return null;
                }

                continue;
            }

            $trimmed = trim($line);

            if ($trimmed === '' || $trimmed[0] === '#') {
                continue;
            }

            if ($trimmed[0] === '[') {
                if (preg_match('/^\[\[?\s*('.self::KEY.')\s*\]\]?\s*(#.*)?$/', $trimmed, $match) !== 1) {
                    return null;
                }

                $table = $this->segments($match[1]);
                $ours = array_slice($table, 0, 2) === ['mcp_servers', $name];

                if ($open !== null && ! $ours) {
                    $spans[] = [$open, $this->lastContentLine($lines, $open, $index - 1)];
                    $open = null;
                } elseif ($open === null && $ours) {
                    $open = $index;
                }

                continue;
            }

            if (preg_match('/^('.self::KEY.')\s*=/', $trimmed, $match) !== 1) {
                return null;
            }

            $key = [...$table, ...$this->segments($match[1])];

            if (($open === null && array_slice($key, 0, 2) === ['mcp_servers', $name])
                || ($table === [] && $key[0] === 'mcp_servers')) {
                return null;
            }

            if (! $this->scanValue(substr($trimmed, strlen($match[0])), $string, $depth)) {
                return null;
            }
        }

        if ($string !== null || $depth !== 0) {
            return null;
        }

        if ($open !== null) {
            $spans[] = [$open, $this->lastContentLine($lines, $open, count($lines) - 1)];
        }

        return [$lines, $spans];
    }

    /**
     * Follow the strings, arrays and inline tables of a value, or of a line
     * of one that spans several lines, updating whether a multi-line string
     * and how many brackets are still open. Returns false when it is not
     * valid.
     */
    private function scanValue(string $text, ?string &$string, int &$depth): bool
    {
        for ($i = 0, $length = strlen($text); $i < $length; $i++) {
            if ($string === '"""' && $text[$i] === '\\') {
                $i++;
            } elseif ($string !== null) {
                if (substr($text, $i, 3) === $string) {
                    $string = null;
                    $i += 2;
                }
            } elseif ($text[$i] === '#') {
                break;
            } elseif (in_array(substr($text, $i, 3), ['"""', "'''"], true)) {
                $string = substr($text, $i, 3);
                $i += 2;
            } elseif ($text[$i] === '"' || $text[$i] === "'") {
                $pattern = $text[$i] === '"' ? '/"(?:[^"\\\\\n]|\\\\.)*"/A' : "/'[^'\\n]*'/A";

                if (preg_match($pattern, $text, $match, 0, $i) !== 1) {
                    return false;
                }

                $i += strlen($match[0]) - 1;
            } elseif ($text[$i] === '[' || $text[$i] === '{') {
                $depth++;
            } elseif (($text[$i] === ']' || $text[$i] === '}') && --$depth < 0) {
                return false;
            }
        }

        return true;
    }

    /**
     * The segments of a dotted key, unquoted.
     *
     * @return list<string>
     */
    private function segments(string $key): array
    {
        preg_match_all('/'.self::SEGMENT.'/', $key, $matches);

        return array_map(fn ($segment) => match ($segment[0]) {
            '"' => json_decode($segment) ?? $segment,
            "'" => substr($segment, 1, -1),
            default => $segment,
        }, $matches[0]);
    }

    /**
     * The last line from $first to $last that is not blank or a comment.
     *
     * @param  list<string>  $lines
     */
    private function lastContentLine(array $lines, int $first, int $last): int
    {
        while ($last > $first && (trim($lines[$last]) === '' || trim($lines[$last])[0] === '#')) {
            $last--;
        }

        return $last;
    }

    /**
     * The [mcp_servers.<name>] table for $server, one key per line.
     */
    public function snippet(string $name, array $server): string
    {
        $lines = ['[mcp_servers.'.$this->key($name).']'];

        foreach ($server as $key => $value) {
            $lines[] = $this->key($key).' = '.$this->value($value);
        }

        return implode("\n", $lines)."\n";
    }

    private function key(string $key): string
    {
        return preg_match('/^[A-Za-z0-9_-]+$/', $key) === 1 ? $key : $this->string($key);
    }

    private function value(string|array $value): string
    {
        return match (true) {
            is_string($value) => $this->string($value),
            array_is_list($value) => '['.implode(', ', array_map($this->string(...), $value)).']',
            default => '{ '.implode(', ', array_map(
                fn ($key) => $this->key($key).' = '.$this->string($value[$key]),
                array_keys($value),
            )).' }',
        };
    }

    /**
     * A TOML basic string: JSON's escapes are all valid TOML escapes.
     */
    private function string(string $value): string
    {
        return json_encode($value, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    }
}
