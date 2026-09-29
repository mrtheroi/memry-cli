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
    | Headers Helper Command
    |--------------------------------------------------------------------------
    |
    | The shell command Claude Code runs to get the memry MCP headers. Null
    | resolves it to the running memry executable followed by `mcp-headers`.
    |
    */

    'helper_command' => null,

];
