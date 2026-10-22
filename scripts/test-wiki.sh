#!/usr/bin/env bash
# Throwaway MediaWiki in Docker, for `just test-integration` and CI.
#
#   scripts/test-wiki.sh up     start and provision it; print shell exports on stdout
#   scripts/test-wiki.sh down   remove the container
#
# Installs MediaWiki on SQLite with a random admin password, creates a bot
# password and one fixture page, and never touches a real wiki. Progress goes to
# stderr, so `eval "$(scripts/test-wiki.sh up)"` sees only the exports.
set -euo pipefail

name=wikisync-test-wiki
image=mediawiki:1.45.6 # Appropedia runs 1.45.x
port=${WIKISYNC_TEST_PORT:-8080}
server="http://127.0.0.1:$port"
api="$server/api.php"
page=Wikisync_integration

randhex() { od -An -N16 -tx1 /dev/urandom | tr -d ' \n'; } # 32 hex characters
log() { printf 'test-wiki: %s\n' "$*" >&2; }
wexec() { docker exec "$name" "$@"; }

case "${1:-}" in
up)
    docker rm -f "$name" >/dev/null 2>&1 || true
    log "starting $image on $server"
    docker run -d --name "$name" -p "127.0.0.1:$port:80" "$image" >/dev/null

    admin_pass=$(randhex)
    bot_pass=$(randhex) # a bot password must be exactly 32 characters

    log "installing MediaWiki on SQLite"
    wexec mkdir -p /var/www/data
    wexec php maintenance/run.php install \
        --dbtype=sqlite --dbpath=/var/www/data \
        --server="$server" --scriptpath= \
        --pass="$admin_pass" WikisyncTest Admin >&2

    log "creating a bot password and the fixture page"
    # Its success message prints the password, so keep stdout out of the logs.
    wexec php maintenance/run.php createBotPassword \
        --appid=wikisync --grants=editpage Admin "$bot_pass" >/dev/null
    printf "%s\n\n%s\n" "'''wikisync integration test'''" \
        "This page is a fixture for the wikisync integration test." |
        docker exec -i "$name" php maintenance/run.php edit \
            --user=Admin --summary=fixture "$page" >&2

    # Maintenance ran as root; the web server must be able to write the database.
    wexec chown -R www-data:www-data /var/www/data

    ready=
    for _ in $(seq 1 60); do
        out=$(curl -fsS "$api?action=query&meta=siteinfo&format=json" 2>/dev/null || true)
        case $out in *'"sitename"'*) ready=1 && break ;; esac
        sleep 1
    done
    if [ -z "$ready" ]; then
        log "the wiki did not answer at $api"
        docker logs "$name" >&2 || true
        exit 1
    fi
    log "ready at $api"

    printf 'export WIKISYNC_TEST_API_URL=%s\n' "$api"
    printf 'export WIKISYNC_TEST_PAGE=%s\n' "$page"
    printf 'export WIKISYNC_USERNAME=Admin@wikisync\n'
    printf 'export WIKISYNC_PASSWORD=%s\n' "$bot_pass"
    ;;
down)
    docker rm -f "$name" >/dev/null 2>&1 || true
    ;;
*)
    echo "usage: $0 up|down" >&2
    exit 2
    ;;
esac
