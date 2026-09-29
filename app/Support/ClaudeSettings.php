<?php

namespace App\Support;

class ClaudeSettings
{
    public function __construct(private string $path) {}

    /**
     * The file at $CLAUDE_CONFIG_DIR/settings.json, or ~/.claude/settings.json by default.
     */
    public static function resolve(): self
    {
        return new self((getenv('CLAUDE_CONFIG_DIR') ?: getenv('HOME').'/.claude').'/settings.json');
    }

    public function path(): string
    {
        return $this->path;
    }

    /**
     * Add the given matcher group to the SessionStart hooks, first removing
     * every hook whose command contains $marker (and any matcher group left
     * empty by that), so a reinstall never duplicates it. Returns false,
     * leaving the file untouched, when it is not a valid JSON object.
     */
    public function replaceSessionStartHook(array $group, string $marker): bool
    {
        if (! is_dir(dirname($this->path))) {
            mkdir(dirname($this->path), 0700, true);
        }

        // Objects, not arrays, so empty objects like "env": {} are written back as they were.
        $settings = is_file($this->path) ? json_decode(file_get_contents($this->path)) : new \stdClass;

        if (! $this->isWellFormed($settings)) {
            return false;
        }

        $settings->hooks ??= new \stdClass;
        $settings->hooks->SessionStart ??= [];
        $settings->hooks->SessionStart = $this->withoutHooks($settings->hooks->SessionStart, $marker);
        $settings->hooks->SessionStart[] = $group;

        $this->write($settings);

        return true;
    }

    /**
     * Remove every SessionStart hook whose command contains $marker (and any
     * matcher group left empty by that), keeping every other hook and setting.
     * A SessionStart list and hooks object left empty are dropped too, as
     * installing the hook creates them when missing. Returns whether a hook
     * was removed, or null, leaving the file untouched, when it is not a
     * valid JSON object with the expected hooks shape.
     */
    public function removeSessionStartHook(string $marker): ?bool
    {
        if (! is_file($this->path)) {
            return false;
        }

        $settings = json_decode(file_get_contents($this->path));

        if (! $this->isWellFormed($settings)) {
            return null;
        }

        $groups = $settings->hooks->SessionStart ?? [];
        // Encoded first: withoutHooks() edits the groups it keeps in place.
        $before = json_encode($groups);
        $kept = $this->withoutHooks($groups, $marker);

        if (json_encode($kept) === $before) {
            return false;
        }

        $settings->hooks->SessionStart = $kept;

        if ($settings->hooks->SessionStart === []) {
            unset($settings->hooks->SessionStart);
        }

        if (get_object_vars($settings->hooks) === []) {
            unset($settings->hooks);
        }

        $this->write($settings);

        return true;
    }

    private function write(\stdClass $settings): void
    {
        $this->writeAtomically(json_encode($settings, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE).PHP_EOL);
    }

    /**
     * Write to a temporary file in the same directory and rename it over the
     * settings, so a crash mid-write never leaves a truncated settings.json.
     */
    private function writeAtomically(string $contents): void
    {
        $mode = is_file($this->path) ? fileperms($this->path) & 0777 : 0666 & ~umask();
        $temporary = tempnam(dirname($this->path), '.settings.json.');

        file_put_contents($temporary, $contents);
        chmod($temporary, $mode);
        rename($temporary, $this->path);
    }

    /**
     * A JSON object whose hooks, if present, have the shape Claude Code expects:
     * an object whose SessionStart is a list of groups with a list of hooks.
     */
    private function isWellFormed(mixed $settings): bool
    {
        if (! $settings instanceof \stdClass) {
            return false;
        }

        if (! isset($settings->hooks)) {
            return true;
        }

        if (! $settings->hooks instanceof \stdClass) {
            return false;
        }

        $groups = $settings->hooks->SessionStart ?? [];

        if (! is_array($groups) || ! array_is_list($groups)) {
            return false;
        }

        foreach ($groups as $group) {
            if (! $group instanceof \stdClass || ! is_array($group->hooks ?? []) || ! array_is_list($group->hooks ?? [])) {
                return false;
            }
        }

        return true;
    }

    private function withoutHooks(array $groups, string $marker): array
    {
        $kept = [];

        foreach ($groups as $group) {
            $hooks = array_values(array_filter(
                $group->hooks ?? [],
                fn ($hook) => ! str_contains($hook->command ?? '', $marker),
            ));

            if (count($hooks) === count($group->hooks ?? [])) {
                $kept[] = $group;
            } elseif ($hooks !== []) {
                $group->hooks = $hooks;
                $kept[] = $group;
            }
        }

        return $kept;
    }
}
