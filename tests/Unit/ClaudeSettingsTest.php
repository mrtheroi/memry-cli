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
