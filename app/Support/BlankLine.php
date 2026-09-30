<?php

namespace App\Support;

/**
 * The blank line that sets a block memry adds to a file apart from the
 * rest of it, added when the block goes in and dropped when it comes out.
 */
class BlankLine
{
    /**
     * The contents, ending with a blank line unless they are empty, ready
     * for a block to be appended.
     */
    public static function after(string $contents): string
    {
        return match (true) {
            $contents === '', str_ends_with($contents, "\n\n") => $contents,
            str_ends_with($contents, "\n") => $contents."\n",
            default => $contents."\n\n",
        };
    }

    /**
     * The contents before and after a removed block, joined without the
     * blank line that set it apart.
     */
    public static function join(string $before, string $after): string
    {
        return match (true) {
            str_ends_with($before, "\n\n") && $after === '' => substr($before, 0, -1),
            str_ends_with($before, "\n\n") && str_starts_with($after, "\n") => $before.substr($after, 1),
            default => $before.$after,
        };
    }
}
