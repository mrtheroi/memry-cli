<?php

namespace Tests;

use LaravelZero\Framework\Testing\TestCase as BaseTestCase;

abstract class TestCase extends BaseTestCase
{
    /** The environment variables agents resolve their files from. */
    private const AGENT_ENV = ['HOME', 'CODEX_HOME', 'XDG_CONFIG_HOME'];

    /** @var array<string, string|false> */
    private array $originalEnv = [];

    private string $home;

    protected function setUp(): void
    {
        parent::setUp();

        // config/app.php fixes the environment to development; as testing, commands
        // ask Laravel Prompts questions through the console so tests can answer them.
        $this->app['env'] = 'testing';

        // Never let a test reach the real agent configs: every test starts in an empty HOME.
        foreach (self::AGENT_ENV as $name) {
            $this->originalEnv[$name] = getenv($name);
            putenv($name);
        }

        $this->home = sys_get_temp_dir().'/memry-test-home-'.bin2hex(random_bytes(6));
        mkdir($this->home, 0700, true);
        putenv('HOME='.$this->home);
    }

    protected function tearDown(): void
    {
        foreach ($this->originalEnv as $name => $value) {
            putenv($value === false ? $name : "{$name}={$value}");
        }

        exec('rm -rf '.escapeshellarg($this->home));

        parent::tearDown();
    }
}
