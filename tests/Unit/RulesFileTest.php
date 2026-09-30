<?php

use App\Support\RulesFile;

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/memry-rules-'.bin2hex(random_bytes(6));
    $this->path = $this->dir.'/agent/AGENTS.md';
    $this->block = "<!-- memry:start -->\nUse memry.\n<!-- memry:end -->\n";
});

afterEach(function () {
    exec('rm -rf '.escapeshellarg($this->dir));
});

it('creates the file and its directories with only the memry block', function () {
    expect((new RulesFile($this->path))->put('Use memry.'))->toBeTrue()
        ->and(file_get_contents($this->path))->toBe($this->block);
});

it('appends the memry block after the existing rules, keeping them as they were', function (string $existing, string $separator) {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, $existing);

    (new RulesFile($this->path))->put('Use memry.');

    expect(file_get_contents($this->path))->toBe($existing.$separator.$this->block);
})->with([
    'ending with a newline' => ["# Rules\n\nBe concise. Diseño ✓\n", "\n"],
    'without a final newline' => ['# Rules', "\n\n"],
    'ending with blank lines' => ["# Rules\n\n\n", ''],
]);

it('replaces an existing memry block in place, keeping the rules around it', function () {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, "# Before\n\n<!-- memry:start -->\nOld rules.\nMore.\n<!-- memry:end -->\n\n# After\n");

    (new RulesFile($this->path))->put('Use memry.');

    expect(file_get_contents($this->path))->toBe("# Before\n\n".$this->block."\n# After\n");
});

it('leaves exactly the same file after writing the same block twice', function () {
    $rules = new RulesFile($this->path);
    $rules->put('Use memry.');
    $rules->put('Use memry.');

    expect(file_get_contents($this->path))->toBe($this->block);
});

it('leaves the file untouched when its memry markers are broken', function (string $contents) {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, $contents);

    expect((new RulesFile($this->path))->put('Use memry.'))->toBeFalse()
        ->and(file_get_contents($this->path))->toBe($contents);
})->with([
    'start without end' => ["# Rules\n<!-- memry:start -->\nOld.\n"],
    'end without start' => ["# Rules\nOld.\n<!-- memry:end -->\n"],
    'end before start' => ["<!-- memry:end -->\n<!-- memry:start -->\n"],
    'two blocks' => ["<!-- memry:start -->\nA\n<!-- memry:end -->\n<!-- memry:start -->\nB\n<!-- memry:end -->\n"],
]);

it('removes only the memry block and the blank line before it', function (string $existing, string $expected) {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, $existing);
    $rules = new RulesFile($this->path);
    $rules->put('Use memry.');

    expect($rules->remove())->toBeTrue()
        ->and(file_get_contents($this->path))->toBe($expected);
})->with([
    'ending with a newline' => ["# Rules\n\nBe concise. Diseño ✓\n", "# Rules\n\nBe concise. Diseño ✓\n"],
    'without a final newline' => ['# Rules', "# Rules\n"],
    'ending with blank lines' => ["# Rules\n\n\n", "# Rules\n\n"],
]);

it('removes a memry block between other rules without leaving a double blank line', function () {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, "# Before\n\n".$this->block."\n# After\n");

    (new RulesFile($this->path))->remove();

    expect(file_get_contents($this->path))->toBe("# Before\n\n# After\n");
});

it('deletes the file when only the memry block was in it', function () {
    $rules = new RulesFile($this->path);
    $rules->put('Use memry.');

    expect($rules->remove())->toBeTrue()
        ->and(file_exists($this->path))->toBeFalse();
});

it('removes nothing when there is no memry block or no file', function (?string $contents) {
    if ($contents !== null) {
        mkdir(dirname($this->path), 0700, true);
        file_put_contents($this->path, $contents);
    }

    expect((new RulesFile($this->path))->remove())->toBeFalse()
        ->and(is_file($this->path) ? file_get_contents($this->path) : null)->toBe($contents);
})->with([
    'no file' => [null],
    'no block' => ["# Rules\n"],
]);

it('leaves the file untouched when removing with broken memry markers', function () {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, $contents = "# Rules\n<!-- memry:start -->\nOld.\n");

    expect((new RulesFile($this->path))->remove())->toBeNull()
        ->and(file_get_contents($this->path))->toBe($contents);
});

it('replaces the file atomically, keeping its permissions', function (string $method) {
    mkdir(dirname($this->path), 0700, true);
    file_put_contents($this->path, "# Rules\n".($method === 'remove' ? "\n".$this->block : ''));
    chmod($this->path, 0640);
    clearstatcache();
    $inode = fileinode($this->path);

    $method === 'put' ? (new RulesFile($this->path))->put('Use memry.') : (new RulesFile($this->path))->remove();
    clearstatcache();

    expect(fileinode($this->path))->not->toBe($inode)
        ->and(fileperms($this->path) & 0777)->toBe(0640);
})->with(['put', 'remove']);
