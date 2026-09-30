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

    /**
     * The same command as a list of arguments, for agents that take the
     * executable and its arguments separately instead of a shell command.
     *
     * @return list<string>
     */
    public static function arguments(string $subcommand): array
    {
        $executable = config('memry.executable')
            ? [config('memry.executable')]
            : (\Phar::running(false) !== '' ? [\Phar::running(false)] : [PHP_BINARY, base_path('memry')]);

        return [...$executable, $subcommand];
    }

    /**
     * The environment variables to run those arguments with: a custom
     * MEMRY_CONFIG must travel with them, as agents do not pass on ours.
     *
     * @return array<string, string>
     */
    public static function environment(): array
    {
        return getenv('MEMRY_CONFIG') ? ['MEMRY_CONFIG' => getenv('MEMRY_CONFIG')] : [];
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
