<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Server URL
    |--------------------------------------------------------------------------
    |
    | The memry server used by `memry setup` when no --url option is given.
    |
    */

    'url' => env('MEMRY_URL', 'https://db-mcp-production-n8vlvz.laravel.cloud'),

    /*
    |--------------------------------------------------------------------------
    | Executable
    |--------------------------------------------------------------------------
    |
    | The shell command Claude Code uses to run memry, for the MCP headers
    | helper and the SessionStart hook. Set MEMRY_EXECUTABLE to a stable
    | path (the Homebrew wrapper does) so an upgrade does not break it.
    | Unset resolves it to the running memry executable. A custom
    | MEMRY_CONFIG is prefixed in both cases.
    |
    */

    'executable' => env('MEMRY_EXECUTABLE'),

];
