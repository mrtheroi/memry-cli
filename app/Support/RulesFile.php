<?php

namespace App\Support;

/**
 * A Markdown instructions file of an agent, holding memry's rules in a
 * marker-delimited block so the rest of the file is never touched.
 */
class RulesFile
{
    public const START = '<!-- memry:start -->';

    public const END = '<!-- memry:end -->';

    public function __construct(private string $path) {}

    public function path(): string
    {
        return $this->path;
    }

    /**
     * Write $rules as the memry block.
     */
    public function put(string $rules): bool
    {
        if (! is_dir(dirname($this->path))) {
            mkdir(dirname($this->path), 0700, true);
        }

        $contents = is_file($this->path) ? file_get_contents($this->path) : '';
        $block = self::START."\n".$rules."\n".self::END."\n";
        $span = $this->blockSpan($contents);

        if ($span === false) {
            return false;
        }

        AtomicFile::write($this->path, $span === null
            ? BlankLine::after($contents).$block
            : substr_replace($contents, $block, $span[0], $span[1]));

        return true;
    }

    /**
     * The offset and length of the memry block, including the newline
     * after its end marker, null when there is none, or false when the
     * markers do not delimit exactly one block.
     *
     * @return array{int, int}|null|false
     */
    private function blockSpan(string $contents): array|null|false
    {
        $starts = substr_count($contents, self::START);
        $ends = substr_count($contents, self::END);

        if ($starts === 0 && $ends === 0) {
            return null;
        }

        $start = strpos($contents, self::START);

        if ($starts !== 1 || $ends !== 1 || strpos($contents, self::END) < $start) {
            return false;
        }

        $end = strpos($contents, self::END) + strlen(self::END);

        if (($contents[$end] ?? '') === "\n") {
            $end++;
        }

        return [$start, $end - $start];
    }

    /**
     * Remove the memry block and the blank line setting it apart. Returns
     * whether there was a block, or null, leaving the file untouched, when
     * its markers are broken.
     */
    public function remove(): ?bool
    {
        $contents = is_file($this->path) ? file_get_contents($this->path) : '';
        $span = $this->blockSpan($contents);

        if ($span === false) {
            return null;
        }

        if ($span === null) {
            return false;
        }

        $before = substr($contents, 0, $span[0]);
        $after = substr($contents, $span[0] + $span[1]);

        $rest = BlankLine::join($before, $after);

        // Nothing but the block means memry created the file.
        trim($rest) === '' ? unlink($this->path) : AtomicFile::write($this->path, $rest);

        return true;
    }
}
