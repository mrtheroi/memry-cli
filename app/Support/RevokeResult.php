<?php

namespace App\Support;

enum RevokeResult
{
    case Revoked;

    /** The server answered 401: the token was no longer valid. */
    case AlreadyRevoked;

    /** Any other error response, or the server could not be reached. */
    case Failed;
}
