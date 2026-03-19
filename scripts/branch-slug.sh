#!/usr/bin/env bash
# Random git-safe branch slug: two adjective–noun pairs (crypto-ish via /dev/urandom).
# Keep word lists in sync with lib/dockpipe/domain/branchslug.go (branchSlugAdjectives / branchSlugNouns).
# Sourced by dockpipe-legacy.sh — do not `set -euo pipefail` here (would affect the parent shell).

_dockpipe_urandom_mod() {
  local max=$1
  local x
  x=$(od -An -N4 -tu4 /dev/urandom 2>/dev/null | tr -d ' \n' || true)
  if [[ -z "$x" ]]; then
    x=$RANDOM$RANDOM
  fi
  echo $((x % max))
}

dockpipe_random_branch_slug() {
  local -a _adj=(
    able apt azure bold brisk bright calm clear cool crisp
    curious dapper eager fancy fast fleet fresh gentle grand happy
    honest jolly keen kind light lively lucky merry mighty mint
    modern narrow neat nimble noble open patient polite proud quick
    quiet rapid rare ready real rich robust round sharp silent
    simple sleek smart smooth social solid sound spry steady still
    stoic strong subtle super sweet swift tidy tiny true vivid
    warm wild wise witty young zesty
  )
  local -a _noun=(
    adapter anchor api array atom badge beacon binary bitmap block
    branch bridge buffer bundle byte cache canvas channel chunk cipher
    client cloud cluster commit cookie core cron daemon delta deploy
    digest docker driver edge engine event fiber field filter frame
    gateway graph grid handler hash header heap hook http index
    ingress kernel lambda layer ledger lint loader lock log loop
    matrix merge metric mirror module mount mutex ngrok node packet
    patch peer pipe pixel pod poll portal probe proxy pulse
    queue quota raft range replica repo request ring route runner
    schema scope script server session shard shell signal socket spark
    stack stage stream subnet switch sync table token trace tunnel
    vector vertex volume voucher watch webhook widget worker worktree zone
  )
  local ia na ib nb al nl
  al=${#_adj[@]}
  nl=${#_noun[@]}
  ia=$(_dockpipe_urandom_mod "$al")
  na=$(_dockpipe_urandom_mod "$nl")
  ib=$(_dockpipe_urandom_mod "$al")
  nb=$(_dockpipe_urandom_mod "$nl")
  echo "${_adj[$ia]}-${_noun[$na]}-${_adj[$ib]}-${_noun[$nb]}"
}
