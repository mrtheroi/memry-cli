<?php

namespace App\Support;

class Email
{
    /**
     * Trim and lowercase the email, or return null when it is not a valid
     * address (which also rejects bytes that are not valid UTF-8).
     */
    public static function normalize(string $email): ?string
    {
        $email = strtolower(trim($email));

        return filter_var($email, FILTER_VALIDATE_EMAIL) === false ? null : $email;
    }
}
