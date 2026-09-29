<?php

use App\Support\ClaudeSettings;

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/memry-settings-'.bin2hex(random_bytes(6));
    mkdir($this->dir, 0700, true);
    $this->path = $this->dir.'/settings.json';
    $this->group = ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'memry hook:session-start', 'timeout' => 10]]];
});

afterEach(function () {
    exec('rm -rf '.escapeshellarg($this->dir));
});

it('keeps non-ASCII characters readable when rewriting the settings', function () {
    file_put_contents($this->path, json_encode(['statusLine' => ['text' => 'Diseño ✓']], JSON_UNESCAPED_UNICODE));

    (new ClaudeSettings($this->path))->replaceSessionStartHook($this->group, 'hook:session-start');

    expect(file_get_contents($this->path))->toContain('Diseño ✓');
});

it('leaves the settings untouched when the hooks have an unexpected shape', function (string $contents) {
    file_put_contents($this->path, $contents);

    $installed = (new ClaudeSettings($this->path))->replaceSessionStartHook($this->group, 'hook:session-start');

    expect($installed)->toBeFalse()
        ->and(file_get_contents($this->path))->toBe($contents);
})->with([
    'hooks is a string' => ['{"hooks": "nope"}'],
    'hooks is a list' => ['{"hooks": []}'],
    'SessionStart is an object' => ['{"hooks": {"SessionStart": {"matcher": "startup"}}}'],
    'SessionStart is a string' => ['{"hooks": {"SessionStart": "nope"}}'],
    'a group is not an object' => ['{"hooks": {"SessionStart": ["nope"]}}'],
    'a group has non-list hooks' => ['{"hooks": {"SessionStart": [{"hooks": "nope"}]}}'],
]);

it('leaves no temporary file behind after writing the settings', function () {
    file_put_contents($this->path, '{}');

    (new ClaudeSettings($this->path))->replaceSessionStartHook($this->group, 'hook:session-start');

    expect(array_values(array_diff(scandir($this->dir), ['.', '..'])))->toBe(['settings.json']);
});

it('removes only the memry hooks, keeping every other hook and setting', function () {
    file_put_contents($this->path, json_encode([
        'env' => new stdClass,
        'hooks' => [
            'SessionStart' => [
                ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'other-tool start']]],
                ['matcher' => 'startup|resume', 'hooks' => [
                    ['type' => 'command', 'command' => "'/opt/memry/memry' hook:session-start", 'timeout' => 10],
                    ['type' => 'command', 'command' => 'shared-group-tool'],
                ]],
                $this->group,
            ],
            'Stop' => [['hooks' => [['type' => 'command', 'command' => 'on-stop']]]],
        ],
    ]));

    $removed = (new ClaudeSettings($this->path))->removeSessionStartHook('hook:session-start');

    expect($removed)->toBeTrue()
        ->and(json_decode(file_get_contents($this->path), true))->toBe([
            'env' => [],
            'hooks' => [
                'SessionStart' => [
                    ['matcher' => 'startup', 'hooks' => [['type' => 'command', 'command' => 'other-tool start']]],
                    ['matcher' => 'startup|resume', 'hooks' => [['type' => 'command', 'command' => 'shared-group-tool']]],
                ],
                'Stop' => [['hooks' => [['type' => 'command', 'command' => 'on-stop']]]],
            ],
        ])
        ->and(file_get_contents($this->path))->toContain('"env": {}');
});

it('drops the hooks that removing the memry hook leaves empty', function (array $hooks, array $expected) {
    file_put_contents($this->path, json_encode(['model' => 'opus', 'hooks' => $hooks]));

    (new ClaudeSettings($this->path))->removeSessionStartHook('hook:session-start');

    expect(json_decode(file_get_contents($this->path), true))->toBe($expected);
})->with([
    'only the memry hook' => fn () => [['SessionStart' => [$this->group]], ['model' => 'opus']],
    'memry and a Stop hook' => fn () => [
        ['SessionStart' => [$this->group], 'Stop' => [['hooks' => [['type' => 'command', 'command' => 'on-stop']]]]],
        ['model' => 'opus', 'hooks' => ['Stop' => [['hooks' => [['type' => 'command', 'command' => 'on-stop']]]]]],
    ],
]);

it('leaves the settings untouched when there is no memry hook to remove', function (string $contents) {
    file_put_contents($this->path, $contents);

    $removed = (new ClaudeSettings($this->path))->removeSessionStartHook('hook:session-start');

    expect($removed)->toBeFalse()
        ->and(file_get_contents($this->path))->toBe($contents);
})->with([
    'no hooks' => ['{"model":"opus"}'],
    'empty SessionStart' => ['{"hooks":{"SessionStart":[]}}'],
    'other SessionStart hooks' => ['{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"other"}]}]}}'],
]);

it('removes the memry hook from a group it shares with other hooks', function () {
    file_put_contents($this->path, json_encode(['hooks' => ['SessionStart' => [['hooks' => [
        ['type' => 'command', 'command' => 'memry hook:session-start'],
        ['type' => 'command', 'command' => 'other'],
    ]]]]]));

    $removed = (new ClaudeSettings($this->path))->removeSessionStartHook('hook:session-start');

    expect($removed)->toBeTrue()
        ->and(json_decode(file_get_contents($this->path), true))
        ->toBe(['hooks' => ['SessionStart' => [['hooks' => [['type' => 'command', 'command' => 'other']]]]]]);
});

it('does not create the settings when removing a hook from a missing file', function () {
    $path = $this->dir.'/missing/settings.json';

    $removed = (new ClaudeSettings($path))->removeSessionStartHook('hook:session-start');

    expect($removed)->toBeFalse()
        ->and(file_exists(dirname($path)))->toBeFalse();
});

it('leaves malformed settings untouched when removing the memry hook', function (string $contents) {
    file_put_contents($this->path, $contents);

    $removed = (new ClaudeSettings($this->path))->removeSessionStartHook('hook:session-start');

    expect($removed)->toBeNull()
        ->and(file_get_contents($this->path))->toBe($contents);
})->with([
    'not JSON' => ['{"hooks": '],
    'not an object' => ['[]'],
    'SessionStart is an object' => ['{"hooks": {"SessionStart": {"matcher": "startup"}}}'],
    'a group has non-list hooks' => ['{"hooks": {"SessionStart": [{"hooks": "memry hook:session-start"}]}}'],
]);
