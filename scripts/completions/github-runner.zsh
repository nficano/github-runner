#compdef github-runner

_github_runner() {
    local -a commands
    commands=(
        'register:Register a new runner with GitHub'
        'unregister:Remove runner registration'
        'start:Start runner daemon with worker pool'
        'stop:Signal running daemon to stop gracefully'
        'run:Execute a single job and exit'
        'list:List registered runners and their status'
        'verify:Test runner connectivity and auth'
        'status:Live status of running daemon'
        'cache:Cache management subcommands'
        'exec:Run a workflow locally'
        'version:Print version information'
        'help:Help about any command'
    )

    _arguments -C \
        '--config[Config file path]:file:_files -g "*.toml"' \
        '--log-level[Log level]:level:(debug info warn error)' \
        '--log-format[Log format]:format:(json text)' \
        '--help[Show help]' \
        '1:command:->command' \
        '*::arg:->args'

    case "$state" in
        command)
            _describe 'command' commands
            ;;
        args)
            case "${words[1]}" in
                register)
                    _arguments \
                        '--url[Repository, org, or enterprise URL]:url:' \
                        '--token[Registration token]:token:' \
                        '--name[Runner name]:name:' \
                        '--executor[Executor type]:executor:(shell docker kubernetes firecracker)' \
                        '--labels[Comma-separated labels]:labels:' \
                        '--work-dir[Working directory]:dir:_directories' \
                        '--ephemeral[Register as ephemeral]'
                    ;;
                start)
                    _arguments \
                        '--concurrency[Override global concurrency]:number:' \
                        '--listen[Health/metrics listen address]:address:' \
                        '--pid-file[PID file path]:file:_files' \
                        '--foreground[Run in foreground]'
                    ;;
                list|status)
                    _arguments \
                        '--format[Output format]:format:(table json yaml)'
                    ;;
                cache)
                    local -a cache_commands
                    cache_commands=(
                        'clear:Clear all caches'
                        'stats:Show cache utilization'
                        'prune:Remove expired entries'
                    )
                    _describe 'cache command' cache_commands
                    ;;
            esac
            ;;
    esac
}

_github_runner "$@"
