<?php

namespace App\Commands;

use App\Support\ConfigFile;
use App\Support\Stdin;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;
use LaravelZero\Framework\Commands\Command;
use Throwable;

class SessionStartHookCommand extends Command
{
    protected $signature = 'hook:session-start';

    protected $description = 'Print the memry context of the current project (Claude Code SessionStart hook)';

    protected $hidden = true;

    /**
     * Always succeed: a failing hook must never block a Claude Code session.
     */
    public function handle(Stdin $stdin): int
    {
        try {
            $this->printContext($stdin);
        } catch (Throwable) {
            // Print nothing.
        }

        return self::SUCCESS;
    }

    private function printContext(Stdin $stdin): void
    {
        $cwd = (json_decode($stdin->read(), true)['cwd'] ?? '') ?: getcwd();
        $config = ConfigFile::resolve()->read();

        if (! is_string($config['url'] ?? null) || $config['url'] === ''
            || ! is_string($config['token'] ?? null) || $config['token'] === '') {
            return;
        }

        $git = Process::run(['git', '-C', $cwd, 'rev-parse', '--show-toplevel']);
        $root = $git->successful() ? trim($git->output()) : $cwd;
        $repo = basename($root);
        $project = $this->declaredProject($root) ?? $repo;

        $response = Http::withToken($config['token'])
            ->accept('text/plain')
            ->timeout(3)
            ->get($config['url'].'/api/context?project='.rawurlencode($project));

        if ($response->failed()) {
            return;
        }

        // Like the bash hook's $(curl ...), drop the body's trailing newlines.
        $body = rtrim($response->body(), "\n");

        $this->output->write(<<<TXT
            ## memry memory (project: {$project})
            memry is available through the `memry` MCP tools.
            - Use get-memory with an id to read a memory from the context below in full, and search-memory to find older ones.
            - Save decisions, bug fixes and discoveries with save-memory (project "{$project}", with a topic_key for evolving topics). Save a memory about another product under that product's project instead.
            - Before ending the session, save a summary with session-summary (project "{$project}", repo "{$repo}").

            {$body}

            TXT);
    }

    /**
     * The project declared in the repo's .memry.json, or null when there is no usable one.
     */
    private function declaredProject(string $root): ?string
    {
        $file = $root.'/.memry.json';
        if (! is_file($file) || ! is_readable($file)) {
            return null;
        }

        $project = json_decode(file_get_contents($file), true)['project'] ?? null;
        $project = is_string($project) ? trim($project) : '';

        return $project !== '' ? $project : null;
    }
}
