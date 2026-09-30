<?php

namespace Tests;

use LaravelZero\Framework\Testing\TestCase as BaseTestCase;

abstract class TestCase extends BaseTestCase
{
    protected function setUp(): void
    {
        parent::setUp();

        // config/app.php fixes the environment to development; as testing, commands
        // ask Laravel Prompts questions through the console so tests can answer them.
        $this->app['env'] = 'testing';
    }
}
