<?php

use App\Support\AtomicFile;

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/memry-atomic-'.bin2hex(random_bytes(6));
    mkdir($this->dir, 0700, true);
    $this->path = $this->dir.'/file.json';
});

afterEach(function () {
    exec('rm -rf '.escapeshellarg($this->dir));
});

it('replaces the file with a new one instead of writing into it, keeping its permissions', function () {
    file_put_contents($this->path, 'old');
    chmod($this->path, 0640);
    clearstatcache();
    $inode = fileinode($this->path);

    AtomicFile::write($this->path, 'new');
    clearstatcache();

    expect(file_get_contents($this->path))->toBe('new')
        ->and(fileinode($this->path))->not->toBe($inode)
        ->and(fileperms($this->path) & 0777)->toBe(0640)
        ->and(scandir($this->dir))->toBe(['.', '..', 'file.json']);
});
