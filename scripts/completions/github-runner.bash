#!/usr/bin/env bash
# bash completion for github-runner

_github_runner_completions() {
    local cur prev commands
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="register unregister start stop run list verify status cache exec version help"

    case "${prev}" in
        github-runner)
            COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
            return 0
            ;;
        cache)
            COMPREPLY=($(compgen -W "clear stats prune" -- "${cur}"))
            return 0
            ;;
        --executor)
            COMPREPLY=($(compgen -W "shell docker kubernetes firecracker" -- "${cur}"))
            return 0
            ;;
        --format)
            COMPREPLY=($(compgen -W "table json yaml" -- "${cur}"))
            return 0
            ;;
        --log-level)
            COMPREPLY=($(compgen -W "debug info warn error" -- "${cur}"))
            return 0
            ;;
        --log-format)
            COMPREPLY=($(compgen -W "json text" -- "${cur}"))
            return 0
            ;;
        --config)
            COMPREPLY=($(compgen -f -X '!*.toml' -- "${cur}"))
            return 0
            ;;
    esac

    if [[ "${cur}" == -* ]]; then
        local flags="--config --log-level --log-format --help"
        COMPREPLY=($(compgen -W "${flags}" -- "${cur}"))
        return 0
    fi
}

complete -F _github_runner_completions github-runner
