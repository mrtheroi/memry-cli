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

    'url' => env('MEMRY_URL', 'https://api.memry.com.mx'),

    /*
    |--------------------------------------------------------------------------
    | Executable
    |--------------------------------------------------------------------------
    |
    | The memry executable agents run: Claude Code for the MCP headers
    | helper and the SessionStart hook, the other agents for `memry mcp`.
    | Set MEMRY_EXECUTABLE to a stable path (the Homebrew wrapper does) so
    | an upgrade does not break it. Unset resolves it to the running memry
    | executable. A custom MEMRY_CONFIG travels with it in both cases.
    |
    */

    'executable' => env('MEMRY_EXECUTABLE'),

];
