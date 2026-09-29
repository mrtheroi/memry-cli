<?php

namespace App\Support;

class Executable
{
    /**
     * The shell command that runs the given memry subcommand with this
     * executable: the PHAR itself, or the PHP binary plus the memry script.
     */
    public static function command(string $subcommand): string
    {
        return self::executable().' '.$subcommand;
    }

    private static function executable(): string
    {
        $executable = config('memry.executable') ?: (\Phar::running(false) !== ''
            ? escapeshellarg(\Phar::running(false))
            : escapeshellarg(PHP_BINARY).' '.escapeshellarg(base_path('memry')));

        // Claude Code runs our commands without our environment, so a custom config path must travel with them.
        $env = getenv('MEMRY_CONFIG') ? 'MEMRY_CONFIG='.escapeshellarg(getenv('MEMRY_CONFIG')).' ' : '';

        return $env.$executable;
    }
}
