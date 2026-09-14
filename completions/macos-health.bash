# bash completion for macos-health

_macos_health_completions() {
    local cur opts
    COMPREPLY=()
    cur="${COMPWORDS[COMPCWORD]}"
    opts="--once --json --stream --samples --duration --record --replay --serve --check --schema --prometheus --interval --no-color --no-alt-screen --ascii --redact --sensors --ports --system-processes --theme --group --config --save-config --demo --version --help"

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi
}

complete -F _macos_health_completions macos-health
