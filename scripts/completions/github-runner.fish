# fish completion for github-runner

complete -c github-runner -f

# Top-level commands
complete -c github-runner -n '__fish_use_subcommand' -a register -d 'Register a new runner with GitHub'
complete -c github-runner -n '__fish_use_subcommand' -a unregister -d 'Remove runner registration'
complete -c github-runner -n '__fish_use_subcommand' -a start -d 'Start runner daemon with worker pool'
complete -c github-runner -n '__fish_use_subcommand' -a stop -d 'Signal running daemon to stop gracefully'
complete -c github-runner -n '__fish_use_subcommand' -a run -d 'Execute a single job and exit'
complete -c github-runner -n '__fish_use_subcommand' -a list -d 'List registered runners and their status'
complete -c github-runner -n '__fish_use_subcommand' -a verify -d 'Test runner connectivity and auth'
complete -c github-runner -n '__fish_use_subcommand' -a status -d 'Live status of running daemon'
complete -c github-runner -n '__fish_use_subcommand' -a cache -d 'Cache management subcommands'
complete -c github-runner -n '__fish_use_subcommand' -a exec -d 'Run a workflow locally'
complete -c github-runner -n '__fish_use_subcommand' -a version -d 'Print version information'

# Global flags
complete -c github-runner -l config -d 'Config file path' -r -F
complete -c github-runner -l log-level -d 'Log level' -r -a 'debug info warn error'
complete -c github-runner -l log-format -d 'Log format' -r -a 'json text'

# Cache subcommands
complete -c github-runner -n '__fish_seen_subcommand_from cache' -a clear -d 'Clear all caches'
complete -c github-runner -n '__fish_seen_subcommand_from cache' -a stats -d 'Show cache utilization'
complete -c github-runner -n '__fish_seen_subcommand_from cache' -a prune -d 'Remove expired entries'

# Register flags
complete -c github-runner -n '__fish_seen_subcommand_from register' -l url -d 'Repository URL' -r
complete -c github-runner -n '__fish_seen_subcommand_from register' -l token -d 'Registration token' -r
complete -c github-runner -n '__fish_seen_subcommand_from register' -l name -d 'Runner name' -r
complete -c github-runner -n '__fish_seen_subcommand_from register' -l executor -d 'Executor type' -r -a 'shell docker kubernetes firecracker'
complete -c github-runner -n '__fish_seen_subcommand_from register' -l labels -d 'Comma-separated labels' -r
complete -c github-runner -n '__fish_seen_subcommand_from register' -l work-dir -d 'Working directory' -r -a '(__fish_complete_directories)'
complete -c github-runner -n '__fish_seen_subcommand_from register' -l ephemeral -d 'Register as ephemeral'

# Start flags
complete -c github-runner -n '__fish_seen_subcommand_from start' -l concurrency -d 'Override concurrency' -r
complete -c github-runner -n '__fish_seen_subcommand_from start' -l listen -d 'Listen address' -r
complete -c github-runner -n '__fish_seen_subcommand_from start' -l pid-file -d 'PID file path' -r -F
complete -c github-runner -n '__fish_seen_subcommand_from start' -l foreground -d 'Run in foreground'

# List/status flags
complete -c github-runner -n '__fish_seen_subcommand_from list status' -l format -d 'Output format' -r -a 'table json yaml'
