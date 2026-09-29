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
    | helper and the SessionStart hook. Null resolves it to the running
    | memry executable, prefixed with a custom MEMRY_CONFIG when set.
    |
    */

    'executable' => null,

];
