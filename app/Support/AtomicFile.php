<?php

namespace App\Support;

class AtomicFile
{
    /**
     * Write to a temporary file in the same directory and rename it over
     * $path, so a crash mid-write never leaves a truncated file. An existing
     * file keeps its permissions; a new one gets the default ones.
     */
    public static function write(string $path, string $contents): void
    {
        $mode = is_file($path) ? fileperms($path) & 0777 : 0666 & ~umask();
        $temporary = tempnam(dirname($path), '.'.basename($path).'.');

        file_put_contents($temporary, $contents);
        chmod($temporary, $mode);
        rename($temporary, $path);
    }
}
