<?php

use App\Agents\Protocol;

it('tells the agent to load the project context at the start of every session', function () {
    expect(Protocol::text())
        ->toContain('At the start of every session, call get-context')
        ->toContain('`.memry.json`');
});

it('tells the agent how to search, save and summarize memories', function () {
    expect(Protocol::text())
        ->toContain('search-memory')
        ->toContain('get-memory')
        ->toContain('save-memory')
        ->toContain('topic_key')
        ->toContain('session-summary');
});

it('stays short enough for the smallest global rules file', function () {
    // Windsurf limits its global rules to 6,000 characters.
    expect(strlen(Protocol::text()))->toBeLessThan(1500);
});
