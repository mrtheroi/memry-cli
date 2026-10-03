<?php

use App\Support\Stdin;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\Http;

beforeEach(function () {
    $this->tmpDir = sys_get_temp_dir().'/memry-test-'.bin2hex(random_bytes(6));
    mkdir($this->tmpDir, 0700, true);
    $this->configPath = $this->tmpDir.'/config.json';
    file_put_contents($this->configPath, json_encode(['url' => 'https://memry.test', 'token' => 'secret-token']));
    putenv('MEMRY_CONFIG='.$this->configPath);
    Http::preventStrayRequests();
});

afterEach(function () {
    putenv('MEMRY_CONFIG');
    exec('rm -rf '.escapeshellarg($this->tmpDir));
});

/**
 * Give the hook the JSON Claude Code would write to its stdin.
 */
function hookInput(array $input): void
{
    app()->instance(Stdin::class, new class(json_encode($input)) extends Stdin
    {
        public function __construct(private string $contents) {}

        public function read(): string
        {
            return $this->contents;
        }
    });
}

function gitRepo(string $path): string
{
    mkdir($path, 0755, true);
    exec('git -C '.escapeshellarg($path).' init -q');

    return $path;
}

/**
 * Run the hook and return its exit code and exact stdout.
 */
function runHook(array $input): array
{
    hookInput($input);

    return [Artisan::call('hook:session-start'), Artisan::output()];
}

