<?php

namespace App\Support;

/**
 * The process standard input and output streams, bound in the container so
 * tests can replace them.
 */
class StdioStreams
{
    /**
     * @param  resource  $input
     * @param  resource  $output
     */
    public function __construct(public $input, public $output) {}
}
