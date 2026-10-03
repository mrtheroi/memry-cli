<?php

namespace App\Support;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;

class AuthToken
{
    /**
     * Revoke the token on the server it belongs to.
     */
    public static function revoke(string $url, string $token): RevokeResult
    {
        try {
            $response = Http::acceptJson()->timeout(10)->withoutRedirecting()->withToken($token)->delete($url.'/api/auth/token');
        } catch (ConnectionException) {
            return RevokeResult::Failed;
        }

        return match (true) {
            $response->successful() => RevokeResult::Revoked,
            $response->unauthorized() => RevokeResult::AlreadyRevoked,
            default => RevokeResult::Failed,
        };
    }
}