it('prints the protocol block and the context body using the git top-level as project', function () {
    $repo = gitRepo($this->tmpDir.'/MyProject');
    mkdir($repo.'/src/deep', 0755, true);
    Http::fake(['*/api/context*' => Http::response("## Latest session\nDid things")]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => $repo.'/src/deep', 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe(<<<'TXT'
## memry memory (project: MyProject)
memry is available through the `memry` MCP tools.
- Use get-memory with an id to read a memory from the context below in full, and search-memory to find older ones.
- Save decisions, bug fixes and discoveries with save-memory (project "MyProject", with a topic_key for evolving topics). Save a memory about another product under that product's project instead.
- Before ending the session, save a summary with session-summary (project "MyProject", repo "MyProject").

## Latest session
Did things
TXT."\n");
});

it('uses the project from .memry.json at the git top-level and the folder name as repo', function () {
    $repo = gitRepo($this->tmpDir.'/memry-cli');
    mkdir($repo.'/src/deep', 0755, true);
    file_put_contents($repo.'/.memry.json', '{"project": "memry"}');
    Http::fake(['*/api/context*' => Http::response('body')]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => $repo.'/src/deep', 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toStartWith("## memry memory (project: memry)\n")
        ->and($output)->toContain('(project "memry", repo "memry-cli")');
});

it('trims the project from .memry.json', function () {
    $repo = gitRepo($this->tmpDir.'/memry-cli');
    file_put_contents($repo.'/.memry.json', '{"project": "  memry \n"}');
    Http::fake(['*/api/context*' => Http::response('body')]);

    [, $output] = runHook(['session_id' => 'abc', 'cwd' => $repo, 'source' => 'startup']);

    expect($output)->toStartWith("## memry memory (project: memry)\n");
});

it('falls back to the repo name when .memry.json has no usable project', function (string $contents) {
    $repo = gitRepo($this->tmpDir.'/memry-cli');
    file_put_contents($repo.'/.memry.json', $contents);
    Http::fake(['*/api/context*' => Http::response('body')]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => $repo, 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toStartWith("## memry memory (project: memry-cli)\n");
})->with([
    'invalid json' => ['not json'],
    'missing project' => ['{"name": "memry"}'],
    'empty project' => ['{"project": ""}'],
    'blank project' => ['{"project": "   "}'],
    'non-string project' => ['{"project": 42}'],
    'non-object json' => ['"memry"'],
]);

it('falls back to the cwd basename outside a git repository', function () {
    mkdir($dir = $this->tmpDir.'/PlainFolder');
    Http::fake(['*/api/context*' => Http::response('body')]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => $dir, 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toStartWith("## memry memory (project: PlainFolder)\n");
});

it('falls back to the repo name when .memry.json is unreadable', function () {
    $repo = gitRepo($this->tmpDir.'/memry-cli');
    file_put_contents($file = $repo.'/.memry.json', '{"project": "memry"}');
    chmod($file, 0000);
    Http::fake(['*/api/context*' => Http::response('body')]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => $repo, 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toStartWith("## memry memory (project: memry-cli)\n");
});

it('honors .memry.json in the cwd outside a git repository', function () {
    mkdir($dir = $this->tmpDir.'/PlainFolder');
    file_put_contents($dir.'/.memry.json', '{"project": "memry"}');
    Http::fake(['*/api/context*' => Http::response('body')]);

    [, $output] = runHook(['session_id' => 'abc', 'cwd' => $dir, 'source' => 'startup']);

    expect($output)->toStartWith("## memry memory (project: memry)\n")
        ->and($output)->toContain('repo "PlainFolder"');
});

it('uses the working directory when the input has no cwd', function (array $input) {
    mkdir($dir = $this->tmpDir.'/FromPwd');
    $previous = getcwd();
    chdir($dir);
    Http::fake(['*/api/context*' => Http::response('body')]);

    try {
        [$exitCode, $output] = runHook($input);
    } finally {
        chdir($previous);
    }

    expect($exitCode)->toBe(0)
        ->and($output)->toStartWith("## memry memory (project: FromPwd)\n");
})->with([
    'empty cwd' => [['session_id' => 'abc', 'cwd' => '', 'source' => 'startup']],
    'null cwd' => [['session_id' => 'abc', 'cwd' => null, 'source' => 'startup']],
    'missing cwd' => [['session_id' => 'abc', 'source' => 'startup']],
]);

it('requests the context with the bearer token, an encoded project and a 3 second timeout', function () {
    mkdir($dir = $this->tmpDir.'/My Project');
    $timeout = null;
    Http::fake(function ($request, array $options) use (&$timeout) {
        $timeout = $options['timeout'] ?? null;

        return Http::response('body');
    });

    [, $output] = runHook(['session_id' => 'abc', 'cwd' => $dir, 'source' => 'startup']);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/context?project=My%20Project'
        && $request->method() === 'GET'
        && $request->hasHeader('Authorization', 'Bearer secret-token'));
    expect($timeout)->toBe(3)
        ->and($output)->not->toContain('secret-token');
});

it('requests the context of the project from .memry.json', function () {
    $repo = gitRepo($this->tmpDir.'/memry-cli');
    file_put_contents($repo.'/.memry.json', '{"project": "memry app"}');
    Http::fake(['*/api/context*' => Http::response('body')]);

    runHook(['session_id' => 'abc', 'cwd' => $repo, 'source' => 'startup']);

    Http::assertSent(fn ($request) => $request->url() === 'https://memry.test/api/context?project=memry%20app');
});

it('prints nothing, sends nothing and exits zero without a usable config', function (?string $contents) {
    unlink($this->configPath);
    if ($contents !== null) {
        file_put_contents($this->configPath, $contents);
    }
    Http::fake();

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => gitRepo($this->tmpDir.'/MyProject'), 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('');
    Http::assertNothingSent();
})->with([
    'missing file' => [null],
    'without token' => ['{"url": "https://memry.test"}'],
    'without url' => ['{"token": "secret-token"}'],
    'empty token' => ['{"url": "https://memry.test", "token": ""}'],
    'invalid json' => ['not json'],
]);

it('prints nothing and exits zero when the request fails', function (Closure $response) {
    Http::fake(['*/api/context*' => $response()]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => gitRepo($this->tmpDir.'/MyProject'), 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('');
})->with([
    'unauthorized' => [fn () => Http::response('Unauthenticated.', 401)],
    'server error' => [fn () => Http::response('partial', 500)],
    'connection failure' => [fn () => Http::failedConnection()],
]);

it('prints nothing and exits zero instead of following a redirect', function () {
    Http::fake([
        'https://memry.test/api/context*' => Http::response('', 302, ['Location' => 'https://memry.test/login']),
        'https://memry.test/login' => Http::response('<html>Log in</html>', 200),
    ]);

    [$exitCode, $output] = runHook(['session_id' => 'abc', 'cwd' => gitRepo($this->tmpDir.'/MyProject'), 'source' => 'startup']);

    expect($exitCode)->toBe(0)
        ->and($output)->toBe('');
    Http::assertNotSent(fn ($request) => $request->url() === 'https://memry.test/login');
});

it('prints nothing and exits zero on an unexpected error', function () {
    app()->instance(Stdin::class, new class extends Stdin
    {
        public function read(): string
        {
            throw new RuntimeException('stdin exploded');
        }
    });

    expect(Artisan::call('hook:session-start'))->toBe(0)
        ->and(Artisan::output())->toBe('');
});

it('ends the output with exactly one newline like the bash hook', function () {
    Http::fake(['*/api/context*' => Http::response("body\n\n")]);

    [, $output] = runHook(['session_id' => 'abc', 'cwd' => gitRepo($this->tmpDir.'/MyProject'), 'source' => 'startup']);

    expect($output)->toEndWith("\n\nbody\n");
});

it('is hidden from the command list', function () {
    $this->artisan('list')
        ->doesntExpectOutputToContain('hook:session-start')
        ->assertExitCode(0);
});
