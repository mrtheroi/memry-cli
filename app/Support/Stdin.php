<?php

namespace App\Support;

/**
 * The process standard input, bound in the container so tests can replace it.
 */
class Stdin
{
    public function read(): string
    {
        return (string) file_get_contents('php://stdin');
    }
}
